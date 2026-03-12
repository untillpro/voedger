/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package router

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/iblobstoragestg"
	"github.com/voedger/voedger/pkg/istructs"
)

func (s *routerService) blobHTTPRequestHandler_Write(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForBLOBs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		reqCtxWithAttribs := withLogAttribs(req.Context(), data, bus.Request{Resource: "sys._Blob_Write"}, req)
		logServeRequest(reqCtxWithAttribs)
		if !s.blobRequestHandler.HandleWrite(data.appQName, data.wsid, data.header, reqCtxWithAttribs, req.URL.Query(),
			newBLOBOKResponseIniter(rw, http.StatusOK), req.Body, func(sysErr coreutils.SysError) {
				writeCommonError_V1(rw, sysErr, sysErr.HTTPStatus)
			}, s.requestSender) {
			rw.WriteHeader(http.StatusServiceUnavailable)
			rw.Header().Add("Retry-After", strconv.Itoa(DefaultRetryAfterSecondsOn503))
		}
	})
}

func (s *routerService) blobHTTPRequestHandler_Read(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForBLOBs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		reqCtxWithAttribs := withLogAttribs(req.Context(), data, bus.Request{Resource: "sys._Blob_Read"}, req)
		logServeRequest(reqCtxWithAttribs)
		vars := mux.Vars(req)
		existingBLOBIDOrSUID := vars[URLPlaceholder_blobIDOrSUUID]
		if !s.blobRequestHandler.HandleRead(data.appQName, data.wsid, data.header, reqCtxWithAttribs,
			newBLOBOKResponseIniter(rw, http.StatusOK), func(sysErr coreutils.SysError) {
				writeCommonError_V1(rw, sysErr, sysErr.HTTPStatus)
			}, existingBLOBIDOrSUID, s.requestSender, iblobstoragestg.RLimiter_Null) {
			rw.WriteHeader(http.StatusServiceUnavailable)
			rw.Header().Add("Retry-After", strconv.Itoa(DefaultRetryAfterSecondsOn503))
		}
	})
}

func newBLOBOKResponseIniter(r http.ResponseWriter, okStatusCode int) func(headersKV ...string) io.Writer {
	return func(headersKV ...string) io.Writer {
		for i := 0; i < len(headersKV); i += 2 {
			r.Header().Set(headersKV[i], headersKV[i+1])
		}
		r.WriteHeader(okStatusCode)
		return r
	}
}
