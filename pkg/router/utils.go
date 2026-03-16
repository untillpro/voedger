/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package router

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/strconvu"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/processors"
)

var onBeforeWriteResponse func(w http.ResponseWriter) // not nil in tests only

func WriteTextResponse(w http.ResponseWriter, msg string, code int) {
	w.Header().Set(httpu.ContentType, "text/plain")
	w.WriteHeader(code)
	writeResponse(w, msg)
}

func ReplyCommonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set(httpu.ContentType, httpu.ContentType_ApplicationJSON)
	w.WriteHeader(code)
	writeCommonError_V2(w, errors.New(msg), code)
}

func ReplyJSON(w http.ResponseWriter, data string, code int) {
	w.Header().Set(httpu.ContentType, httpu.ContentType_ApplicationJSON)
	w.WriteHeader(code)
	writeResponse(w, data)
}

func writeCommonError_V2(w http.ResponseWriter, err error, code int) bool {
	return writeResponse(w, fmt.Sprintf(`{"status":%d,"message":%q}`, code, err.Error()))
}

func writeCommonError_V1(w http.ResponseWriter, err error, code int) bool {
	w.Header().Set(httpu.ContentType, httpu.ContentType_ApplicationJSON)
	w.WriteHeader(code)
	sysErr := coreutils.WrapSysErrorToExact(err, code)
	return writeResponse(w, sysErr.ToJSON_APIV1())
}

func writeResponse(w http.ResponseWriter, data string) bool {
	if onBeforeWriteResponse != nil {
		onBeforeWriteResponse(w)
	}
	if _, err := w.Write([]byte(data)); err != nil { //nolint G705 data is always JSON; Content-Type is set to application/json by all callers
		stack := debug.Stack()
		log.Println("failed to write response:", err, "\n", string(stack))
		return false
	}
	w.(http.Flusher).Flush()
	return true
}

type annoyingErrorsFilter struct {
	w io.Writer
}

func (f *annoyingErrorsFilter) Write(p []byte) (n int, err error) {
	if bytes.Contains(p, []byte("TLS handshake error")) {
		return len(p), nil
	}
	return f.w.Write(p)
}

func replyServiceUnavailable(rw http.ResponseWriter) {
	rw.WriteHeader(http.StatusServiceUnavailable)
	rw.Header().Add("Retry-After", strconv.Itoa(DefaultRetryAfterSecondsOn503))
}

func replyErr(rw http.ResponseWriter, err error) {
	var sysError coreutils.SysError
	if errors.As(err, &sysError) {
		ReplyJSON(rw, sysError.ToJSON_APIV2(), sysError.HTTPStatus)
	} else {
		ReplyCommonError(rw, err.Error(), http.StatusInternalServerError)
	}
}

// copies Authorization cookie (if present) to header (if missing)
// needed for blobs and n10n.
func GetCookieBearerAuth(req *http.Request) (cookieBearerToken string, ok bool, err error) {
	cookie, err := req.Cookie(httpu.Authorization)
	if errors.Is(err, http.ErrNoCookie) {
		return "", false, nil
	}
	if err != nil {
		// notest
		return "", false, fmt.Errorf("failed to read cookie: %w", err)
	}
	if cookieBearerToken, err = url.QueryUnescape(cookie.Value); err != nil {
		return "", false, fmt.Errorf("failed to unescape cookie value %q: %w", cookie.Value, err)
	}
	return cookieBearerToken, true, nil
}

// createBusRequest creates a bus.Request from validated data
func createBusRequest(data validatedData, req *http.Request) bus.Request {
	res := bus.Request{
		Method:   req.Method,
		WSID:     data.wsid,
		Query:    map[string]string{},
		Header:   data.header,
		AppQName: data.appQName,
		Resource: data.vars[URLPlaceholder_resourceName],
		Body:     data.body,
	}

	if docIDStr, hasDocID := data.vars[URLPlaceholder_id]; hasDocID {
		docIDUint64, err := strconvu.ParseUint64(docIDStr)
		if err != nil {
			// notest: parsed already by route regexp
			panic(err)
		}
		res.DocID = istructs.IDType(docIDUint64)
	}

	for k, v := range req.URL.Query() {
		res.Query[k] = v[0]
	}
	return res
}

func withLogAttribs(ctx context.Context, data validatedData, busRequest bus.Request, req *http.Request) context.Context {
	extension := busRequest.Resource
	if busRequest.IsAPIV2 {
		if busRequest.QName == appdef.NullQName {
			extension = apiPathToExtension(processors.APIPath(busRequest.APIPath))
		} else {
			extension = busRequest.QName.String()
		}
	}
	newReqID := fmt.Sprintf("%s-%d", globalServerStartTime, reqID.Add(1))
	return logger.WithContextAttrs(ctx, map[string]any{
		logger.LogAttr_ReqID:     newReqID,
		logger.LogAttr_WSID:      data.wsid,
		logger.LogAttr_VApp:      data.appQName,
		logger.LogAttr_Extension: extension,
		logAttrib_Origin:         req.Header.Get(httpu.Origin),
	})
}

func logServeRequest(ctx context.Context) {
	if logger.IsVerbose() {
		logger.LogCtx(ctx, 1, logger.LogLevelVerbose, "request accepted")
	}
}

func apiPathToExtension(apiPath processors.APIPath) string {
	switch apiPath {
	case processors.APIPath_Docs:
		return "sys._Docs"
	case processors.APIPath_CDocs:
		return "sys._CDocs"
	case processors.APIPaths_Schema:
		return "sys._Schema"
	case processors.APIPath_Schemas_WorkspaceRoles:
		return "sys._Schemas_WorkspaceRoles"
	case processors.APIPath_Schemas_WorkspaceRole:
		return "sys._Schemas_WorkspaceRole"
	case processors.APIPath_Auth_Login:
		return "sys._Auth_Login"
	case processors.APIPath_Auth_Refresh:
		return "sys._Auth_Refresh"
	case processors.APIPath_Users:
		return "sys._Users"
	}
	return strconv.Itoa(int(apiPath))
}
