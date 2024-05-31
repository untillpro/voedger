/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package federation

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

type IFederation interface {
	Func(relativeURL string, body string, optFuncs ...coreutils.ReqOptFunc) (*coreutils.FuncResponse, error)
	UploadBLOB(appQName appdef.AppQName, wsid istructs.WSID, blobName string, blobMimeType string, blobContent []byte,
		optFuncs ...coreutils.ReqOptFunc) (blobID istructs.RecordID, err error)
	UploadBLOBs(appQName appdef.AppQName, wsid istructs.WSID, blobs []coreutils.BLOB, optFuncs ...coreutils.ReqOptFunc) (blobIDs []istructs.RecordID, err error)
	ReadBLOB(appQName appdef.AppQName, wsid istructs.WSID, blobID istructs.RecordID, optFuncs ...coreutils.ReqOptFunc) (*coreutils.HTTPResponse, error)
	URLStr() string
	Port() int
	N10NUpdate(key in10n.ProjectionKey, val int64, optFuncs ...coreutils.ReqOptFunc) error
	N10NSubscribe(projectionKey in10n.ProjectionKey) (offsetsChan OffsetsChan, unsubscribe func(), err error)
	AdminFunc(relativeURL string, body string, optFuncs ...coreutils.ReqOptFunc) (*coreutils.FuncResponse, error)
}
