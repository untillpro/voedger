/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sqlquery

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
)

func Provide(sr istructsmem.IStatelessResources, federation federation.IFederation, itokens itokens.ITokens,
	blobHandlerPtr blobprocessor.IRequestHandlerPtr, requestSenderPtr bus.IRequestSenderPtr) {
	sr.AddQueries(appdef.SysPackagePath, istructsmem.NewQueryFunction(
		appdef.NewQName(appdef.SysPackage, "SqlQuery"),
		provideExecQrySQLQuery(federation, itokens, blobHandlerPtr, requestSenderPtr),
	))
}
