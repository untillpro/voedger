/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package registryapp

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/apps"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/parser"
	"github.com/voedger/voedger/pkg/registry"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/smtp"
)

func Provide(smtpCfg smtp.Cfg) apps.AppBuilder {
	return func(apis apps.APIs, cfg *istructsmem.AppConfigType, appDefBuilder appdef.IAppDefBuilder, ep extensionpoints.IExtensionPoint) apps.BuiltInAppDef {

		// sys package
		sysPackageFS := sys.Provide(cfg, appDefBuilder, smtpCfg, ep, nil, apis.TimeFunc, apis.ITokens, apis.IFederation, apis.IAppStructsProvider, apis.IAppTokensFactory,
			apis.NumCommandProcessors, nil, apis.IAppStorageProvider)

		// sys/registry resources
		registryPackageFS := registry.Provide(cfg, apis.IAppStructsProvider, apis.ITokens, apis.IFederation)
		cfg.AddSyncProjectors(registry.ProvideSyncProjectorLoginIdx())
		registryAppPackageFS := parser.PackageFS{
			Path: RegistryAppFQN,
			FS:   registryAppSchemaFS,
		}

		return apps.BuiltInAppDef{
			AppQName: istructs.AppQName_sys_registry,
			Packages: []parser.PackageFS{sysPackageFS, registryPackageFS, registryAppPackageFS},
			AppDeploymentDescriptor: cluster.AppDeploymentDescriptor{
				PartsCount:     int(apis.NumCommandProcessors),
				EnginePoolSize: cluster.PoolSize(int(apis.NumCommandProcessors), DefDeploymentQPCount, int(apis.NumCommandProcessors)),
			},
		}
	}
}
