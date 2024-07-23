/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package workspace

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/sys/authnz"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/utils/federation"
)

func ProvideViewNextWSID(adf appdef.IAppDefBuilder) {
	provideViewNextWSID(adf)
}

func Provide(sr istructsmem.IStatelessResources, timeFunc coreutils.TimeFunc, tokensAPI itokens.ITokens,
	federation federation.IFederation, itokens itokens.ITokens, wsPostInitFunc WSPostInitFunc,
	eps map[appdef.AppQName]extensionpoints.IExtensionPoint) {

	sr.AddCommands(appdef.SysPackagePath,
		// c.sys.InitChildWorkspac
		istructsmem.NewCommandFunction(
			authnz.QNameCommandInitChildWorkspace,
			execCmdInitChildWorkspace,
		),

		// c.sys.CreateWorkspaceID
		// target app, (target cluster, base profile WSID)
		istructsmem.NewCommandFunction(
			QNameCommandCreateWorkspaceID,
			execCmdCreateWorkspaceID,
		),

		// c.sys.CreateWorkspace
		istructsmem.NewCommandFunction(
			QNameCommandCreateWorkspace,
			execCmdCreateWorkspace(timeFunc),
		),
	)

	sr.AddQueries(appdef.SysPackagePath,
		// q.sys.QueryChildWorkspaceByName
		istructsmem.NewQueryFunction(
			QNameQueryChildWorkspaceByName,
			qcwbnQryExec,
		),
	)

	// deactivate workspace
	provideDeactivateWorkspace(sr, tokensAPI, federation)

	sr.AddProjectors(appdef.SysPackagePath,
		asyncProjectorInvokeCreateWorkspace(federation, itokens),
		asyncProjectorInvokeCreateWorkspaceID(federation, itokens),
		asyncProjectorInitializeWorkspace(federation, timeFunc, itokens, wsPostInitFunc, eps),
		syncProjectorChildWorkspaceIdx(),
		syncProjectorWorkspaceIDIdx(),
	)

	// // projectors
	// sprb.AddAsyncProjectors(
	// 	asyncProjectorInvokeCreateWorkspace(federation, itokens),
	// 	asyncProjectorInvokeCreateWorkspaceID(federation, itokens),
	// 	asyncProjectorInitializeWorkspace(federation, timeFunc, itokens, wsPostInitFunc, eps),
	// )
	// sprb.AddSyncProjectors(
	// 	syncProjectorChildWorkspaceIdx(),
	// 	syncProjectorWorkspaceIDIdx(),
	// )
}

// proj.sys.ChildWorkspaceIdx
func syncProjectorChildWorkspaceIdx() istructs.Projector {
	return istructs.Projector{
		Name: QNameProjectorChildWorkspaceIdx,
		Func: childWorkspaceIdxProjector,
	}
}

// Projector<A, InitializeWorkspace>
func asyncProjectorInitializeWorkspace(federation federation.IFederation, nowFunc coreutils.TimeFunc,
	tokensAPI itokens.ITokens, wsPostInitFunc WSPostInitFunc, eps map[appdef.AppQName]extensionpoints.IExtensionPoint) istructs.Projector {
	return istructs.Projector{
		Name: qNameAPInitializeWorkspace,
		Func: initializeWorkspaceProjector(nowFunc, federation, eps, tokensAPI, wsPostInitFunc),
	}
}

// Projector<A, InvokeCreateWorkspaceID>
func asyncProjectorInvokeCreateWorkspaceID(federation federation.IFederation, tokensAPI itokens.ITokens) istructs.Projector {
	return istructs.Projector{
		Name: qNameAPInvokeCreateWorkspaceID,
		Func: invokeCreateWorkspaceIDProjector(federation, tokensAPI),
	}
}

// Projector<A, InvokeCreateWorkspace>
func asyncProjectorInvokeCreateWorkspace(federation federation.IFederation, tokensAPI itokens.ITokens) istructs.Projector {
	return istructs.Projector{
		Name: qNameAPInvokeCreateWorkspace,
		Func: invokeCreateWorkspaceProjector(federation, tokensAPI),
	}
}

// sp.sys.WorkspaceIDIdx
func syncProjectorWorkspaceIDIdx() istructs.Projector {
	return istructs.Projector{
		Name: QNameProjectorViewWorkspaceIDIdx,
		Func: workspaceIDIdxProjector,
	}
}
