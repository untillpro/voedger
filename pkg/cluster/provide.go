/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package cluster

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/parser"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/utils/federation"
)

func Provide(cfg *istructsmem.AppConfigType, asp istructs.IAppStructsProvider, time coreutils.ITime,
	federation federation.IFederation, itokens itokens.ITokens, sidecarApps []appparts.SidecarApp) parser.PackageFS {
	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(ClusterPackage, "DeployApp"),
		provideCmdDeployApp(asp, time, sidecarApps)))
	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(ClusterPackage, "VSqlUpdate"),
		provideExecCmdVSqlUpdate(federation, itokens, time, asp)))
	return parser.PackageFS{
		Path: ClusterPackageFQN,
		FS:   schemaFS,
	}
}
