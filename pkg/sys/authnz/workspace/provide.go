/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package workspace

import (
	"net/url"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/projectors"
	"github.com/voedger/voedger/pkg/sys/authnz"
	sysshared "github.com/voedger/voedger/pkg/sys/shared"
	"github.com/voedger/voedger/pkg/vvm"
)

func Provide(cfg *istructsmem.AppConfigType, appDefBuilder appdef.IAppDefBuilder, asp istructs.IAppStructsProvider, now func() time.Time, tokensAPI itokens.ITokens,
	federationURL func() *url.URL) {
	// c.sys.InitChildWorkspace
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		authnz.QNameCommandInitChildWorkspace,
		appDefBuilder.AddStruct(appdef.NewQName(appdef.SysPackage, "InitChildWorkspaceParams"), appdef.DefKind_Object).
			AddField(authnz.Field_WSName, appdef.DataKind_string, true).
			AddField(authnz.Field_WSKind, appdef.DataKind_QName, true).
			AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, false).
			AddField(authnz.Field_WSClusterID, appdef.DataKind_int32, true).
			AddField(field_TemplateName, appdef.DataKind_string, false).
			AddField(Field_TemplateParams, appdef.DataKind_string, false).
			QName(),
		appdef.NullQName,
		appdef.NullQName,
		execCmdInitChildWorkspace,
	))

	// View<ChildWorkspaceIdx>
	// target app, user profile
	projectors.ProvideViewDef(appDefBuilder, QNameViewChildWorkspaceIdx, func(b appdef.IViewBuilder) {
		b.PartKeyDef().AddField(field_dummy, appdef.DataKind_int32, true)
		b.ClustColsDef().AddField(authnz.Field_WSName, appdef.DataKind_string, true)
		b.ValueDef().AddField(Field_ChildWorkspaceID, appdef.DataKind_int64, true)
	})

	// CDoc<ChildWorkspace>
	// many, target app, user profile
	appDefBuilder.AddStruct(authnz.QNameCDocChildWorkspace, appdef.DefKind_CDoc).
		AddField(authnz.Field_WSName, appdef.DataKind_string, true).
		AddField(authnz.Field_WSKind, appdef.DataKind_QName, true).
		AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, false).
		AddField(field_TemplateName, appdef.DataKind_string, false).
		AddField(Field_TemplateParams, appdef.DataKind_string, false).
		AddField(authnz.Field_WSClusterID, appdef.DataKind_int32, true).
		AddField(authnz.Field_WSID, appdef.DataKind_int64, false).    // to be updated afterwards
		AddField(authnz.Field_WSError, appdef.DataKind_string, false) // to be updated afterwards

	// c.sys.CreateWorkspaceID
	// target app, (target cluster, base profile WSID)
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		sysshared.QNameCommandCreateWorkspaceID,
		appDefBuilder.AddStruct(appdef.NewQName(appdef.SysPackage, "CreateWorkspaceIDParams"), appdef.DefKind_Object).
			AddField(Field_OwnerWSID, appdef.DataKind_int64, true).
			AddField(Field_OwnerQName, appdef.DataKind_QName, true).
			AddField(Field_OwnerID, appdef.DataKind_int64, true).
			AddField(Field_OwnerApp, appdef.DataKind_string, true).
			AddField(authnz.Field_WSName, appdef.DataKind_string, true).
			AddField(authnz.Field_WSKind, appdef.DataKind_QName, true).
			AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, false).
			AddField(field_TemplateName, appdef.DataKind_string, false).
			AddField(Field_TemplateParams, appdef.DataKind_string, false).
			QName(),
		appdef.NullQName,
		appdef.NullQName,
		execCmdCreateWorkspaceID(asp, cfg.Name),
	))

	// View<WorkspaceIDIdx>
	projectors.ProvideViewDef(appDefBuilder, QNameViewWorkspaceIDIdx, func(b appdef.IViewBuilder) {
		b.PartKeyDef().AddField(Field_OwnerWSID, appdef.DataKind_int64, true)
		b.ClustColsDef().AddField(authnz.Field_WSName, appdef.DataKind_string, true)
		b.ValueDef().
			AddField(authnz.Field_WSID, appdef.DataKind_int64, true).
			AddField(field_IDOfCDocWorkspaceID, appdef.DataKind_RecordID, false) // TODO: not required for backward compatibility. Actually is required
	})

	// CDoc<WorkspaceID>
	// target app, (target cluster, base profile WSID)
	appDefBuilder.AddStruct(QNameCDocWorkspaceID, appdef.DefKind_CDoc).
		AddField(Field_OwnerWSID, appdef.DataKind_int64, true).
		AddField(Field_OwnerQName, appdef.DataKind_QName, true).
		AddField(Field_OwnerID, appdef.DataKind_int64, true).
		AddField(Field_OwnerApp, appdef.DataKind_string, true).
		AddField(authnz.Field_WSName, appdef.DataKind_string, true).
		AddField(authnz.Field_WSKind, appdef.DataKind_QName, true).
		AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, false).
		AddField(field_TemplateName, appdef.DataKind_string, false).
		AddField(Field_TemplateParams, appdef.DataKind_string, false).
		AddField(authnz.Field_WSID, appdef.DataKind_int64, false)

	// c.sys.CreateWorkspace
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		sysshared.QNameCommandCreateWorkspace,
		appDefBuilder.AddStruct(appdef.NewQName(appdef.SysPackage, "CreateWorkspaceParams"), appdef.DefKind_Object).
			AddField(Field_OwnerWSID, appdef.DataKind_int64, true).
			AddField(Field_OwnerQName, appdef.DataKind_QName, true).
			AddField(Field_OwnerID, appdef.DataKind_int64, true).
			AddField(Field_OwnerApp, appdef.DataKind_string, true).
			AddField(authnz.Field_WSName, appdef.DataKind_string, true).
			AddField(authnz.Field_WSKind, appdef.DataKind_QName, true).
			AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, false).
			AddField(field_TemplateName, appdef.DataKind_string, false).
			AddField(Field_TemplateParams, appdef.DataKind_string, false).
			QName(),
		appdef.NullQName,
		appdef.NullQName,
		execCmdCreateWorkspace(now, asp, cfg.Name),
	))

	// singleton CDoc<sys.WorkspaceDescriptor>
	// target app, new WSID
	appDefBuilder.AddStruct(sysshared.QNameCDocWorkspaceDescriptor, appdef.DefKind_CDoc).
		AddField(Field_OwnerWSID, appdef.DataKind_int64, false). // owner* fields made non-required for app workspaces
		AddField(Field_OwnerQName, appdef.DataKind_QName, false).
		AddField(Field_OwnerID, appdef.DataKind_int64, false).
		AddField(Field_OwnerApp, appdef.DataKind_string, false). // QName -> each target app must know the owner QName -> string
		AddField(authnz.Field_WSName, appdef.DataKind_string, true).
		AddField(authnz.Field_WSKind, appdef.DataKind_QName, true).
		AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, false).
		AddField(field_TemplateName, appdef.DataKind_string, false).
		AddField(Field_TemplateParams, appdef.DataKind_string, false).
		AddField(authnz.Field_WSID, appdef.DataKind_int64, false).
		AddField(Field_CreateError, appdef.DataKind_string, false).
		AddField(authnz.Field_СreatedAtMs, appdef.DataKind_int64, true).
		AddField(Field_InitStartedAtMs, appdef.DataKind_int64, false).
		AddField(sysshared.Field_InitError, appdef.DataKind_string, false).
		AddField(sysshared.Field_InitCompletedAtMs, appdef.DataKind_int64, false).
		AddField(sysshared.Field_Status, appdef.DataKind_int32, false).
		SetSingleton()

	// q.sys.QueryChildWorkspaceByName
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		QNameQueryChildWorkspaceByName,
		appDefBuilder.AddStruct(appdef.NewQName(appdef.SysPackage, "QueryChildWorkspaceByNameParams"), appdef.DefKind_Object).
			AddField(authnz.Field_WSName, appdef.DataKind_string, true).
			QName(),
		appDefBuilder.AddStruct(appdef.NewQName(appdef.SysPackage, "QueryChildWorkspaceByNameResult"), appdef.DefKind_Object).
			AddField(authnz.Field_WSName, appdef.DataKind_string, true).
			AddField(authnz.Field_WSKind, appdef.DataKind_string, true).
			AddField(authnz.Field_WSKindInitializationData, appdef.DataKind_string, true).
			AddField(field_TemplateName, appdef.DataKind_string, true).
			AddField(Field_TemplateParams, appdef.DataKind_string, false).
			AddField(authnz.Field_WSID, appdef.DataKind_int64, false).
			AddField(authnz.Field_WSError, appdef.DataKind_string, false).
			AddField(appdef.SystemField_IsActive, appdef.DataKind_bool, true).
			QName(),
		qcwbnQryExec,
	))

	//register asynchronous projector QNames
	appDefBuilder.AddStruct(qNameAPInitializeWorkspace, appdef.DefKind_Object)
	appDefBuilder.AddStruct(qNameAPInvokeCreateWorkspaceID, appdef.DefKind_Object)
	appDefBuilder.AddStruct(qNameAPInvokeCreateWorkspace, appdef.DefKind_Object)

	ProvideViewNextWSID(appDefBuilder)

	// deactivate workspace
	provideDeactivateWorkspace(cfg, appDefBuilder, tokensAPI, federationURL, asp)
}

// proj.sys.ChildWorkspaceIdx
func ProvideSyncProjectorChildWorkspaceIdxFactory() istructs.ProjectorFactory {
	return func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name: QNameViewChildWorkspaceIdx,
			Func: ChildWorkspaceIdxProjector,
		}
	}
}

// Projector<A, InitializeWorkspace>
func ProvideAsyncProjectorInitializeWorkspace(federationURL func() *url.URL, nowFunc func() time.Time, appQName istructs.AppQName, epWSTemplates vvm.IEPWSTemplates,
	tokensAPI itokens.ITokens, wsPostInitFunc WSPostInitFunc) istructs.ProjectorFactory {
	return func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name: qNameAPInitializeWorkspace,
			Func: initializeWorkspaceProjector(nowFunc, appQName, federationURL, epWSTemplates, tokensAPI, wsPostInitFunc),
		}
	}
}

// Projector<A, InvokeCreateWorkspaceID>
func ProvideAsyncProjectorFactoryInvokeCreateWorkspaceID(federationURL func() *url.URL, appQName istructs.AppQName, tokensAPI itokens.ITokens) istructs.ProjectorFactory {
	return func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name: qNameAPInvokeCreateWorkspaceID,
			Func: invokeCreateWorkspaceIDProjector(federationURL, appQName, tokensAPI),
		}
	}
}

// Projector<A, InvokeCreateWorkspace>
func ProvideAsyncProjectorFactoryInvokeCreateWorkspace(federationURL func() *url.URL, appQName istructs.AppQName, tokensAPI itokens.ITokens) istructs.ProjectorFactory {
	return func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name: qNameAPInvokeCreateWorkspace,
			Func: invokeCreateWorkspaceProjector(federationURL, appQName, tokensAPI),
		}
	}
}

// sp.sys.WorkspaceIDIdx
func ProvideAsyncProjectorWorkspaceIDIdx() istructs.ProjectorFactory {
	return func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name: QNameViewWorkspaceIDIdx,
			Func: workspaceIDIdxProjector,
		}
	}
}
