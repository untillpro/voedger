/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package blobprocessor

import (
	"context"
	"io"
	"net/url"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
)

func (r *implIRequestHandler) HandleRead(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	okResponseIniter func(headersKeyValue ...string) io.Writer,
	errorResponder ErrorResponder, existingBLOBIDOrSUUID string, requestSender bus.IRequestSender, rLimiter iblobstorage.RLimiterType) bool {
	doneCh := make(chan interface{})
	return r.handle(&implIBLOBMessage_Read{
		implIBLOBMessage_base: implIBLOBMessage_base{
			appQName:         appQName,
			wsid:             wsid,
			header:           header,
			requestCtx:       requestCtx,
			okResponseIniter: okResponseIniter,
			errorResponder:   errorResponder,
			done:             doneCh,
			requestSender:    requestSender,
		},
		existingBLOBIDOrSUUID: existingBLOBIDOrSUUID,
		rLimiter:              rLimiter,
	}, doneCh)
}

// [~server.apiv2.blobs/cmp.blobber.implIRequestHandler_Read2~impl]
func (r *implIRequestHandler) HandleRead_V2(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	okResponseIniter func(headersKeyValue ...string) io.Writer,
	errorResponder ErrorResponder, ownerRecord appdef.QName, ownerRecordField string, ownerID istructs.RecordID,
	requestSender bus.IRequestSender, rLimiter iblobstorage.RLimiterType) bool {
	doneCh := make(chan interface{})
	return r.handle(&implIBLOBMessage_Read{
		implIBLOBMessage_base: implIBLOBMessage_base{
			appQName:         appQName,
			wsid:             wsid,
			header:           header,
			requestCtx:       requestCtx,
			okResponseIniter: okResponseIniter,
			errorResponder:   errorResponder,
			done:             doneCh,
			requestSender:    requestSender,
			isAPIv2:          true,
		},
		ownerRecord:      ownerRecord,
		ownerRecordField: ownerRecordField,
		ownerID:          ownerID,
		rLimiter:         rLimiter,
	}, doneCh)

}

func (r *implIRequestHandler) HandleReadTemp_V2(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	okResponseIniter func(headersKeyValue ...string) io.Writer,
	errorResponder ErrorResponder, requestSender bus.IRequestSender, suuid iblobstorage.SUUID, rLimiter iblobstorage.RLimiterType) bool {
	doneCh := make(chan interface{})
	return r.handle(&implIBLOBMessage_Read{
		implIBLOBMessage_base: implIBLOBMessage_base{
			appQName:         appQName,
			wsid:             wsid,
			header:           header,
			requestCtx:       requestCtx,
			okResponseIniter: okResponseIniter,
			errorResponder:   errorResponder,
			done:             doneCh,
			requestSender:    requestSender,
			isAPIv2:          true,
		},
		existingBLOBIDOrSUUID: string(suuid),
		rLimiter:              rLimiter,
	}, doneCh)

}

func (r *implIRequestHandler) handleWrite(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	urlQueryValues url.Values, okResponseIniter func(headersKeyValue ...string) io.Writer, reader io.ReadCloser,
	errorResponder ErrorResponder, requestSender bus.IRequestSender, isAPIv2 bool, ownerRecord appdef.QName, ownerRecordField string) bool {
	doneCh := make(chan interface{})
	return r.handle(&implIBLOBMessage_Write{
		implIBLOBMessage_base: implIBLOBMessage_base{
			appQName:         appQName,
			wsid:             wsid,
			header:           header,
			requestCtx:       requestCtx,
			okResponseIniter: okResponseIniter,
			errorResponder:   errorResponder,
			done:             doneCh,
			requestSender:    requestSender,
			isAPIv2:          isAPIv2,
		},
		urlQueryValues:   urlQueryValues,
		reader:           reader,
		ownerRecord:      ownerRecord,
		ownerRecordField: ownerRecordField,
		appParts:         r.appParts,
	}, doneCh)
}

func (r *implIRequestHandler) HandleWrite(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	urlQueryValues url.Values, okResponseIniter func(headersKeyValue ...string) io.Writer, reader io.ReadCloser,
	errorResponder ErrorResponder, requestSender bus.IRequestSender) bool {
	return r.handleWrite(appQName, wsid, header, requestCtx, urlQueryValues, okResponseIniter, reader, errorResponder, requestSender, false, appdef.NullQName, "")
}

func (r *implIRequestHandler) HandleWrite_V2(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	okResponseIniter func(headersKeyValue ...string) io.Writer, reader io.ReadCloser,
	errorResponder ErrorResponder, requestSender bus.IRequestSender, ownerRecord appdef.QName, ownerRecordField string) bool {
	return r.handleWrite(appQName, wsid, header, requestCtx, nil, okResponseIniter, reader, errorResponder, requestSender, true, ownerRecord, ownerRecordField)
}

func (r *implIRequestHandler) HandleWriteTemp_V2(appQName appdef.AppQName, wsid istructs.WSID, header map[string]string, requestCtx context.Context,
	okResponseIniter func(headersKeyValue ...string) io.Writer, reader io.ReadCloser,
	errorResponder ErrorResponder, requestSender bus.IRequestSender) bool {
	return r.handleWrite(appQName, wsid, header, requestCtx, nil, okResponseIniter, reader, errorResponder, requestSender, true,
		appdef.NullQName, appdef.NullName)
}

func (r *implIRequestHandler) handle(msg any, doneCh <-chan interface{}) bool {
	if success := r.procbus.Submit(uint(r.chanGroupIdx), 0, msg); !success {
		return false
	}
	<-doneCh
	return true
}
