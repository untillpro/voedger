/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 */

package invite

import (
	"fmt"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/state"
	sysshared "github.com/voedger/voedger/pkg/sys/shared"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func ProvideAsyncProjectorApplyLeaveWorkspaceFactory(timeFunc func() time.Time, federation coreutils.IFederation, appQName istructs.AppQName, tokens itokens.ITokens) istructs.ProjectorFactory {
	return func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{
			Name:         qNameAPApplyLeaveWorkspace,
			EventsFilter: []appdef.QName{qNameCmdInitiateLeaveWorkspace},
			Func:         applyLeaveWorkspace(timeFunc, federation, appQName, tokens),
		}
	}
}

func applyLeaveWorkspace(timeFunc func() time.Time, federation coreutils.IFederation, appQName istructs.AppQName, tokens itokens.ITokens) func(event istructs.IPLogEvent, state istructs.IState, intents istructs.IIntents) (err error) {
	return func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
		return event.CUDs(func(rec istructs.ICUDRow) (err error) {
			//TODO additional check that CUD only once?
			if rec.QName() != qNameCDocInvite {
				return
			}

			skbCDocInvite, err := s.KeyBuilder(state.RecordsStorage, qNameCDocInvite)
			if err != nil {
				return
			}
			skbCDocInvite.PutRecordID(state.Field_ID, rec.ID())
			svCDocInvite, err := s.MustExist(skbCDocInvite)
			if err != nil {
				return
			}

			skbCDocSubject, err := s.KeyBuilder(state.RecordsStorage, sysshared.QNameCDocSubject)
			if err != nil {
				return
			}
			skbCDocSubject.PutRecordID(state.Field_ID, svCDocInvite.AsRecordID(field_SubjectID))
			svCDocSubject, err := s.MustExist(skbCDocSubject)
			if err != nil {
				return
			}

			token, err := payloads.GetSystemPrincipalToken(tokens, appQName)
			if err != nil {
				return
			}

			//Update subject
			_, err = coreutils.FederationFunc(
				federation.URL(),
				fmt.Sprintf("api/%s/%d/c.sys.CUD", appQName, event.Workspace()),
				fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"sys.IsActive":false}}]}`, svCDocSubject.AsRecordID(appdef.SystemField_ID)),
				coreutils.WithAuthorizeBy(token))
			if err != nil {
				return
			}

			//Deactivate joined workspace
			_, err = coreutils.FederationFunc(
				federation.URL(),
				fmt.Sprintf("api/%s/%d/c.sys.DeactivateJoinedWorkspace", appQName, svCDocInvite.AsInt64(field_InviteeProfileWSID)),
				fmt.Sprintf(`{"args":{"InvitingWorkspaceWSID":%d}}`, event.Workspace()),
				coreutils.WithAuthorizeBy(token))
			if err != nil {
				return
			}

			//Update invite
			_, err = coreutils.FederationFunc(
				federation.URL(),
				fmt.Sprintf("api/%s/%d/c.sys.CUD", appQName, event.Workspace()),
				fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"State":%d,"Updated":%d}}]}`, rec.ID(), State_Left, timeFunc().UnixMilli()),
				coreutils.WithAuthorizeBy(token))

			return
		})
	}
}
