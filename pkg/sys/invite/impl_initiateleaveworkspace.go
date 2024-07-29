/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 */

package invite

import (
	"net/http"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func provideCmdInitiateLeaveWorkspace(sr istructsmem.IStatelessResources, timeFunc coreutils.TimeFunc) {
	sr.AddCommands(appdef.SysPackagePath, istructsmem.NewCommandFunction(
		qNameCmdInitiateLeaveWorkspace,
		execCmdInitiateLeaveWorkspace(timeFunc),
	))
}

func execCmdInitiateLeaveWorkspace(timeFunc coreutils.TimeFunc) func(args istructs.ExecCommandArgs) (err error) {
	return func(args istructs.ExecCommandArgs) (err error) {
		skbPrincipal, err := args.State.KeyBuilder(state.RequestSubject, appdef.NullQName)
		if err != nil {
			return
		}
		svPrincipal, err := args.State.MustExist(skbPrincipal)
		if err != nil {
			return
		}

		skbViewInviteIndex, err := args.State.KeyBuilder(state.View, qNameViewInviteIndex)
		if err != nil {
			return
		}
		skbViewInviteIndex.PutInt32(field_Dummy, value_Dummy_One)
		skbViewInviteIndex.PutString(Field_Login, svPrincipal.AsString(state.Field_Name))
		svViewInviteIndex, err := args.State.MustExist(skbViewInviteIndex)
		if err != nil {
			return
		}

		skbCDocInvite, err := args.State.KeyBuilder(state.Record, qNameCDocInvite)
		if err != nil {
			return err
		}
		skbCDocInvite.PutRecordID(state.Field_ID, svViewInviteIndex.AsRecordID(field_InviteID))
		svCDocInvite, err := args.State.MustExist(skbCDocInvite)
		if err != nil {
			return err
		}

		if !isValidInviteState(svCDocInvite.AsInt32(field_State), qNameCmdInitiateLeaveWorkspace) {
			return coreutils.NewHTTPError(http.StatusBadRequest, ErrInviteStateInvalid)
		}

		svbCDocInvite, err := args.Intents.UpdateValue(skbCDocInvite, svCDocInvite)
		if err != nil {
			return err
		}
		svbCDocInvite.PutInt32(field_State, State_ToBeLeft)
		svbCDocInvite.PutInt64(field_Updated, timeFunc().UnixMilli())
		svbCDocInvite.PutBool(appdef.SystemField_IsActive, false)

		return
	}
}
