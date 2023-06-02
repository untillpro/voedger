/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package registryapp

import (
	"github.com/voedger/voedger/pkg/appdef"
	istructsmem "github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/authnz/signupin"
	"github.com/voedger/voedger/pkg/sys/smtp"
	"github.com/voedger/voedger/pkg/vvm"
)

func Provide(smtpCfg smtp.Cfg) vvm.VVMAppBuilder {
	return func(vvmCfg *vvm.VVMConfig, vvmAPI vvm.VVMAPI, cfg *istructsmem.AppConfigType, appDefBuilder appdef.IAppDefBuilder, sep vvm.IStandardExtensionPoints) {

		// sys package
		sys.Provide(vvmCfg.TimeFunc, cfg, appDefBuilder, vvmAPI, smtpCfg, sep, nil, vvmCfg.NumCommandProcessors)

		// sys/registry resources
		// note: q.sys.RefreshPrincipalToken is moved to sys package because it is strange to call it in sys/registry: provided token is issued for different app (e.g. airs-bp)
		signupin.Provide(cfg, appDefBuilder, vvmAPI.ITokens, vvmAPI.FederationURL, vvmAPI.IAppStructsProvider)
		cfg.AddSyncProjectors(
			signupin.ProvideSyncProjectorLoginIdxFactory(),
		)
	}
}
