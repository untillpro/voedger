/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sys

import (
	"embed"
	"runtime/debug"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/parser"
	"github.com/voedger/voedger/pkg/projectors"
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
	coreutils "github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/utils/federation"
)

//go:embed *.vsql
var SysFS embed.FS

func ProvideStateless(sr istructsmem.IStatelessResources, smtpCfg smtp.Cfg, eps map[appdef.AppQName]extensionpoints.IExtensionPoint, buildInfo *debug.BuildInfo,
	storageProvider istorage.IAppStorageProvider, wsPostInitFunc workspace.WSPostInitFunc, timeFunc coreutils.TimeFunc,
	itokens itokens.ITokens, federation federation.IFederation, asp istructs.IAppStructsProvider, atf payloads.IAppTokensFactory) {
	blobber.ProvideBlobberCmds(sr)
	collection.Provide(sr)
	journal.Provide(sr, eps)
	builtin.Provide(sr, buildInfo, storageProvider)
	workspace.Provide(sr, timeFunc, itokens, federation, itokens, wsPostInitFunc, eps)
	sqlquery.Provide(sr, asp)
	verifier.Provide(sr, itokens, federation, asp, smtpCfg, timeFunc)
	authnz.Provide(sr, itokens, atf)
	invite.Provide(sr, timeFunc, federation, itokens, smtpCfg)
	uniques.Provide(sr)
	describe.Provide(sr)
}

func Provide(cfg *istructsmem.AppConfigType) parser.PackageFS {
	verifier.ProvideLimits(cfg)
	projectors.ProvideOffsetsDef(cfg.AppDefBuilder())
	workspace.ProvideViewNextWSID(cfg.AppDefBuilder())
	builtin.ProvideCUDValidators(cfg)
	builtin.ProvideSysIsActiveValidation(cfg)
	uniques.ProvideEventValidator(cfg)
	return parser.PackageFS{
		Path: appdef.SysPackage,
		FS:   SysFS,
	}
}
