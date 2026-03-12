/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sysprovide

import (
	"runtime/debug"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/parser"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/blobber"
	"github.com/voedger/voedger/pkg/sys/builtin"
	"github.com/voedger/voedger/pkg/sys/collection"
	"github.com/voedger/voedger/pkg/sys/describe"
	"github.com/voedger/voedger/pkg/sys/invite"
	"github.com/voedger/voedger/pkg/sys/journal"
	"github.com/voedger/voedger/pkg/sys/smtp"
	"github.com/voedger/voedger/pkg/sys/sqlquery"
	"github.com/voedger/voedger/pkg/sys/uniques"
	"github.com/voedger/voedger/pkg/sys/verifier"
	"github.com/voedger/voedger/pkg/sys/workspace"
)

func ProvideStateless(sr istructsmem.IStatelessResources, smtpCfg smtp.Cfg, eps map[appdef.AppQName]extensionpoints.IExtensionPoint, buildInfo *debug.BuildInfo,
	storageProvider istorage.IAppStorageProvider, wsPostInitFunc workspace.WSPostInitFunc, time timeu.ITime,
	itokens itokens.ITokens, federation federation.IFederation, asp istructs.IAppStructsProvider, atf payloads.IAppTokensFactory,
	blobHandlerPtr blobprocessor.IRequestHandlerPtr, requestSenderPtr bus.IRequestSenderPtr) {
	blobber.ProvideBlobberCmds(sr)
	collection.Provide(sr)
	journal.Provide(sr, eps)
	builtin.Provide(sr, buildInfo, storageProvider)
	workspace.Provide(sr, time, itokens, federation, itokens, wsPostInitFunc, eps)
	sqlquery.Provide(sr, federation, itokens, blobHandlerPtr, requestSenderPtr)
	verifier.Provide(sr, itokens, federation, asp, smtpCfg)
	authnz.Provide(sr, itokens, atf)
	invite.Provide(sr, time, federation, itokens, smtpCfg)
	uniques.Provide(sr)
	describe.Provide(sr)
}

func Provide(cfg *istructsmem.AppConfigType) parser.PackageFS {
	verifier.ProvideLimits(cfg)
	builtin.ProvideCUDValidators(cfg)
	builtin.ProvideSysIsActiveValidation(cfg)
	uniques.ProvideEventValidator(cfg)
	blobber.ProvideBlobberCUDValidators(cfg)
	return parser.PackageFS{
		Path: appdef.SysPackage,
		FS:   sys.SysFS,
	}
}
