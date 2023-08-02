/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	ibus "github.com/untillpro/airs-ibus"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/iprocbus"
	istructs "github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

type blobWriteDetailsSingle struct {
	name     string
	mimeType string
}

type blobWriteDetailsMultipart struct {
	boundary string
}

type blobReadDetails struct {
	blobID istructs.RecordID
}

type blobBaseMessage struct {
	req                 *http.Request
	resp                http.ResponseWriter
	doneChan            chan struct{}
	wsid                istructs.WSID
	appQName            istructs.AppQName
	header              map[string][]string
	clusterAppBlobberID istructs.ClusterAppID
	blobMaxSize         BLOBMaxSizeType
}

type blobMessage struct {
	blobBaseMessage
	blobDetails interface{}
}

func (bm *blobBaseMessage) Release() {
	bm.req.Body.Close()
}

func blobReadMessageHandler(bbm blobBaseMessage, blobReadDetails blobReadDetails, blobStorage iblobstorage.IBLOBStorage, bus ibus.IBus, busTimeout time.Duration) {
	defer close(bbm.doneChan)

	// request to HVM to check the principalToken
	req := ibus.Request{
		Method:   ibus.HTTPMethodPOST,
		WSID:     int64(bbm.wsid),
		AppQName: bbm.appQName.String(),
		Resource: "c.sys.DownloadBLOBHelper",
		Header:   bbm.header,
		Body:     []byte(`{}`),
		Host:     localhost,
	}
	blobHelperResp, _, _, err := bus.SendRequest2(bbm.req.Context(), req, busTimeout)
	if err != nil {
		writeTextResponse(bbm.resp, "failed to exec c.sys.DownloadBLOBHelper: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if blobHelperResp.StatusCode != http.StatusOK {
		writeTextResponse(bbm.resp, "c.sys.DownloadBLOBHelper returned error: "+string(blobHelperResp.Data), blobHelperResp.StatusCode)
		return
	}

	// read the BLOB
	key := iblobstorage.KeyType{
		AppID: bbm.clusterAppBlobberID,
		WSID:  bbm.wsid,
		ID:    blobReadDetails.blobID,
	}
	stateWriterDiscard := func(state iblobstorage.BLOBState) error {
		if state.Status != iblobstorage.BLOBStatus_Completed {
			return errors.New("blob is not completed")
		}
		if len(state.Error) > 0 {
			return errors.New(state.Error)
		}
		bbm.resp.Header().Set(coreutils.ContentType, state.Descr.MimeType)
		bbm.resp.Header().Add("Content-Disposition", fmt.Sprintf(`attachment;filename="%s"`, state.Descr.Name))
		bbm.resp.WriteHeader(http.StatusOK)
		return nil
	}
	if err := blobStorage.ReadBLOB(bbm.req.Context(), key, stateWriterDiscard, bbm.resp); err != nil {
		if err == iblobstorage.ErrBLOBNotFound {
			writeTextResponse(bbm.resp, err.Error(), http.StatusNotFound)
			return
		}
		writeTextResponse(bbm.resp, err.Error(), http.StatusInternalServerError)
	}
}

func writeBLOB(ctx context.Context, wsid int64, appQName string, header map[string][]string, resp http.ResponseWriter,
	clusterAppBlobberID istructs.ClusterAppID, blobName, blobMimeType string, blobStorage iblobstorage.IBLOBStorage, body io.ReadCloser,
	blobMaxSize int64, bus ibus.IBus, busTimeout time.Duration) (blobID int64) {
	// request HVM for check the principalToken and get a blobID
	req := ibus.Request{
		Method:   ibus.HTTPMethodPOST,
		WSID:     int64(wsid),
		AppQName: appQName,
		Resource: "c.sys.UploadBLOBHelper",
		Body:     []byte(`{}`),
		Header:   header,
		Host:     localhost,
	}
	blobHelperResp, _, _, err := bus.SendRequest2(ctx, req, busTimeout)
	if err != nil {
		writeTextResponse(resp, "failed to exec c.sys.UploadBLOBHelper: "+err.Error(), http.StatusInternalServerError)
		return 0
	}
	if blobHelperResp.StatusCode != http.StatusOK {
		writeTextResponse(resp, "c.sys.UploadBLOBHelper returned error: "+string(blobHelperResp.Data), blobHelperResp.StatusCode)
		return 0
	}
	cmdResp := map[string]interface{}{}
	if err := json.Unmarshal(blobHelperResp.Data, &cmdResp); err != nil {
		writeTextResponse(resp, "failed to json-unmarshal c.sys.UploadBLOBHelper result: "+err.Error(), http.StatusInternalServerError)
		return 0
	}
	newIDs := cmdResp["NewIDs"].(map[string]interface{})

	blobID = int64(newIDs["1"].(float64))
	// write the BLOB
	key := iblobstorage.KeyType{
		AppID: clusterAppBlobberID,
		WSID:  istructs.WSID(wsid),
		ID:    istructs.RecordID(blobID),
	}
	descr := iblobstorage.DescrType{
		Name:     blobName,
		MimeType: blobMimeType,
	}

	if err := blobStorage.WriteBLOB(ctx, key, descr, body, blobMaxSize); err != nil {
		if err == iblobstorage.ErrBLOBSizeQuotaExceeded {
			writeTextResponse(resp, fmt.Sprintf("blob size quouta exceeded (max %d allowed)", blobMaxSize), http.StatusForbidden)
			return 0
		}
		writeTextResponse(resp, err.Error(), http.StatusInternalServerError)
		return 0
	}

	// set WDoc<sys.BLOB>.status = BLOBStatus_Completed
	req.Resource = "c.sys.CUD"
	req.Body = []byte(fmt.Sprintf(`{"cuds":[{"sys.ID": %d,"fields":{"status":%d}}]}`, blobID, iblobstorage.BLOBStatus_Completed))
	cudWDocBLOBUpdateResp, _, _, err := bus.SendRequest2(ctx, req, busTimeout)
	if err != nil {
		writeTextResponse(resp, "failed to exec c.sys.CUD: "+err.Error(), http.StatusInternalServerError)
		return 0
	}
	if cudWDocBLOBUpdateResp.StatusCode != http.StatusOK {
		writeTextResponse(resp, "c.sys.CUD returned error: "+string(cudWDocBLOBUpdateResp.Data), cudWDocBLOBUpdateResp.StatusCode)
		return 0
	}

	return blobID
}

func blobWriteMessageHandlerMultipart(bbm blobBaseMessage, blobStorage iblobstorage.IBLOBStorage, boundary string,
	bus ibus.IBus, busTimeout time.Duration) {
	defer close(bbm.doneChan)

	r := multipart.NewReader(bbm.req.Body, boundary)
	var part *multipart.Part
	var err error
	blobIDs := []string{}
	partNum := 0
	for err == nil {
		part, err = r.NextPart()
		if err != nil {
			if err != io.EOF {
				writeTextResponse(bbm.resp, "failed to parse multipart: "+err.Error(), http.StatusBadRequest)
				return
			} else if partNum == 0 {
				writeTextResponse(bbm.resp, "empty multipart request", http.StatusBadRequest)
				return
			}
			break
		}
		contentDisposition := part.Header.Get("Content-Disposition")
		mediaType, params, err := mime.ParseMediaType(contentDisposition)
		if err != nil {
			writeTextResponse(bbm.resp, fmt.Sprintf("failed to parse Content-Disposition of part number %d: %s", partNum, contentDisposition), http.StatusBadRequest)
		}
		if mediaType != "form-data" {
			writeTextResponse(bbm.resp, fmt.Sprintf("unsupported ContentDisposition mediaType of part number %d: %s", partNum, mediaType), http.StatusBadRequest)
		}
		contentType := part.Header.Get(coreutils.ContentType)
		if len(contentType) == 0 {
			contentType = "application/x-binary"
		}
		part.Header[coreutils.Authorization] = bbm.header[coreutils.Authorization] // add auth header for c.sys.*BLOBHelper
		blobID := writeBLOB(bbm.req.Context(), int64(bbm.wsid), bbm.appQName.String(), part.Header, bbm.resp, bbm.clusterAppBlobberID,
			params["name"], contentType, blobStorage, part, int64(bbm.blobMaxSize), bus, busTimeout)
		if blobID == 0 {
			return // request handled
		}
		blobIDs = append(blobIDs, fmt.Sprint(blobID))
		partNum++
	}
	writeTextResponse(bbm.resp, strings.Join(blobIDs, ","), http.StatusOK)
}

func blobWriteMessageHandlerSingle(bbm blobBaseMessage, blobWriteDetails blobWriteDetailsSingle, blobStorage iblobstorage.IBLOBStorage, header map[string][]string,
	bus ibus.IBus, busTimeout time.Duration) {
	defer close(bbm.doneChan)

	blobID := writeBLOB(bbm.req.Context(), int64(bbm.wsid), bbm.appQName.String(), header, bbm.resp, bbm.clusterAppBlobberID, blobWriteDetails.name,
		blobWriteDetails.mimeType, blobStorage, bbm.req.Body, int64(bbm.blobMaxSize), bus, busTimeout)
	if blobID > 0 {
		writeTextResponse(bbm.resp, fmt.Sprint(blobID), http.StatusOK)
	}
}

// ctx here is HVM context. It used to track HVM shutdown. Blobber will use the request's context
func blobMessageHandler(hvmCtx context.Context, sc iprocbus.ServiceChannel, blobStorage iblobstorage.IBLOBStorage, bus ibus.IBus, busTimeout time.Duration) {
	for hvmCtx.Err() == nil {
		select {
		case mesIntf := <-sc:
			blobMessage := mesIntf.(blobMessage)
			switch blobDetails := blobMessage.blobDetails.(type) {
			case blobReadDetails:
				blobReadMessageHandler(blobMessage.blobBaseMessage, blobDetails, blobStorage, bus, busTimeout)
			case blobWriteDetailsSingle:
				blobWriteMessageHandlerSingle(blobMessage.blobBaseMessage, blobDetails, blobStorage, blobMessage.header, bus, busTimeout)
			case blobWriteDetailsMultipart:
				blobWriteMessageHandlerMultipart(blobMessage.blobBaseMessage, blobStorage, blobDetails.boundary, bus, busTimeout)
			}
		case <-hvmCtx.Done():
			return
		}
	}
}

func (s *httpService) blobRequestHandler(resp http.ResponseWriter, req *http.Request, details interface{}) {
	vars := mux.Vars(req)
	wsid, err := strconv.ParseInt(vars[wsid], parseInt64Base, parseInt64Bits)
	if err != nil {
		// impossible, checked by router url rule
		// notest
		panic(err)
	}
	mes := blobMessage{
		blobBaseMessage: blobBaseMessage{
			req:                 req,
			resp:                resp,
			wsid:                istructs.WSID(wsid),
			doneChan:            make(chan struct{}),
			appQName:            istructs.NewAppQName(vars[appOwner], vars[appName]),
			header:              req.Header,
			clusterAppBlobberID: s.ClusterAppBlobberID,
			blobMaxSize:         s.BLOBMaxSize,
		},
		blobDetails: details,
	}
	if _, ok := mes.blobBaseMessage.header[coreutils.Authorization]; !ok {
		if cookie, err := req.Cookie(coreutils.Authorization); err == nil {
			if val, err := url.QueryUnescape(cookie.Value); err == nil {
				// authorization token in cookies -> c.sys.DownloadBLOBHelper requires it in headers
				mes.blobBaseMessage.header[coreutils.Authorization] = []string{val}
			}
		}
	}
	if !s.BlobberParams.procBus.Submit(0, 0, mes) {
		resp.WriteHeader(http.StatusServiceUnavailable)
		resp.Header().Add("Retry-After", fmt.Sprint(s.BlobberParams.RetryAfterSecondsOn503))
		return
	}
	select {
	case <-mes.doneChan:
	case <-req.Context().Done():
	}
}

func (s *httpService) blobReadRequestHandler() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		blobID, err := strconv.ParseInt(vars[blobID], parseInt64Base, parseInt64Bits)
		if err != nil {
			// impossible, checked by router url rule
			// notest
			panic(err)
		}
		principalToken := headerOrCookieAuth(resp, req)
		if len(principalToken) == 0 {
			return
		}
		blobReadDetails := blobReadDetails{
			blobID: istructs.RecordID(blobID),
		}
		s.blobRequestHandler(resp, req, blobReadDetails)
	}
}

func (s *httpService) blobWriteRequestHandler() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		principalToken, isHandled := headerAuth(resp, req)
		if len(principalToken) == 0 {
			if !isHandled {
				writeUnauthorized(resp)
			}
			return
		}

		queryParamName, queryParamMimeType, boundary, ok := getBlobParams(resp, req)
		if !ok {
			return
		}

		if len(queryParamName) > 0 {
			s.blobRequestHandler(resp, req, blobWriteDetailsSingle{
				name:     queryParamName,
				mimeType: queryParamMimeType,
			})
		} else {
			s.blobRequestHandler(resp, req, blobWriteDetailsMultipart{
				boundary: boundary,
			})
		}
	}
}

func headerAuth(rw http.ResponseWriter, req *http.Request) (principalToken string, isHandled bool) {
	authHeader := req.Header.Get(coreutils.Authorization)
	if len(authHeader) > 0 {
		if len(authHeader) < bearerPrefixLen || authHeader[:bearerPrefixLen] != coreutils.BearerPrefix {
			writeUnauthorized(rw)
			return "", true
		}
		return authHeader[bearerPrefixLen:], false
	}
	return "", false
}

func headerOrCookieAuth(rw http.ResponseWriter, req *http.Request) (principalToken string) {
	principalToken, isHandled := headerAuth(rw, req)
	if isHandled {
		return ""
	}
	if len(principalToken) > 0 {
		return principalToken
	}
	for _, c := range req.Cookies() {
		if c.Name == coreutils.Authorization {
			val, err := url.QueryUnescape(c.Value)
			if err != nil {
				writeTextResponse(rw, "failed to unescape cookie '"+c.Value+"'", http.StatusBadRequest)
				return ""
			}
			if len(val) < bearerPrefixLen || val[:bearerPrefixLen] != coreutils.BearerPrefix {
				writeUnauthorized(rw)
				return ""
			}
			return val[bearerPrefixLen:]
		}
	}
	writeUnauthorized(rw)
	return ""
}

// determines BLOBs write kind: name+mimeType in query params -> single BLOB, body is BLOB content, otherwise -> body is multipart/form-data
// (is multipart/form-data) == len(boundary) > 0
func getBlobParams(rw http.ResponseWriter, req *http.Request) (name, mimeType, boundary string, ok bool) {
	badRequest := func(msg string) {
		writeTextResponse(rw, msg, http.StatusBadRequest)
	}
	values := req.URL.Query()
	nameQuery, isSingleBLOB := values["name"]
	mimeTypeQuery, hasMimeType := values["mimeType"]
	if (isSingleBLOB && !hasMimeType) || (!isSingleBLOB && hasMimeType) {
		badRequest("both name and mimeType query params must be specified")
		return
	}

	contentType := req.Header.Get(coreutils.ContentType)
	if isSingleBLOB {
		if contentType == "multipart/form-data" {
			badRequest(`name+mimeType query params and "multipart/form-data" Content-Type header are mutual exclusive`)
			return
		}
		name = nameQuery[0]
		mimeType = mimeTypeQuery[0]
		ok = true
		return
	}
	if len(contentType) == 0 {
		badRequest(`neither "name"+"mimeType" query params nor Content-Type header is not provided`)
		return
	}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		badRequest("failed ot parse Content-Type header: " + contentType)
		return
	}
	if mediaType != "multipart/form-data" {
		badRequest("name+mimeType query params are not provided -> Content-Type must be mutipart/form-data but actual is " + contentType)
		return
	}
	boundary = params["boundary"]
	if len(boundary) == 0 {
		badRequest("boundary of multipart/form-data is not specified")
		return
	}
	return name, mimeType, boundary, true
}
