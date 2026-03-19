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
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
)

type blobWorkpiece struct {
	pipeline.IWorkpiece
	blobMessage      any
	blobMessageWrite *implIBLOBMessage_Write
	blobMessageRead  *implIBLOBMessage_Read
	logCtx           context.Context
	duration         iblobstorage.DurationType
	blobName         []string
	blobContentType  []string
	ttl              string
	descr            iblobstorage.DescrType
	mediaType        string
	boundary         string
	contentType      string
	newBLOBID        istructs.RecordID
	newSUUID         iblobstorage.SUUID
	blobState        iblobstorage.BLOBState
	blobKey          iblobstorage.IBLOBKey
	writer           io.Writer
	registerFuncName appdef.QName
	resultErr        error
	uploadedSize     uint64
	registerFuncBody string
}

type implIBLOBMessage_base struct {
	appQName         appdef.AppQName
	wsid             istructs.WSID
	header           map[string]string
	requestCtx       context.Context
	okResponseIniter func(headersKeyValue ...string) io.Writer
	errorResponder   ErrorResponder
	done             chan interface{}
	requestSender    bus.IRequestSender
	isAPIv2          bool
}

type implIBLOBMessage_Read struct {
	implIBLOBMessage_base

	// APIv1
	existingBLOBIDOrSUUID string
	rLimiter              iblobstorage.RLimiterType

	// APIv2
	ownerRecord      appdef.QName
	ownerRecordField appdef.FieldName
	ownerID          istructs.RecordID
}

type implIBLOBMessage_Write struct {
	implIBLOBMessage_base
	reader           io.ReadCloser
	urlQueryValues   url.Values
	ownerRecord      appdef.QName
	ownerRecordField string
	appParts         appparts.IAppPartitions
}

type WLimiterFactory func() iblobstorage.WLimiterType

type BLOBServiceChannel iprocbus.ServiceChannel

type BLOBServiceChannelGroupIdx uint

type badRequestWrapper struct {
	pipeline.NOOP
}

type sendWriteResult struct {
	pipeline.NOOP
}

// [~server.apiv2.blobs/cmp.blobber.ServicePipeline_catchReadError~impl]
type catchReadError struct {
	pipeline.NOOP
}

type blobReadOrWriteSwitch struct {
}

func (b *blobWorkpiece) Release() {
	b.blobMessage.(pipeline.IWorkpiece).Release()
}

type implIRequestHandler struct {
	procbus      iprocbus.IProcBus
	chanGroupIdx BLOBServiceChannelGroupIdx
	appParts     appparts.IAppPartitions
}

func (m *implIBLOBMessage_base) Release() {
	close(m.done)
}

type IRequestHandlerPtr *IRequestHandler
