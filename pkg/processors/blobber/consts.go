/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package blobprocessor

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iblobstorage"
)

const (
	temporaryBLOBIDLenTreshold = 40 // greater -> temporary, persistent oherwise
	branchReadBLOB             = "readBLOB"
	branchWriteBLOB            = "writeBLOB"
	attrOwnerQName             = "ownerqname"
	attrOwnerField             = "ownerfield"
	attrOwnerID                = "ownerid"
	attrBlobID                 = "blobid"
)

var (
	durationToRegisterFuncs = map[iblobstorage.DurationType]appdef.QName{
		iblobstorage.DurationType_1Day: appdef.NewQName(appdef.SysPackage, "RegisterTempBLOB1d"),
	}
	registerPersistentBLOBFuncQName = appdef.NewQName(appdef.SysPackage, "UploadBLOBHelper")
	downloadPersistentBLOBFuncQName = appdef.NewQName(appdef.SysPackage, "DownloadBLOBAuthnz")
)
