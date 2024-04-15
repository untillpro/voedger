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
)

func Provide(cfg *istructsmem.AppConfigType, appDefBuilder appdef.IAppDefBuilder, asp istructs.IAppStructsProvider, timeFunc coreutils.TimeFunc, tokensAPI itokens.ITokens,
	federation coreutils.IFederation, itokens itokens.ITokens, ep extensionpoints.IExtensionPoint, wsPostInitFunc WSPostInitFunc) {
	// c.sys.InitChildWorkspace
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		authnz.QNameCommandInitChildWorkspace,
		provideExecCmdInitChildWorkspace(cfg.AppDef),
	))

	// c.sys.CreateWorkspaceID
	// target app, (target cluster, base profile WSID)
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		QNameCommandCreateWorkspaceID,
		execCmdCreateWorkspaceID(asp, cfg.Name),
	))

	// c.sys.CreateWorkspace
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		QNameCommandCreateWorkspace,
		execCmdCreateWorkspace(timeFunc, asp, cfg.Name),
	))

	// q.sys.QueryChildWorkspaceByName
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		QNameQueryChildWorkspaceByName,
		qcwbnQryExec,
	))

	provideViewNextWSID(appDefBuilder)

	// deactivate workspace
	provideDeactivateWorkspace(cfg, tokensAPI, federation)

	// projectors
	cfg.AddAsyncProjectors(
		asyncProjectorInvokeCreateWorkspace(federation, itokens),
		asyncProjectorInvokeCreateWorkspaceID(federation, itokens),
		asyncProjectorInitializeWorkspace(federation, timeFunc, ep, itokens, wsPostInitFunc),
	)
	cfg.AddSyncProjectors(
		syncProjectorChildWorkspaceIdx(),
		syncProjectorWorkspaceIDIdx(),
	)
}

// proj.sys.ChildWorkspaceIdx
func syncProjectorChildWorkspaceIdx() istructs.Projector {
	return istructs.Projector{
		Name: QNameProjectorChildWorkspaceIdx,
		Func: childWorkspaceIdxProjector,
	}
}

// Projector<A, InitializeWorkspace>
func asyncProjectorInitializeWorkspace(federation coreutils.IFederation, nowFunc coreutils.TimeFunc, ep extensionpoints.IExtensionPoint,
	tokensAPI itokens.ITokens, wsPostInitFunc WSPostInitFunc) istructs.Projector {
	return istructs.Projector{
		Name: qNameAPInitializeWorkspace,
		Func: initializeWorkspaceProjector(nowFunc, federation, ep, tokensAPI, wsPostInitFunc),
	}
}

// Projector<A, InvokeCreateWorkspaceID>
func asyncProjectorInvokeCreateWorkspaceID(federation coreutils.IFederation, tokensAPI itokens.ITokens) istructs.Projector {
	return istructs.Projector{
		Name: qNameAPInvokeCreateWorkspaceID,
		Func: invokeCreateWorkspaceIDProjector(federation, tokensAPI),
	}
}

// Projector<A, InvokeCreateWorkspace>
func asyncProjectorInvokeCreateWorkspace(federation coreutils.IFederation, tokensAPI itokens.ITokens) istructs.Projector {
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
