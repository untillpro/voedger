/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package workspace

import (
	"context"
	"fmt"
	"net/http"

	"github.com/untillpro/goutils/logger"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/collection"
	"github.com/voedger/voedger/pkg/sys/invite"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func provideDeactivateWorkspace(cfg *istructsmem.AppConfigType, tokensAPI itokens.ITokens, federation coreutils.IFederation,
	asp istructs.IAppStructsProvider) {

	// c.sys.DeactivateWorkspace
	// target app, target WSID
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		qNameCmdInitiateDeactivateWorkspace,
		cmdInitiateDeactivateWorkspaceExec,
	))

	// c.sys.OnWorkspaceDeactivated
	// owner app, owner WSID
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(appdef.SysPackage, "OnWorkspaceDeactivated"),
		cmdOnWorkspaceDeactivatedExec,
	))

	// c.sys.OnJoinedWorkspaceDeactivated
	// target app, profile WSID
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(appdef.SysPackage, "OnJoinedWorkspaceDeactivated"),
		cmdOnJoinedWorkspaceDeactivateExec,
	))

	// c.sys.OnChildWorkspaceDeactivated
	// ownerApp/ownerWSID
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(appdef.SysPackage, "OnChildWorkspaceDeactivated"),
		cmdOnChildWorkspaceDeactivatedExec,
	))

	// target app, target WSID
	cfg.AddAsyncProjectors(istructs.Projector{
		Name: qNameProjectorApplyDeactivateWorkspace,
		Func: projectorApplyDeactivateWorkspace(federation, cfg.Name, tokensAPI, asp),
	})
}

func cmdInitiateDeactivateWorkspaceExec(args istructs.ExecCommandArgs) (err error) {
	kb, err := args.State.KeyBuilder(state.Record, authnz.QNameCDocWorkspaceDescriptor)
	if err != nil {
		// notest
		return err
	}
	kb.PutQName(state.Field_Singleton, authnz.QNameCDocWorkspaceDescriptor)
	wsDesc, err := args.State.MustExist(kb)
	if err != nil {
		// notest
		return err
	}
	status := wsDesc.AsInt32(authnz.Field_Status)
	if status != int32(authnz.WorkspaceStatus_Active) {
		return coreutils.NewHTTPErrorf(http.StatusConflict, "Workspace Status is not Active")
	}

	wsDescUpdater, err := args.Intents.UpdateValue(kb, wsDesc)
	if err != nil {
		// notest
		return err
	}
	wsDescUpdater.PutInt32(authnz.Field_Status, int32(authnz.WorkspaceStatus_ToBeDeactivated))
	return nil
}

func cmdOnJoinedWorkspaceDeactivateExec(args istructs.ExecCommandArgs) (err error) {
	invitedToWSID := args.ArgumentObject.AsInt64(field_InvitedToWSID)
	svCDocJoinedWorkspace, skb, ok, err := invite.GetCDocJoinedWorkspace(args.State, invitedToWSID)
	if err != nil || !ok {
		return err
	}
	if !svCDocJoinedWorkspace.AsBool(appdef.SystemField_IsActive) {
		return nil
	}
	cdocJoinedWorkspaceUpdater, err := args.Intents.UpdateValue(skb, svCDocJoinedWorkspace)
	if err != nil {
		// notest
		return err
	}
	cdocJoinedWorkspaceUpdater.PutBool(appdef.SystemField_IsActive, false)
	return nil
}

// app/pseudoProfileWSID, ownerApp
func cmdOnWorkspaceDeactivatedExec(args istructs.ExecCommandArgs) (err error) {
	ownerWSID := args.ArgumentObject.AsInt64(Field_OwnerWSID)
	wsName := args.ArgumentObject.AsString(authnz.Field_WSName)
	kb, err := args.State.KeyBuilder(state.View, QNameViewWorkspaceIDIdx)
	if err != nil {
		// notest
		return err
	}
	kb.PutInt64(Field_OwnerWSID, ownerWSID)
	kb.PutString(authnz.Field_WSName, wsName)
	viewRec, ok, err := args.State.CanExist(kb)
	if err != nil {
		// notest
		return err
	}
	if !ok {
		logger.Verbose("workspace", wsName, ":", ownerWSID, "is not mentioned in view.sys.WorkspaceIDId")
		return
	}
	idOfCDocWorkspaceID := viewRec.AsRecordID(field_IDOfCDocWorkspaceID)
	kb, err = args.State.KeyBuilder(state.Record, QNameCDocWorkspaceID)
	if err != nil {
		// notest
		return err
	}
	kb.PutRecordID(state.Field_ID, idOfCDocWorkspaceID)
	cdocWorkspaceID, err := args.State.MustExist(kb)
	if err != nil {
		// notest
		return err
	}

	if !cdocWorkspaceID.AsBool(appdef.SystemField_IsActive) {
		logger.Verbose("cdoc.sys.WorkspaceID is inactive already")
		return nil
	}

	cdocWorkspaceIDUpdater, err := args.Intents.UpdateValue(kb, cdocWorkspaceID)
	if err != nil {
		// notest
		return err
	}
	cdocWorkspaceIDUpdater.PutBool(appdef.SystemField_IsActive, false)
	return nil
}

// ownerApp/ownerWSID
func cmdOnChildWorkspaceDeactivatedExec(args istructs.ExecCommandArgs) (err error) {
	ownerID := args.ArgumentObject.AsInt64(Field_OwnerID)
	kb, err := args.State.KeyBuilder(state.Record, appdef.NullQName)
	if err != nil {
		// notest
		return err
	}
	kb.PutRecordID(state.Field_ID, istructs.RecordID(ownerID))
	cdocOwnerSV, err := args.State.MustExist(kb)
	if err != nil {
		// notest
		return err
	}
	if !cdocOwnerSV.AsBool(appdef.SystemField_IsActive) {
		return nil
	}
	cdocOwnerUpdater, err := args.Intents.UpdateValue(kb, cdocOwnerSV)
	if err != nil {
		// notest
		return err
	}
	cdocOwnerUpdater.PutBool(appdef.SystemField_IsActive, false)
	return nil
}

// target app, target WSID
func projectorApplyDeactivateWorkspace(federation coreutils.IFederation, appQName istructs.AppQName, tokensAPI itokens.ITokens,
	asp istructs.IAppStructsProvider) func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
	return func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
		kb, err := s.KeyBuilder(state.Record, authnz.QNameCDocWorkspaceDescriptor)
		if err != nil {
			// notest
			return err
		}
		kb.PutQName(state.Field_Singleton, authnz.QNameCDocWorkspaceDescriptor)
		wsDesc, err := s.MustExist(kb)
		if err != nil {
			// notest
			return err
		}
		ownerApp := wsDesc.AsString(Field_OwnerApp)
		ownerWSID := wsDesc.AsInt64(Field_OwnerWSID)
		ownerID := wsDesc.AsInt64(Field_OwnerID)

		sysToken, err := payloads.GetSystemPrincipalToken(tokensAPI, appQName)
		if err != nil {
			// notest
			return err
		}

		// Foreach cdoc.sys.Subject
		as, err := asp.AppStructs(appQName)
		if err != nil {
			// notest
			return err
		}
		subjectsKB := as.ViewRecords().KeyBuilder(collection.QNameCollectionView)
		subjectsKB.PutInt32(collection.Field_PartKey, collection.PartitionKeyCollection)
		subjectsKB.PutQName(collection.Field_DocQName, invite.QNameCDocSubject)
		err = as.ViewRecords().Read(context.Background(), event.Workspace(), subjectsKB, func(_ istructs.IKey, value istructs.IValue) (err error) {
			subject := value.AsRecord(collection.Field_Record)
			if istructs.SubjectKindType(subject.AsInt32(authnz.Field_SubjectKind)) != istructs.SubjectKind_User {
				return nil
			}
			profileWSID := istructs.WSID(subject.AsInt64(invite.Field_ProfileWSID))
			// app is always current
			// impossible to have logins from different apps among subjects (Michael said)
			url := fmt.Sprintf(`api/%s/%d/c.sys.OnJoinedWorkspaceDeactivated`, appQName, profileWSID)

			body := fmt.Sprintf(`{"args":{"InvitedToWSID":%d}}`, event.Workspace())
			_, err = federation.Func(url, body, coreutils.WithAuthorizeBy(sysToken), coreutils.WithDiscardResponse())
			return err
		})
		if err != nil {
			// notestdebt
			return err
		}

		// currentApp/ApplicationWS/c.sys.OnWorkspaceDeactivated(OnwerWSID, WSName)
		wsName := wsDesc.AsString(authnz.Field_WSName)
		body := fmt.Sprintf(`{"args":{"OwnerWSID":%d, "WSName":"%s"}}`, ownerWSID, wsName)
		cdocWorkspaceIDWSID := coreutils.GetPseudoWSID(istructs.WSID(ownerWSID), wsName, event.Workspace().ClusterID())
		if _, err := federation.Func(fmt.Sprintf("api/%s/%d/c.sys.OnWorkspaceDeactivated", ownerApp, cdocWorkspaceIDWSID), body,
			coreutils.WithDiscardResponse(), coreutils.WithAuthorizeBy(sysToken)); err != nil {
			return fmt.Errorf("c.sys.OnWorkspaceDeactivated failed: %w", err)
		}

		// c.sys.OnChildWorkspaceDeactivated(ownerID))
		body = fmt.Sprintf(`{"args":{"OwnerID":%d}}`, ownerID)
		if _, err := federation.Func(fmt.Sprintf("api/%s/%d/c.sys.OnChildWorkspaceDeactivated", ownerApp, ownerWSID), body,
			coreutils.WithDiscardResponse(), coreutils.WithAuthorizeBy(sysToken)); err != nil {
			return fmt.Errorf("c.sys.OnChildWorkspaceDeactivated failed: %w", err)
		}

		// cdoc.sys.WorkspaceDescriptor.Status = Inactive
		body = fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"Status":%d}}]}`, wsDesc.AsRecordID(appdef.SystemField_ID), authnz.WorkspaceStatus_Inactive)
		if _, err := federation.Func(fmt.Sprintf("api/%s/%d/c.sys.CUD", appQName, event.Workspace()), body,
			coreutils.WithDiscardResponse(), coreutils.WithAuthorizeBy(sysToken)); err != nil {
			return fmt.Errorf("cdoc.sys.WorkspaceDescriptor.Status=Inactive failed: %w", err)
		}

		logger.Info("workspace", wsDesc.AsString(authnz.Field_WSName), "deactivated")
		return nil
	}
}
