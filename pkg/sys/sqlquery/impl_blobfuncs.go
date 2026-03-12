/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package sqlquery

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/blastrain/vitess-sqlparser/sqlparser"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/strconvu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
)

const (
	blobFuncBlobInfo = "blobinfo"
	blobFuncBlobText = "blobtext"
	blobTextMaxBytes = 10000
)

type blobFuncDesc struct {
	funcName  string // "blobinfo" or "blobtext"
	fieldName string // blob field name
	startFrom uint64 // for blobtext, optional byte offset
}

func executeBlobFunctions(
	ctx context.Context,
	blobFuncs []blobFuncDesc,
	blobHandlerPtr blobprocessor.IRequestHandlerPtr,
	requestSenderPtr bus.IRequestSenderPtr,
	appQName appdef.AppQName,
	wsid istructs.WSID,
	ownerRecord appdef.QName,
	ownerID istructs.RecordID,
	token string,
) (map[string]interface{}, error) {
	header := map[string]string{
		httpu.Authorization: httpu.BearerPrefix + token,
	}

	result := make(map[string]interface{})

	// Group blob functions by field name so we make one HandleRead_V2 call per field
	type fieldRequest struct {
		wantInfo  bool
		wantText  bool
		startFrom uint64
	}

	var fieldOrder []string
	fieldRequests := map[string]*fieldRequest{}

	for _, bf := range blobFuncs {
		fr, exists := fieldRequests[bf.fieldName]
		if !exists {
			fr = &fieldRequest{}
			fieldRequests[bf.fieldName] = fr
			fieldOrder = append(fieldOrder, bf.fieldName)
		}
		switch bf.funcName {
		case blobFuncBlobInfo:
			fr.wantInfo = true
		case blobFuncBlobText:
			fr.wantText = true
			fr.startFrom = bf.startFrom
		}
	}

	for _, fieldName := range fieldOrder {
		fr := fieldRequests[fieldName]
		info, text, err := executeBlobRead(ctx, fieldName, fr.wantText, fr.startFrom,
			blobHandlerPtr, requestSenderPtr, appQName, wsid, ownerRecord, ownerID, header)
		if err != nil {
			return nil, err
		}
		if fr.wantInfo {
			result[fmt.Sprintf("blobinfo(%s)", fieldName)] = info
		}
		if fr.wantText {
			result[fmt.Sprintf("blobtext(%s)", fieldName)] = text
		}
	}

	return result, nil
}

// executeBlobRead makes a single HandleRead_V2 call and returns both blob metadata (info)
// and blob content (text). If wantText is false, blob content is discarded.
func executeBlobRead(
	ctx context.Context,
	fieldName string,
	wantContent bool,
	startFrom uint64,
	blobHandlerPtr blobprocessor.IRequestHandlerPtr,
	requestSenderPtr bus.IRequestSenderPtr,
	appQName appdef.AppQName,
	wsid istructs.WSID,
	ownerRecord appdef.QName,
	ownerID istructs.RecordID,
	header map[string]string,
) (info interface{}, text interface{}, err error) {
	capturedHeaders := make(map[string]string)
	var blobErr error

	var contentWriter = io.Discard
	var rLimiter iblobstorage.RLimiterType
	var writer *blobTextCapture
	if wantContent {
		writer = newBlobTextCapture(startFrom, blobTextMaxBytes)
		contentWriter = writer
		rLimiter = writer.limit
	} else {
		rLimiter = stopReadImmediately
	}

	okResponseIniter := func(headersKeyValue ...string) io.Writer {
		for i := 0; i < len(headersKeyValue); i += 2 {
			capturedHeaders[headersKeyValue[i]] = headersKeyValue[i+1]
		}
		return contentWriter
	}

	errorResponder := func(sysErr coreutils.SysError) {
		blobErr = fmt.Errorf("blob read error: %w", sysErr)
	}

	ok := (*blobHandlerPtr).HandleRead_V2(appQName, wsid, header, ctx,
		okResponseIniter, errorResponder,
		ownerRecord, fieldName, ownerID, *requestSenderPtr, rLimiter)

	if !ok {
		return nil, nil, coreutils.NewHTTPErrorf(http.StatusServiceUnavailable)
	}
	if blobErr != nil {
		return nil, nil, blobErr
	}

	// Build info metadata
	infoMap := map[string]interface{}{
		"name":     capturedHeaders[coreutils.BlobName],
		"mimetype": capturedHeaders[httpu.ContentType],
	}
	if sizeStr, ok := capturedHeaders[httpu.ContentLength]; ok {
		if size, parseErr := strconvu.ParseUint64(sizeStr); parseErr == nil {
			infoMap["size"] = size
		}
	}
	info = infoMap

	// Build text content if requested
	if wantContent {
		contentType := capturedHeaders[httpu.ContentType]
		data := writer.Bytes()
		if isTextMIME(contentType) {
			text = string(data)
		} else {
			text = base64.StdEncoding.EncodeToString(data)
		}
	}

	return info, text, nil
}

func isTextMIME(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "text/") || ct == httpu.ContentType_ApplicationJSON
}

func stopReadImmediately(uint64) error {
	return iblobstorage.ErrReadLimitReached
}

func newBlobTextCapture(startFrom uint64, maxBytes uint64) *blobTextCapture {
	return &blobTextCapture{startFrom: startFrom, endPos: startFrom + maxBytes}
}

func (w *blobTextCapture) limit(wantReadBytes uint64) error {
	if w.blobPos >= w.endPos {
		return iblobstorage.ErrReadLimitReached
	}
	w.blobPos += wantReadBytes
	return nil
}

func (w *blobTextCapture) Write(p []byte) (int, error) {
	chunkStart := w.blobPos - uint64(len(p))
	from := max(w.startFrom, chunkStart) - chunkStart
	to := min(w.endPos, w.blobPos) - chunkStart
	if from < to {
		w.buf = append(w.buf, p[from:to]...)
	}
	return len(p), nil
}

func (w *blobTextCapture) Bytes() []byte {
	return w.buf
}

func blobFuncResultToJSON(blobResults map[string]interface{}) (string, error) {
	bb, err := json.Marshal(blobResults)
	if err != nil {
		// notest
		return "", err
	}
	return string(bb), nil
}

func parseFuncExpr(funcExpr *sqlparser.FuncExpr, sourceTableType appdef.IType) (blobFuncDesc, error) {
	switch funcExpr.Name.Lowered() {
	case blobFuncBlobInfo, blobFuncBlobText:
		return parseBlobFuncExpr(funcExpr, sourceTableType)
	default:
		return blobFuncDesc{}, fmt.Errorf("unsupported function: %s", funcExpr.Name.String())
	}
}

func parseBlobFuncExpr(funcExpr *sqlparser.FuncExpr, sourceTableType appdef.IType) (blobFuncDesc, error) {
	funcName := funcExpr.Name.Lowered()
	if len(funcExpr.Exprs) == 0 {
		return blobFuncDesc{}, fmt.Errorf("%s requires at least one argument (field name)", funcName)
	}

	// First argument: field name
	firstArg, ok := funcExpr.Exprs[0].(*sqlparser.AliasedExpr)
	if !ok {
		// notest: do not know how to trigger
		return blobFuncDesc{}, fmt.Errorf("%s: first argument must be a field name", funcName)
	}
	colName, ok := firstArg.Expr.(*sqlparser.ColName)
	if !ok {
		return blobFuncDesc{}, fmt.Errorf("%s: first argument must be a field name", funcName)
	}
	fieldName := colName.Name.String()
	if sourceTableWithFields, ok := sourceTableType.(appdef.IWithFields); ok {
		fieldName = recoverFieldName(sourceTableWithFields, fieldName)
	} else {
		// notest
		panic("impossible")
	}

	bf := blobFuncDesc{
		funcName:  funcName,
		fieldName: fieldName,
	}

	// Second optional argument: startFrom (only for blobtext)
	if len(funcExpr.Exprs) > 1 {
		if funcName != blobFuncBlobText {
			return blobFuncDesc{}, fmt.Errorf("%s does not accept a second argument", funcName)
		}
		secondArg, ok := funcExpr.Exprs[1].(*sqlparser.AliasedExpr)
		if !ok {
			// notest: do not know how to trigger
			return blobFuncDesc{}, fmt.Errorf("%s: second argument must be a number", funcName)
		}
		sqlVal, ok := secondArg.Expr.(*sqlparser.SQLVal)
		if !ok || sqlVal.Type != sqlparser.IntVal {
			return blobFuncDesc{}, fmt.Errorf("%s: second argument (startFrom) must be an integer", funcName)
		}
		startFrom, err := strconvu.ParseUint64(string(sqlVal.Val))
		if err != nil {
			return blobFuncDesc{}, fmt.Errorf("%s: invalid startFrom value: %w", funcName, err)
		}
		bf.startFrom = startFrom
	}

	if len(funcExpr.Exprs) > 2 {
		return blobFuncDesc{}, fmt.Errorf("%s accepts at most 2 arguments", funcName)
	}

	return bf, nil
}

func mergeJSONWithBlobResults(recJSON string, blobResults map[string]interface{}) (string, error) {
	var recData map[string]interface{}
	if err := json.Unmarshal([]byte(recJSON), &recData); err != nil {
		return "", err
	}
	for k, v := range blobResults {
		recData[k] = v
	}
	bb, err := json.Marshal(recData)
	if err != nil {
		return "", err
	}
	return string(bb), nil
}
