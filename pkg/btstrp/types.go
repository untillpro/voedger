/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package btstrp

import (
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/iblobstoragestg"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
	dbcertcache "github.com/voedger/voedger/pkg/vvm/db_cert_cache"
)

type ClusterBuiltInApp appparts.BuiltInApp

// PostWireInterfacePtrs groups placeholders for interfaces that are needed during app wiring,
// but can be created only after app wiring completes. Bootstrap fills these pointer cells later to break wire cycles.
type PostWireInterfacePtrs struct {
	BlobberAppStorage iblobstoragestg.BlobAppStoragePtr
	RouterAppStorage  dbcertcache.RouterAppStoragePtr
	BlobHandler       blobprocessor.IRequestHandlerPtr
	RequestSender     bus.IRequestSenderPtr
}
