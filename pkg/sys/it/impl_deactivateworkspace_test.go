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
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/invite"
	it "github.com/voedger/voedger/pkg/vit"
)

func TestBasicUsage_InitiateDeactivateWorkspace(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	wsName := vit.NextName()

	prn1 := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail)
	wsp := it.SimpleWSParams(wsName)

	ws := vit.CreateWorkspace(wsp, prn1)

	// initiate deactivate workspace
	vit.PostWS(ws, "c.sys.InitiateDeactivateWorkspace", "{}")
	waitForDeactivate(vit, ws.Owner.AppQName, ws.WSID, ws.Name)

	// 410 Gone on work in an inactive workspace
	bodyCmd := `{"cuds":[{"fields":{"sys.QName":"app1pkg.computers","sys.ID":1}}]}`
	vit.PostWS(ws, "c.sys.CUD", bodyCmd, httpu.Expect410()).Println()
	bodyQry := `{"args":{"Schema":"sys.WorkspaceDescriptor"},"elements":[{"fields":["Status"]}]}`
	vit.PostWS(ws, "q.sys.Collection", bodyQry, httpu.Expect410()).Println()

	// still able to work in an inactive workspace with the system token
	sysToken := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
	vit.PostWS(ws, "q.sys.Collection", bodyQry, httpu.WithAuthorizeBy(sysToken.Token))
	vit.PostWS(ws, "c.sys.CUD", bodyCmd, httpu.WithAuthorizeBy(sysToken.Token))

	// 409 conflict on deactivate an already deactivated worksace
	vit.PostWS(ws, "c.sys.InitiateDeactivateWorkspace", "{}", httpu.WithAuthorizeBy(sysToken.Token), httpu.Expect409())
}

func waitForDeactivate(vit *it.VIT, appQName appdef.AppQName, wsid istructs.WSID, name string) {
	sysToken := vit.GetSystemPrincipal(appQName).Token
	deadline := it.TestDeadline()
	for time.Now().Before(deadline) {
		resp := vit.PostApp(appQName, wsid, "q.sys.Collection",
			`{"args":{"Schema":"sys.WorkspaceDescriptor"},"elements":[{"fields":["Status"]}]}`,
			httpu.WithAuthorizeBy(sysToken))
		if int32(resp.SectionRow()[0].(float64)) == int32(authnz.WorkspaceStatus_Inactive) {
			return
		}
		time.Sleep(awaitTime)
	}
	vit.T.Fatal("workspace", name, "is not deactivated in an acceptable time")
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
	vit.PostWS(newWS, "c.sys.CUD", body, httpu.WithAuthorizeBy(prn2.Token), httpu.Expect403())

	// join login TestEmail2 to ws1
	expireDatetime := vit.Now().UnixMilli()
	roleOwner := "app1pkg.InviteTestRole"
	updateRolesEmailSubject := "your roles are updated"
	inviteID := InitiateInvitationByEMail(vit, newWS, expireDatetime, it.TestEmail2, roleOwner, inviteEmailTemplate, updateRolesEmailSubject)
	email := vit.CaptureEmail()
	verificationCode := email.Body[:6]
	WaitForInviteState(vit, newWS, inviteID, invite.State_ToBeInvited, invite.State_Invited)
	testEmail2Prn := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail2)
	InitiateJoinWorkspace(vit, newWS, inviteID, testEmail2Prn, verificationCode)
	WaitForInviteState(vit, newWS, inviteID, invite.State_Invited, invite.State_Joined)

	// check prn2 could work in ws1
	body = `{"cuds":[{"fields":{"sys.QName":"app1pkg.computers","sys.ID":1}}]}`
	vit.PostWS(newWS, "c.sys.CUD", body, httpu.WithAuthorizeBy(prn2.Token))

	// deactivate
	vit.PostWS(newWS, "c.sys.InitiateDeactivateWorkspace", "{}")
	waitForDeactivate(vit, newWS.Owner.AppQName, newWS.WSID, newWS.Name)

	// check cdoc.sys.JoinedWorkspace.IsActive == false
	joinedWorkspace := FindCDocJoinedWorkspaceByInvitingWorkspaceWSIDAndLogin(vit, newWS.WSID, testEmail2Prn)
	require.False(joinedWorkspace.isActive)

	// check appWS/cdoc.sys.WorkspaceID.IsActive == false
	wsidOfCDocWorkspaceID := coreutils.GetPseudoWSID(prn1.ProfileWSID, newWS.Name, istructs.CurrentClusterID())
	body = fmt.Sprintf(`{"args":{"Query":"select IDOfCDocWorkspaceID from sys.WorkspaceIDIdx where OwnerWSID = %d and WSName = '%s'"}, "elements":[{"fields":["Result"]}]}`,
		prn1.ProfileWSID, newWS.Name)
	sysToken := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
	resp := vit.PostApp(istructs.AppQName_test1_app1, wsidOfCDocWorkspaceID, "q.sys.SqlQuery", body, httpu.WithAuthorizeBy(sysToken.Token))
	viewWorkspaceIDIdx := map[string]interface{}{}
	require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &viewWorkspaceIDIdx))
	idOfCDocWorkspaceID := int64(viewWorkspaceIDIdx["IDOfCDocWorkspaceID"].(float64))
	body = fmt.Sprintf(`{"args":{"ID": %d},"elements":[{"fields": ["Result"]}]}`, idOfCDocWorkspaceID)
	resp = vit.PostApp(istructs.AppQName_test1_app1, wsidOfCDocWorkspaceID, "q.sys.GetCDoc", body, httpu.WithAuthorizeBy(sysToken.Token))
	jsonBytes := []byte(resp.SectionRow()[0].(string))
	cdocWorkspaceID := map[string]interface{}{}
	require.NoError(json.Unmarshal(jsonBytes, &cdocWorkspaceID))
	require.False(cdocWorkspaceID[appdef.SystemField_IsActive].(bool))
}

func TestDeactivateUserProfile(t *testing.T) {
	logCap := logger.StartCapture(t, logger.LogLevelVerbose)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	loginName := vit.NextName() + "@123.com"
	pwd := "1"
	login := vit.SignUp(loginName, pwd, istructs.AppQName_test1_app1)
	prn := vit.SignIn(login)

	cdocLoginID := vit.GetCDocLoginID(login)

	// obtain a valid VerifiedValueToken before deactivation: c.registry.ResetPasswordByEmail
	// requires a verified Email field, so the only way to reach the GetCDocLogin lookup is
	// to hand the command a token issued for the still-active login
	profileWSID := istructs.WSID(0)
	verifyToken, verifyCode := InitiateEmailVerificationFunc(vit, func() *federation.FuncResponse {
		body := fmt.Sprintf(`{"args":{"AppName":"%s","Email":"%s"},"elements":[{"fields":["VerificationToken","ProfileWSID"]}]}`, istructs.AppQName_test1_app1, loginName)
		resp := vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "q.registry.InitiateResetPasswordByEmail", body)
		profileWSID = istructs.WSID(resp.SectionRow()[1].(float64))
		return resp
	})
	body := fmt.Sprintf(`{"args":{"VerificationToken":"%s","VerificationCode":"%s","ProfileWSID":%d,"AppName":"%s"},"elements":[{"fields":["VerifiedValueToken"]}]}`,
		verifyToken, verifyCode, profileWSID, istructs.AppQName_test1_app1)
	verifiedValueToken := vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID,
		"q.registry.IssueVerifiedValueTokenForResetPassword", body).SectionRow()[0].(string)

	// deactivate the user profile workspace -> cascade -> cdoc.registry.Login.sys.IsActive = false
	vit.PostProfile(prn, "c.sys.InitiateDeactivateWorkspace", "{}")
	waitForDeactivate(vit, prn.AppQName, prn.ProfileWSID, loginName)

	t.Run("410 Gone on work in deactivated profile", func(t *testing.T) {
		body := `{"args":{"Schema":"sys.UserProfile"},"elements":[{"fields":["sys.ID"]}]}`
		vit.PostProfile(prn, "q.sys.Collection", body, httpu.Expect410()).Println()
	})

	expectedCDocLoginIDStr := fmt.Sprintf("%d", cdocLoginID)
	expectVerboseLine := func() {
		logCap.EventuallyHasLine("cdoc.registry.Login", "is deactivated, treating as missing login", expectedCDocLoginIDStr)
	}

	t.Run("q.registry.IssuePrincipalToken -> 401", func(t *testing.T) {
		logCap.Reset()
		body := fmt.Sprintf(`{"args":{"Login":"%s","Password":"%s","AppName":"%s"},"elements":[{"fields":["PrincipalToken"]}]}`,
			loginName, pwd, istructs.AppQName_test1_app1)
		vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "q.registry.IssuePrincipalToken", body,
			it.Expect401("login or password is incorrect")).Println()
		expectVerboseLine()
	})

	t.Run("c.registry.ChangePassword -> 401", func(t *testing.T) {
		logCap.Reset()
		body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s"},"unloggedArgs":{"OldPassword":"%s","NewPassword":"new"}}`,
			loginName, istructs.AppQName_test1_app1, pwd)
		vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "c.registry.ChangePassword", body,
			it.Expect401(fmt.Sprintf("login %s does not exist", loginName))).Println()
		expectVerboseLine()
	})

	t.Run("q.registry.InitiateResetPasswordByEmail -> 400", func(t *testing.T) {
		logCap.Reset()
		body := fmt.Sprintf(`{"args":{"AppName":"%s","Email":"%s"},"elements":[{"fields":["VerificationToken","ProfileWSID"]}]}`,
			istructs.AppQName_test1_app1, loginName)
		vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "q.registry.InitiateResetPasswordByEmail", body,
			it.Expect400("login does not exist")).Println()
		expectVerboseLine()
	})

	t.Run("c.registry.ResetPasswordByEmail -> 401", func(t *testing.T) {
		logCap.Reset()
		body := fmt.Sprintf(`{"args":{"AppName":"%s"},"unloggedArgs":{"Email":"%s","NewPwd":"new"}}`,
			istructs.AppQName_test1_app1, verifiedValueToken)
		vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "c.registry.ResetPasswordByEmail", body,
			it.Expect401(fmt.Sprintf("login %s does not exist", loginName))).Println()
		expectVerboseLine()
	})

	t.Run("c.registry.UpdateGlobalRoles -> 401", func(t *testing.T) {
		logCap.Reset()
		sysRegistryToken := vit.GetSystemPrincipal(istructs.AppQName_sys_registry).Token
		body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s","GlobalRoles":""}}`,
			loginName, istructs.AppQName_test1_app1)
		vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "c.registry.UpdateGlobalRoles", body,
			httpu.WithAuthorizeBy(sysRegistryToken),
			it.Expect401(fmt.Sprintf("login %s does not exist", loginName))).Println()
		expectVerboseLine()
	})
}
