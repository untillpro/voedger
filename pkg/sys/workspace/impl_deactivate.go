/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package workspace

import (
	"fmt"
	"net/http"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/collection"
	"github.com/voedger/voedger/pkg/sys/invite"
)

func provideDeactivateWorkspace(sr istructsmem.IStatelessResources, tokensAPI itokens.ITokens, federation federation.IFederation) {

	sr.AddCommands(appdef.SysPackagePath,
		// c.sys.DeactivateWorkspace
		// target app, target WSID
		istructsmem.NewCommandFunction(
			qNameCmdInitiateDeactivateWorkspace,
			cmdInitiateDeactivateWorkspaceExec,
		),

		// c.sys.OnWorkspaceDeactivated
		// owner app, owner WSID
		istructsmem.NewCommandFunction(
			appdef.NewQName(appdef.SysPackage, "OnWorkspaceDeactivated"),
			cmdOnWorkspaceDeactivatedExec,
		),

		// c.sys.OnJoinedWorkspaceDeactivated
		// target app, profile WSID
		istructsmem.NewCommandFunction(
			appdef.NewQName(appdef.SysPackage, "OnJoinedWorkspaceDeactivated"),
			cmdOnJoinedWorkspaceDeactivateExec,
		),

		// c.sys.OnChildWorkspaceDeactivated
		// ownerApp/ownerWSID
		istructsmem.NewCommandFunction(
			appdef.NewQName(appdef.SysPackage, "OnChildWorkspaceDeactivated"),
			cmdOnChildWorkspaceDeactivatedExec,
		),
	)

	// target app, target WSID
	sr.AddProjectors(appdef.SysPackagePath, istructs.Projector{
		Name: qNameProjectorApplyDeactivateWorkspace,
		Func: projectorApplyDeactivateWorkspace(federation, tokensAPI),
	})
}

func cmdInitiateDeactivateWorkspaceExec(args istructs.ExecCommandArgs) (err error) {
	kb, err := args.State.KeyBuilder(sys.Storage_Record, appdef.QNameCDocWorkspaceDescriptor)
	if err != nil {
		// notest
		return err
	}
	kb.PutQName(sys.Storage_Record_Field_Singleton, appdef.QNameCDocWorkspaceDescriptor)
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
	kb, err := args.State.KeyBuilder(sys.Storage_View, QNameViewWorkspaceIDIdx)
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
	kb, err = args.State.KeyBuilder(sys.Storage_Record, QNameCDocWorkspaceID)
	if err != nil {
		// notest
		return err
	}
	kb.PutRecordID(sys.Storage_Record_Field_ID, idOfCDocWorkspaceID)
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
	kb, err := args.State.KeyBuilder(sys.Storage_Record, appdef.NullQName)
	if err != nil {
		// notest
		return err
	}
	kb.PutRecordID(sys.Storage_Record_Field_ID, istructs.RecordID(ownerID)) // nolint G115
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
func projectorApplyDeactivateWorkspace(federation federation.IFederation, tokensAPI itokens.ITokens) func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
	return func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
		kb, err := s.KeyBuilder(sys.Storage_Record, appdef.QNameCDocWorkspaceDescriptor)
		if err != nil {
			// notest
			return err
		}
		kb.PutQName(sys.Storage_Record_Field_Singleton, appdef.QNameCDocWorkspaceDescriptor)
		wsDesc, err := s.MustExist(kb)
		if err != nil {
			// notest
			return err
		}
		ownerApp := wsDesc.AsString(Field_OwnerApp)
		ownerWSID := wsDesc.AsInt64(Field_OwnerWSID)
		ownerID := wsDesc.AsInt64(Field_OwnerID)

		projectorAppQName := s.App()
		ownerAppQName := appdef.MustParseAppQName(ownerApp) // WorkspaceDescriptor.OwnerApp is checked already

		projectorAppToken, err := payloads.GetSystemPrincipalToken(tokensAPI, projectorAppQName)
		if err != nil {
			// notest
			return err
		}

		ownerAppToken := projectorAppToken
		if ownerAppQName != projectorAppQName {
			if ownerAppToken, err = payloads.GetSystemPrincipalToken(tokensAPI, ownerAppQName); err != nil {
				// notest
				return fmt.Errorf("failed to issue system token for ownerApp %s: %w", ownerAppQName, err)
			}
		}

		// Foreach cdoc.sys.Subject
		subjectsKB, err := s.KeyBuilder(sys.Storage_View, collection.QNameCollectionView)
		if err != nil {
			// notest
			return err
		}
		subjectsKB.PutInt32(collection.Field_PartKey, collection.PartitionKeyCollection)
		subjectsKB.PutQName(collection.Field_DocQName, invite.QNameCDocSubject)
		err = s.Read(subjectsKB, func(_ istructs.IKey, value istructs.IStateValue) (err error) {
			subject := value.(istructs.IStateViewValue).AsRecord(collection.Field_Record)
			if istructs.SubjectKindType(subject.AsInt32(authnz.Field_SubjectKind)) != istructs.SubjectKind_User {
				return nil
			}
			profileWSID := istructs.WSID(subject.AsInt64(invite.Field_ProfileWSID)) // nolint G115

			// app is always current
			// impossible to have logins from different apps among subjects (Michael said)
			url := fmt.Sprintf(`api/%s/%d/c.sys.OnJoinedWorkspaceDeactivated`, projectorAppQName, profileWSID)
			body := fmt.Sprintf(`{"args":{"InvitedToWSID":%d}}`, event.Workspace())
			_, err = federation.Func(url, body, httpu.WithAuthorizeBy(projectorAppToken), httpu.WithDiscardResponse())
			return err
		})
		if err != nil {
			// notestdebt
			return err
		}

		// currentApp/ApplicationWS/c.sys.OnWorkspaceDeactivated(OnwerWSID, WSName)
		wsName := wsDesc.AsString(authnz.Field_WSName)
		body := fmt.Sprintf(`{"args":{"OwnerWSID":%d, "WSName":%q}}`, ownerWSID, wsName)
		cdocWorkspaceIDWSID := coreutils.GetPseudoWSID(istructs.WSID(ownerWSID), wsName, event.Workspace().ClusterID()) // nolint G115
		if _, err := federation.Func(fmt.Sprintf("api/%s/%d/c.sys.OnWorkspaceDeactivated", ownerApp, cdocWorkspaceIDWSID), body,
			httpu.WithDiscardResponse(), httpu.WithAuthorizeBy(ownerAppToken)); err != nil {
			return fmt.Errorf("c.sys.OnWorkspaceDeactivated failed: %w", err)
		}

		// c.sys.OnChildWorkspaceDeactivated(ownerID))
		body = fmt.Sprintf(`{"args":{"OwnerID":%d}}`, ownerID)
		if _, err := federation.Func(fmt.Sprintf("api/%s/%d/c.sys.OnChildWorkspaceDeactivated", ownerApp, ownerWSID), body,
			httpu.WithDiscardResponse(), httpu.WithAuthorizeBy(ownerAppToken)); err != nil {
			return fmt.Errorf("c.sys.OnChildWorkspaceDeactivated failed: %w", err)
		}

		// cdoc.sys.WorkspaceDescriptor.Status = Inactive
		body = fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"Status":%d}}]}`, wsDesc.AsRecordID(appdef.SystemField_ID), authnz.WorkspaceStatus_Inactive)
		if _, err := federation.Func(fmt.Sprintf("api/%s/%d/c.sys.CUD", projectorAppQName, event.Workspace()), body,
			httpu.WithDiscardResponse(), httpu.WithAuthorizeBy(projectorAppToken)); err != nil {
			return fmt.Errorf("cdoc.sys.WorkspaceDescriptor.Status=Inactive failed: %w", err)
		}

		logger.Info("workspace", wsDesc.AsString(authnz.Field_WSName), "deactivated")
		return nil
	}
}
