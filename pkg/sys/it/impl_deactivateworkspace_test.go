/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package sys_it

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/registry"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/invite"
	it "github.com/voedger/voedger/pkg/vit"
)

func TestBug2(t *testing.T) {
	require := require.New(t)
	vit := it.NewVITVVMOnly(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	appPart, err := vit.IAppPartitions.Borrow(istructs.AppQName_sys_registry, 1, appparts.ProcessorKind_Command)
	require.NoError(err)
	defer appPart.Release()

	t.Run("GRANT SELECT ON ALL QUERIES WITH TAG WithoutAuthTag TO sys.Anyone: OperationKind_Select does not work", func(t *testing.T) {
		_, _, err := appPart.IsOperationAllowed(
			appdef.OperationKind_Select,
			appdef.NewQName(registry.RegistryPackage, "IssuePrincipalToken"),
			nil,
			[]appdef.QName{iauthnz.QNameRoleEveryone},
		)
		require.Error(err)
	})

	t.Run("GRANT SELECT ON ALL QUERIES WITH TAG WithoutAuthTag TO sys.Anyone: OperationKind_Execute works", func(t *testing.T) {
		ok, _, err := appPart.IsOperationAllowed(
			appdef.OperationKind_Execute,
			appdef.NewQName(registry.RegistryPackage, "IssuePrincipalToken"),
			nil,
			[]appdef.QName{iauthnz.QNameRoleEveryone},
		)
		require.NoError(err)
		require.True(ok)
	})

	t.Run("GRANT INSERT ON ALL COMMANDS WITH TAG WithoutAuthTag TO sys.Anyone: OperationKind_Execute works", func(t *testing.T) {
		ok, _, err := appPart.IsOperationAllowed(
			appdef.OperationKind_Execute,
			appdef.NewQName(registry.RegistryPackage, "CreateLogin"),
			nil,
			[]appdef.QName{iauthnz.QNameRoleEveryone},
		)
		require.NoError(err)
		require.True(ok)
	})

	t.Run("GRANT INSERT ON ALL COMMANDS WITH TAG WithoutAuthTag TO sys.Anyone: OperationKind_Insert doe not work", func(t *testing.T) {
		_, _, err := appPart.IsOperationAllowed(
			appdef.OperationKind_Insert,
			appdef.NewQName(registry.RegistryPackage, "CreateLogin"),
			nil,
			[]appdef.QName{iauthnz.QNameRoleEveryone},
		)
		require.Error(err)
	})
}

func TestBasicUsage_InitiateDeactivateWorkspace(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	wsName := vit.NextName()

	prn1 := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail)
	wsp := it.SimpleWSParams(wsName)

	ws := vit.CreateWorkspace(wsp, prn1)

	// initiate deactivate workspace
	vit.PostWS(ws, "c.sys.InitiateDeactivateWorkspace", "{}")
	waitForDeactivate(vit, ws)

	// 410 Gone on work in an inactive workspace
	bodyCmd := `{"cuds":[{"fields":{"sys.QName":"app1pkg.computers","sys.ID":1}}]}`
	vit.PostWS(ws, "c.sys.CUD", bodyCmd, coreutils.Expect410()).Println()
	bodyQry := `{"args":{"Schema":"sys.WorkspaceDescriptor"},"elements":[{"fields":["Status"]}]}`
	vit.PostWS(ws, "q.sys.Collection", bodyQry, coreutils.Expect410()).Println()

	// still able to work in an inactive workspace with the system token
	sysToken := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
	vit.PostWS(ws, "q.sys.Collection", bodyQry, coreutils.WithAuthorizeBy(sysToken.Token))
	vit.PostWS(ws, "c.sys.CUD", bodyCmd, coreutils.WithAuthorizeBy(sysToken.Token))

	// 409 conflict on deactivate an already deactivated worksace
	vit.PostWS(ws, "c.sys.InitiateDeactivateWorkspace", "{}", coreutils.WithAuthorizeBy(sysToken.Token), coreutils.Expect409())
}

func waitForDeactivate(vit *it.VIT, ws *it.AppWorkspace) {
	deadline := it.TestDeadline()
	for time.Now().Before(deadline) {
		resp := vit.PostWSSys(ws, "q.sys.Collection", `{"args":{"Schema":"sys.WorkspaceDescriptor"},"elements":[{"fields":["Status"]}]}`)
		if int32(resp.SectionRow()[0].(float64)) == int32(authnz.WorkspaceStatus_Inactive) {
			return
		}
		time.Sleep(awaitTime)
	}
	vit.T.Fatal("workspace", ws.Name, "is not deactivated in an acceptable time")
}

func TestDeactivateJoinedWorkspace(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	wsName1 := vit.NextName()
	prn1 := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail)
	prn2 := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail2)
	wsp := it.SimpleWSParams(wsName1)

	newWS := vit.CreateWorkspace(wsp, prn1)

	// check prn2 could not work in ws1
	body := `{"cuds":[{"fields":{"sys.QName":"app1pkg.computers","sys.ID":1}}]}`
	vit.PostWS(newWS, "c.sys.CUD", body, coreutils.WithAuthorizeBy(prn2.Token), coreutils.Expect403())

	// join login TestEmail2 to ws1
	expireDatetime := vit.Now().UnixMilli()
	roleOwner := iauthnz.QNameRoleWorkspaceOwner.String()
	updateRolesEmailSubject := "your roles are updated"
	inviteID := InitiateInvitationByEMail(vit, newWS, expireDatetime, it.TestEmail2, roleOwner, inviteEmailTemplate, updateRolesEmailSubject)
	email := vit.CaptureEmail()
	verificationCode := email.Body[:6]
	WaitForInviteState(vit, newWS, inviteID, invite.State_ToBeJoined, invite.State_Invited)
	testEmail2Prn := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail2)
	InitiateJoinWorkspace(vit, newWS, inviteID, testEmail2Prn, verificationCode)
	WaitForInviteState(vit, newWS, inviteID, invite.State_ToBeJoined, invite.State_Joined)

	// check prn2 could work in ws1
	body = `{"cuds":[{"fields":{"sys.QName":"app1pkg.computers","sys.ID":1}}]}`
	vit.PostWS(newWS, "c.sys.CUD", body, coreutils.WithAuthorizeBy(prn2.Token))

	// deactivate
	vit.PostWS(newWS, "c.sys.InitiateDeactivateWorkspace", "{}")
	waitForDeactivate(vit, newWS)

	// check cdoc.sys.JoinedWorkspace.IsActive == false
	joinedWorkspace := FindCDocJoinedWorkspaceByInvitingWorkspaceWSIDAndLogin(vit, newWS.WSID, testEmail2Prn)
	require.False(joinedWorkspace.isActive)

	// check appWS/cdoc.sys.WorkspaceID.IsActive == false
	wsidOfCDocWorkspaceID := coreutils.GetPseudoWSID(prn1.ProfileWSID, newWS.Name, istructs.CurrentClusterID())
	body = fmt.Sprintf(`{"args":{"Query":"select IDOfCDocWorkspaceID from sys.WorkspaceIDIdx where OwnerWSID = %d and WSName = '%s'"}, "elements":[{"fields":["Result"]}]}`,
		prn1.ProfileWSID, newWS.Name)
	sysToken := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
	resp := vit.PostApp(istructs.AppQName_test1_app1, wsidOfCDocWorkspaceID, "q.sys.SqlQuery", body, coreutils.WithAuthorizeBy(sysToken.Token))
	viewWorkspaceIDIdx := map[string]interface{}{}
	require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &viewWorkspaceIDIdx))
	idOfCDocWorkspaceID := int64(viewWorkspaceIDIdx["IDOfCDocWorkspaceID"].(float64))
	body = fmt.Sprintf(`{"args":{"ID": %d},"elements":[{"fields": ["Result"]}]}`, idOfCDocWorkspaceID)
	resp = vit.PostApp(istructs.AppQName_test1_app1, wsidOfCDocWorkspaceID, "q.sys.GetCDoc", body, coreutils.WithAuthorizeBy(sysToken.Token))
	jsonBytes := []byte(resp.SectionRow()[0].(string))
	cdocWorkspaceID := map[string]interface{}{}
	require.NoError(json.Unmarshal(jsonBytes, &cdocWorkspaceID))
	require.False(cdocWorkspaceID[appdef.SystemField_IsActive].(bool))
}
