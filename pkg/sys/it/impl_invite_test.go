/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 */

package sys_it

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state/smtptest"
	"github.com/voedger/voedger/pkg/sys/invite"
	coreutils "github.com/voedger/voedger/pkg/utils"
	it "github.com/voedger/voedger/pkg/vit"
)

var (
	initialRoles        = "initial.Roles"
	newRoles            = "new.Roles"
	inviteEmailTemplate = "text:" + strings.Join([]string{
		invite.EmailTemplatePlaceholder_VerificationCode,
		invite.EmailTemplatePlaceholder_InviteID,
		invite.EmailTemplatePlaceholder_WSID,
		invite.EmailTemplatePlaceholder_WSName,
		invite.EmailTemplatePlaceholder_Email,
	}, ";")
	inviteEmailSubject = "you are invited"
)

// impossible to use the test workspace again with the same login due of invite error `subject already exists`
func TestInvite_BasicUsage(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	wsName := "TestInvite_BasicUsage_ws"
	wsParams := it.DummyWSParams(wsName)
	updateRolesEmailTemplate := "text:" + invite.EmailTemplatePlaceholder_Roles
	updateRolesEmailSubject := "your roles are updated"
	expireDatetime := vit.Now().UnixMilli()
	updatedRoles := "updated.Roles"

	email1 := fmt.Sprintf("testinvite_basicusage_%d@123.com", vit.NextNumber())
	email2 := fmt.Sprintf("testinvite_basicusage_%d@123.com", vit.NextNumber())
	email3 := fmt.Sprintf("testinvite_basicusage_%d@123.com", vit.NextNumber())
	login1 := vit.SignUp(email1, "1", istructs.AppQName_test1_app1)
	login2 := vit.SignUp(email2, "1", istructs.AppQName_test1_app1)
	login1Prn := vit.SignIn(login1)
	login2Prn := vit.SignIn(login2)
	prn := vit.GetPrincipal(istructs.AppQName_test1_app1, it.TestEmail)
	ws := vit.CreateWorkspace(wsParams, prn)

	initiateUpdateInviteRoles := func(inviteID int64) {
		vit.PostWS(ws, "c.sys.InitiateUpdateInviteRoles", fmt.Sprintf(`{"args":{"InviteID":%d,"Roles":"%s","EmailTemplate":"%s","EmailSubject":"%s"}}`, inviteID, updatedRoles, updateRolesEmailTemplate, updateRolesEmailSubject))
	}

	findCDocInviteByID := func(inviteID int64) []interface{} {
		return vit.PostWS(ws, "q.sys.Collection", fmt.Sprintf(`
			{"args":{"Schema":"sys.Invite"},
			"elements":[{"fields":[
				"SubjectKind",
				"Login",
				"Email",
				"Roles",
				"ExpireDatetime",
				"VerificationCode",
				"State",
				"Created",
				"Updated",
				"SubjectID",
				"InviteeProfileWSID",
				"ActualLogin",
				"sys.ID"
			]}],
			"filters":[{"expr":"eq","args":{"field":"sys.ID","value":%d}}]}`, inviteID)).SectionRow(0)
	}

	findCDocSubjectByLogin := func(login string) []interface{} {
		return vit.PostWS(ws, "q.sys.Collection", fmt.Sprintf(`
			{"args":{"Schema":"sys.Subject"},
			"elements":[{"fields":[
				"Login",
				"SubjectKind",
				"Roles",
				"sys.ID",
				"sys.IsActive"
			]}],
			"filters":[{"expr":"eq","args":{"field":"Login","value":"%s"}}]}`, login)).SectionRow(0)
	}

	//Invite existing users
	inviteID := InitiateInvitationByEMail(vit, ws, expireDatetime, email1, initialRoles, inviteEmailTemplate, inviteEmailSubject)
	inviteID2 := InitiateInvitationByEMail(vit, ws, expireDatetime, email2, initialRoles, inviteEmailTemplate, inviteEmailSubject)
	inviteID3 := InitiateInvitationByEMail(vit, ws, expireDatetime, email3, initialRoles, inviteEmailTemplate, inviteEmailSubject)

	// need to gather email first because
	actualEmails := []smtptest.Message{vit.CaptureEmail(), vit.CaptureEmail(), vit.CaptureEmail()}

	// State ToBeInvited exists for a very small period of time so let's do not catch it
	WaitForInviteState(vit, ws, invite.State_Invited, inviteID)
	WaitForInviteState(vit, ws, invite.State_Invited, inviteID2)
	WaitForInviteState(vit, ws, invite.State_Invited, inviteID3)

	cDocInvite := findCDocInviteByID(inviteID)

	require.Equal(email1, cDocInvite[1])
	require.Equal(email1, cDocInvite[2])
	require.Equal(initialRoles, cDocInvite[3])
	require.Equal(float64(expireDatetime), cDocInvite[4])
	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[7])
	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	//Check that emails were sent
	var verificationCodeEmail, verificationCodeEmail2, verificationCodeEmail3 string
	for _, actualEmail := range actualEmails {
		switch actualEmail.To[0] {
		case email1:
			verificationCodeEmail = actualEmail.Body[:6]
		case email2:
			verificationCodeEmail2 = actualEmail.Body[:6]
		case email3:
			verificationCodeEmail3 = actualEmail.Body[:6]
		}
	}
	expectedEmails := []smtptest.Message{
		{
			Subject: inviteEmailSubject,
			From:    it.TestSMTPCfg.GetFrom(),
			To:      []string{email1},
			Body:    fmt.Sprintf("%s;%d;%d;%s;%s", verificationCodeEmail, inviteID, ws.WSID, wsName, email1),
			CC:      []string{},
			BCC:     []string{},
		},
		{
			Subject: inviteEmailSubject,
			From:    it.TestSMTPCfg.GetFrom(),
			To:      []string{email2},
			Body:    fmt.Sprintf("%s;%d;%d;%s;%s", verificationCodeEmail2, inviteID2, ws.WSID, wsName, email2),
			CC:      []string{},
			BCC:     []string{},
		},
		{
			Subject: inviteEmailSubject,
			From:    it.TestSMTPCfg.GetFrom(),
			To:      []string{email3},
			Body:    fmt.Sprintf("%s;%d;%d;%s;%s", verificationCodeEmail3, inviteID3, ws.WSID, wsName, email3),
			CC:      []string{},
			BCC:     []string{},
		},
	}
	require.EqualValues(expectedEmails, actualEmails)

	cDocInvite = findCDocInviteByID(inviteID2)

	require.Equal(verificationCodeEmail2, cDocInvite[5])
	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	// overwrite roles is possible when the invite is not accepted yet
	verificationCodeEmail = testOverwriteRoles(t, vit, ws, email1, inviteID)

	//Cancel then invite it again (inviteID3)
	vit.PostWS(ws, "c.sys.CancelSentInvite", fmt.Sprintf(`{"args":{"InviteID":%d}}`, inviteID3))
	WaitForInviteState(vit, ws, invite.State_Cancelled, inviteID3)
	InitiateInvitationByEMail(vit, ws, expireDatetime, email3, initialRoles, inviteEmailTemplate, inviteEmailSubject)
	_ = vit.CaptureEmail()
	WaitForInviteState(vit, ws, invite.State_Invited, inviteID3)

	//Join workspaces
	InitiateJoinWorkspace(vit, ws, inviteID, login1Prn, verificationCodeEmail)
	InitiateJoinWorkspace(vit, ws, inviteID2, login2Prn, verificationCodeEmail2)

	// State_ToBeJoined will be set for a very short period of time so let's do not catch it
	WaitForInviteState(vit, ws, invite.State_Joined, inviteID)
	WaitForInviteState(vit, ws, invite.State_Joined, inviteID2)

	cDocInvite = findCDocInviteByID(inviteID2)

	require.Equal(float64(login2Prn.ProfileWSID), cDocInvite[10])
	require.Equal(float64(istructs.SubjectKind_User), cDocInvite[0])
	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	cDocJoinedWorkspace := FindCDocJoinedWorkspaceByInvitingWorkspaceWSIDAndLogin(vit, ws.WSID, login2Prn)

	require.Equal(initialRoles, cDocJoinedWorkspace.roles)
	require.Equal(wsName, cDocJoinedWorkspace.wsName)

	cDocSubject := findCDocSubjectByLogin(email1)

	require.Equal(email1, cDocSubject[0])
	require.Equal(float64(istructs.SubjectKind_User), cDocSubject[1])
	require.Equal(newRoles, cDocSubject[2]) // overwritten

	t.Run("reinivite the joined already -> error", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"Email":"%s","Roles":"%s","ExpireDatetime":%d,"EmailTemplate":"%s","EmailSubject":"%s"}}`,
			email1, initialRoles, 1674751138000, inviteEmailTemplate, inviteEmailSubject)
		vit.PostWS(ws, "c.sys.InitiateInvitationByEMail", body, coreutils.Expect400(invite.ErrSubjectAlreadyExists.Error()))
	})

	//Update roles
	initiateUpdateInviteRoles(inviteID)
	initiateUpdateInviteRoles(inviteID2)

	WaitForInviteState(vit, ws, invite.State_ToUpdateRoles, inviteID)
	WaitForInviteState(vit, ws, invite.State_ToUpdateRoles, inviteID2)
	cDocInvite = findCDocInviteByID(inviteID)

	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	//Check that emails were send
	require.Equal(updatedRoles, vit.CaptureEmail().Body)
	message := vit.CaptureEmail()
	require.Equal(updateRolesEmailSubject, message.Subject)
	require.Equal(it.TestSMTPCfg.GetFrom(), message.From)
	require.Equal([]string{email2}, message.To)
	require.Equal(updatedRoles, message.Body)

	cDocSubject = findCDocSubjectByLogin(email2)

	require.Equal(updatedRoles, cDocSubject[2])

	//TODO Denis how to get WS by login? I want to check sys.JoinedWorkspace

	WaitForInviteState(vit, ws, invite.State_Joined, inviteID)
	WaitForInviteState(vit, ws, invite.State_Joined, inviteID2)

	//Cancel accepted invite
	vit.PostWS(ws, "c.sys.InitiateCancelAcceptedInvite", fmt.Sprintf(`{"args":{"InviteID":%d}}`, inviteID))

	// State_ToBeCancelled will be set for a veri short period of time so let's do not catch it
	WaitForInviteState(vit, ws, invite.State_Cancelled, inviteID)

	cDocInvite = findCDocInviteByID(inviteID)

	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	cDocSubject = findCDocSubjectByLogin(email1)

	require.False(cDocSubject[4].(bool))

	cDocInvite = findCDocInviteByID(inviteID)

	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	//Leave workspace
	vit.PostWS(ws, "c.sys.InitiateLeaveWorkspace", "{}", coreutils.WithAuthorizeBy(login2Prn.Token))

	// State_ToBeLeft will be set for a veri short period of time so let's do not catch it
	WaitForInviteState(vit, ws, invite.State_Left, inviteID2)

	cDocInvite = findCDocInviteByID(inviteID2)

	require.Equal(float64(vit.Now().UnixMilli()), cDocInvite[8])

	cDocSubject = findCDocSubjectByLogin(email2)

	require.False(cDocSubject[4].(bool))

	//TODO check InviteeProfile joined workspace
}

func TestCancelSentInvite(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	email := fmt.Sprintf("testcancelsentinvite_%d@123.com", vit.NextNumber())
	login := vit.SignUp(email, "1", istructs.AppQName_test1_app1)
	loginPrn := vit.SignIn(login)
	wsParams := it.DummyWSParams("TestCancelSentInvite_ws")
	ws := vit.CreateWorkspace(wsParams, loginPrn)

	t.Run("basic usage", func(t *testing.T) {
		inviteID := InitiateInvitationByEMail(vit, ws, 1674751138000, email, initialRoles, inviteEmailTemplate, inviteEmailSubject)
		WaitForInviteState(vit, ws, invite.State_Invited, inviteID)

		//Read it for successful vit tear down
		vit.CaptureEmail()

		vit.PostWS(ws, "c.sys.CancelSentInvite", fmt.Sprintf(`{"args":{"InviteID":%d}}`, inviteID))
		WaitForInviteState(vit, ws, invite.State_Cancelled, inviteID)
	})
	t.Run("invite not exists -> 400 bad request", func(t *testing.T) {
		vit.PostWS(ws, "c.sys.CancelSentInvite", fmt.Sprintf(`{"args":{"InviteID":%d}}`, -100), coreutils.Expect400RefIntegrity_Existence())
	})
}

func testOverwriteRoles(t *testing.T, vit *it.VIT, ws *it.AppWorkspace, email string, inviteID int64) (verificationCode string) {
	require := require.New(t)

	// reinvite when invitation is not accepted yet -> roles must be overwritten
	newInviteID := InitiateInvitationByEMail(vit, ws, 1674751138000, email, newRoles, inviteEmailTemplate, inviteEmailSubject)
	require.Zero(newInviteID)
	WaitForInviteState(vit, ws, invite.State_Invited, inviteID)
	actualEmail := vit.CaptureEmail()
	verificationCode = actualEmail.Body[:6]

	// expect roles are overwritten in cdoc.sys.Invite
	body := fmt.Sprintf(`{"args":{"Schema":"sys.Invite","ID":%d},"elements":[{"fields":["Roles"]}]}`, inviteID)
	resp := vit.PostWS(ws, "q.sys.Collection", body)
	require.Equal(newRoles, resp.SectionRow()[0].(string))

	return verificationCode
}
