/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package blobprocessor

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/strconvu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors"
)

func getRegisterFunc(ctx context.Context, bw *blobWorkpiece) (err error) {
	if bw.isPersistent() {
		bw.registerFuncName = registerPersistentBLOBFuncQName
		bw.registerFuncBody = fmt.Sprintf(`{"args":{"OwnerRecord":"%s","OwnerRecordField":"%s"}}`,
			bw.blobMessageWrite.ownerRecord, bw.blobMessageWrite.ownerRecordField)
	} else {
		registerFuncName, ok := durationToRegisterFuncs[bw.duration]
		if !ok {
			// notest
			return coreutils.NewHTTPErrorf(http.StatusBadRequest, "unsupported blob duration value: ", bw.duration)
		}
		bw.registerFuncName = registerFuncName
		bw.registerFuncBody = "{}"
	}
	return nil
}

func getBLOBKeyWrite(ctx context.Context, bw *blobWorkpiece) (err error) {
	if bw.isPersistent() {
		bw.blobKey = &iblobstorage.PersistentBLOBKeyType{
			ClusterAppID: istructs.ClusterAppID_sys_blobber,
			WSID:         bw.blobMessageWrite.wsid,
			BlobID:       bw.newBLOBID,
		}
	} else {
		// temp
		bw.blobKey = &iblobstorage.TempBLOBKeyType{
			ClusterAppID: istructs.ClusterAppID_sys_blobber,
			WSID:         bw.blobMessageWrite.wsid,
			SUUID:        bw.newSUUID,
		}
	}
	return nil
}

func provideWriteBLOB(blobStorage iblobstorage.IBLOBStorage, wLimiterFactory WLimiterFactory) func(ctx context.Context, bw *blobWorkpiece) (err error) {
	return func(ctx context.Context, bw *blobWorkpiece) (err error) {
		wLimiter := wLimiterFactory()
		if bw.isPersistent() {
			key := (bw.blobKey).(*iblobstorage.PersistentBLOBKeyType)
			bw.uploadedSize, err = blobStorage.WriteBLOB(bw.blobMessageWrite.requestCtx, *key, bw.descr, bw.blobMessageWrite.reader, wLimiter)
		} else {
			key := (bw.blobKey).(*iblobstorage.TempBLOBKeyType)
			bw.uploadedSize, err = blobStorage.WriteTempBLOB(ctx, *key, bw.descr, bw.blobMessageWrite.reader, wLimiter, bw.duration)
		}
		if errors.Is(err, iblobstorage.ErrBLOBSizeQuotaExceeded) {
			return coreutils.NewHTTPError(http.StatusRequestEntityTooLarge, err)
		}
		if err == nil {
			logger.VerboseCtx(bw.logCtx, "bp.write.success")
		}
		return err
	}
}

func setBLOBStatusCompleted(ctx context.Context, bw *blobWorkpiece) (err error) {
	if !bw.isPersistent() {
		// do not account statuses for temp blobs
		return nil
	}
	// set WDoc<sys.BLOB>.status = BLOBStatus_Completed
	req := bus.Request{
		Method:   http.MethodPost,
		WSID:     bw.blobMessageWrite.wsid,
		AppQName: bw.blobMessageWrite.appQName,
		Resource: "c.sys.CUD",
		Body:     []byte(fmt.Sprintf(`{"cuds":[{"sys.ID": %d,"fields":{"status":%d}}]}`, bw.newBLOBID, iblobstorage.BLOBStatus_Completed)),
		Header:   bw.blobMessageWrite.header,
		Host:     httpu.LocalhostIP.String(),
	}
	_, _, err = bus.GetCommandResponse(bw.blobMessageWrite.requestCtx, bw.blobMessageWrite.requestSender, req)
	if err == nil {
		logger.VerboseCtx(bw.logCtx, "bp.setcompleted.success")
	}
	return err
}

func registerBLOB(ctx context.Context, bw *blobWorkpiece) (err error) {
	req := bus.Request{
		Method:   http.MethodPost,
		WSID:     bw.blobMessageWrite.wsid,
		AppQName: bw.blobMessageWrite.appQName,
		Header:   bw.blobMessageWrite.header,
		Body:     []byte(bw.registerFuncBody),
		Host:     httpu.LocalhostIP.String(),
		APIPath:  int(processors.APIPath_Commands),
		QName:    bw.registerFuncName,
		IsAPIV2:  true,
	}
	_, blobHelperResp, sysErr := bus.GetCommandResponse(bw.blobMessageWrite.requestCtx, bw.blobMessageWrite.requestSender, req)
	if sysErr != nil {
		return fmt.Errorf("%s failed: %w", bw.registerFuncName.String(), sysErr)
	}
	var blobIDStr string
	if bw.isPersistent() {
		bw.newBLOBID = blobHelperResp.NewIDs["1"]
		blobIDStr = strconvu.UintToString(bw.newBLOBID)
	} else {
		bw.newSUUID = iblobstorage.NewSUUID()
		blobIDStr = string(bw.newSUUID)
	}
	bw.logCtx = logger.WithContextAttrs(bw.logCtx, map[string]any{
		attrBlobID: blobIDStr,
	})
	logger.VerboseCtx(bw.logCtx, "bp.register.success")
	return nil
}

func getBLOBMessageWrite(_ context.Context, bw *blobWorkpiece) error {
	bw.blobMessageWrite = bw.blobMessage.(*implIBLOBMessage_Write)
	bw.logCtx = bw.blobMessageWrite.requestCtx
	if bw.blobMessageWrite.ownerRecord != appdef.NullQName {
		bw.logCtx = logger.WithContextAttrs(bw.logCtx, map[string]any{
			attrOwnerQName: bw.blobMessageWrite.ownerRecord.String(),
			attrOwnerField: bw.blobMessageWrite.ownerRecordField,
		})
	}
	return nil
}

func parseQueryParams(_ context.Context, bw *blobWorkpiece) error {
	if bw.blobMessageWrite.isAPIv2 {
		bw.blobName = append(bw.blobName, bw.blobMessageWrite.header[coreutils.BlobName])
		bw.blobContentType = append(bw.blobContentType, bw.blobMessageWrite.header[httpu.ContentType])
		// camelcased here because textproto.CanonicalMIMEHeaderKey() canonizes TTL to Ttl
		if ttlHeader, ok := bw.blobMessageWrite.header["Ttl"]; ok {
			bw.ttl = ttlHeader
		}
	} else {
		bw.blobName = bw.blobMessageWrite.urlQueryValues["name"]
		bw.blobContentType = bw.blobMessageWrite.urlQueryValues["mimeType"]
		if len(bw.blobMessageWrite.urlQueryValues["ttl"]) > 0 {
			bw.ttl = bw.blobMessageWrite.urlQueryValues["ttl"][0]
		}
	}
	return nil
}

func parseMediaType(_ context.Context, bw *blobWorkpiece) error {
	bw.contentType = bw.blobMessageWrite.header[httpu.ContentType]
	if len(bw.contentType) == 0 {
		return nil
	}
	mediaType, params, err := mime.ParseMediaType(bw.contentType)
	if err != nil {
		return fmt.Errorf("failed to parse Content-Type header: %w", err)
	}
	bw.mediaType = mediaType
	bw.boundary = params["boundary"]
	return nil
}

func validateQueryParams(_ context.Context, bw *blobWorkpiece) error {

	if (len(bw.blobName) > 0 && len(bw.blobContentType) == 0) || (len(bw.blobName) == 0 && len(bw.blobContentType) > 0) {
		return errors.New("both name and mimeType query params must be specified")
	}

	isSingleBLOB := len(bw.blobName) > 0 && len(bw.blobContentType) > 0

	if len(bw.ttl) > 0 {
		duration, ttlSupported := federation.TemporaryBLOB_URLTTLToDurationLs[bw.ttl]
		if !ttlSupported {
			return errors.New(`"1d" is only supported for now for temporary blob ttl`)
		}
		bw.duration = duration
	}

	if bw.blobMessageWrite.isAPIv2 && bw.isPersistent() {
		appDef, err := bw.blobMessageWrite.appParts.AppDef(bw.blobMessageWrite.appQName)
		if err != nil {
			return err
		}
		ownerType := appDef.Type(bw.blobMessageWrite.ownerRecord)
		if ownerType == appdef.NullType {
			return fmt.Errorf("blob owner QName %s is unknown", bw.blobMessageWrite.ownerRecord)
		}
		iFields := ownerType.(appdef.IWithFields)
		ownerField := iFields.Field(bw.blobMessageWrite.ownerRecordField)
		if ownerField == nil {
			return fmt.Errorf("blob owner field %s does not exist in blob owner %s", bw.blobMessageWrite.ownerRecordField,
				bw.blobMessageWrite.ownerRecord)
		}
		if ownerField.DataKind() != appdef.DataKind_RecordID {
			return fmt.Errorf("blob owner %s.%s must be of blob type", bw.blobMessageWrite.ownerRecord, bw.blobMessageWrite.ownerRecordField)
		}
	}

	if isSingleBLOB {
		if bw.contentType == httpu.ContentType_MultipartFormData {
			return fmt.Errorf(`name+mimeType query params and "%s" Content-Type header are mutual exclusive`, httpu.ContentType_MultipartFormData)
		}
		bw.descr.Name = bw.blobName[0]
		bw.descr.ContentType = bw.blobContentType[0]
		logger.VerboseCtx(bw.logCtx, "bp.meta", "name=", bw.descr.Name, ",contenttype=", bw.descr.ContentType)
		return nil
	}

	// not a single BLOB
	if len(bw.contentType) == 0 {
		return errors.New(`neither "name"+"mimeType" query params nor Content-Type header is not provided`)
	}

	if bw.mediaType != httpu.ContentType_MultipartFormData {
		return errors.New("name+mimeType query params are not provided -> Content-Type must be mutipart/form-data but actual is " + bw.contentType)
	}

	if len(bw.boundary) == 0 {
		return fmt.Errorf("boundary of %s is not specified", httpu.ContentType_MultipartFormData)
	}

	logger.VerboseCtx(bw.logCtx, "bp.meta", "name=", bw.descr.Name, ",contenttype=", bw.descr.ContentType)
	return nil
}

func replySuccess_V1(bw *blobWorkpiece) (err error) {
	writer := bw.blobMessageWrite.okResponseIniter(httpu.ContentType, "text/plain")
	if bw.isPersistent() {
		_, err = writer.Write([]byte(strconvu.UintToString(bw.newBLOBID)))
	} else {
		_, err = writer.Write([]byte(bw.newSUUID))
	}
	return err
}

func replySuccess_V2(bw *blobWorkpiece) (err error) {
	writer := bw.blobMessageWrite.okResponseIniter(httpu.ContentType, httpu.ContentType_ApplicationJSON)
	if bw.isPersistent() {
		_, err = fmt.Fprintf(writer, `{"blobID":%d}`, bw.newBLOBID)
	} else {
		_, err = fmt.Fprintf(writer, `{"blobSUUID":"%s"}`, bw.newSUUID)
	}
	return err
}

func (b *sendWriteResult) DoSync(_ context.Context, work pipeline.IWorkpiece) (err error) {
	bw := work.(*blobWorkpiece)
	if bw.resultErr == nil {
		if bw.blobMessageWrite.isAPIv2 {
			err = replySuccess_V2(bw)
		} else {
			err = replySuccess_V1(bw)
		}
		return err
	}
	var sysError coreutils.SysError
	errors.As(bw.resultErr, &sysError)
	bw.blobMessageWrite.errorResponder(sysError)
	return nil
}

func (b *sendWriteResult) OnErr(err error, work interface{}, _ pipeline.IWorkpieceContext) (newErr error) {
	bw := work.(*blobWorkpiece)
	bw.resultErr = coreutils.WrapSysError(err, http.StatusInternalServerError)
	var sysError coreutils.SysError
	if errors.As(bw.resultErr, &sysError) && sysError.HTTPStatus == http.StatusBadRequest {
		logger.ErrorCtx(bw.logCtx, "bp.error", err, ", query=", bw.blobMessageWrite.urlQueryValues, ", headers=", bw.blobMessageWrite.header)
	} else {
		logger.ErrorCtx(bw.logCtx, "bp.error", err)
	}
	return nil
}

func (b *badRequestWrapper) OnErr(err error, _ interface{}, _ pipeline.IWorkpieceContext) (newErr error) {
	return coreutils.WrapSysError(err, http.StatusBadRequest)
}
