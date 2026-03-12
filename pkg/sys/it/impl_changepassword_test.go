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
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/istructs"
	it "github.com/voedger/voedger/pkg/vit"
)

func TestBasicUsage_ChangePassword(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	loginName := vit.NextName()
	login := vit.SignUp(loginName, "1", istructs.AppQName_test1_app1)

	// change the password
	// null auth
	newPwd := "2"
	body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s"},"unloggedArgs":{"OldPassword":"1","NewPassword":"%s"}}`, loginName, istructs.AppQName_test1_app1, newPwd)
	vit.PostApp(istructs.AppQName_sys_registry, login.PseudoProfileWSID, "c.registry.ChangePassword", body)

	// note: previous tokens are still valid after password change

	// expect no errors on login with new password
	login.Pwd = newPwd
	vit.SignIn(login)
}

// [~server.users/it.TestQueryProcessor2_UsersChangePassword~impl]
func TestBasicUsage_ChangePassword_APIv2(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	// sign up a user with password "1"
	loginName := vit.NextName()
	login := vit.SignUp(loginName, "1", istructs.AppQName_test1_app1)

	// change password to "2"
	body := fmt.Sprintf(`{
		"login":"%s",
		"oldPassword": "1",
		"newPassword": "2"
	}`, login.Name)
	resp := vit.POST("api/v2/apps/test1/app1/users/change-password", body)
	require.Empty(t, resp.Body)

	// expect no errors on login with new password
	login.Pwd = "2"
	vit.SignIn(login)

	t.Run("passwords with special JSON characters", func(t *testing.T) {
		vit.TimeAdd(time.Minute) // reset rate-limit window before this password change
		specialPwd := `p"a\ss`
		specialLoginName := vit.NextName()
		specialLogin := vit.SignUp(specialLoginName, specialPwd, istructs.AppQName_test1_app1)
		vit.SignIn(specialLogin)

		bodyBytes, err := json.Marshal(map[string]any{
			"login":       specialLogin.Name,
			"oldPassword": specialPwd,
			"newPassword": specialPwd + "x",
		})
		require.NoError(t, err)
		resp := vit.POST("api/v2/apps/test1/app1/users/change-password", string(bodyBytes))
		require.Empty(t, resp.Body)

		specialLogin.Pwd = specialPwd + "x"
		vit.SignIn(specialLogin)
	})
}

func TestChangePasswordErrors_APIv2(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	t.Run("400 bad request", func(t *testing.T) {
		badRequests := []string{
			`{}`,
			`{"login":"abc"}`,
			`{"login":"abc","oldPassword": "1"}`,
			`{"login":"abc","newPassword": "2"}`,
			`{"login":1,"oldPassword": "1","newPassword": "2"}`,
			`{"login":"abc","oldPassword": 1,"newPassword": "2"}`,
			`{"login":"abc","oldPassword": "1","newPassword": 2}`,
		}
		for _, body := range badRequests {
			vit.POST("api/v2/apps/test1/app1/users/change-password", body, httpu.Expect400()).Println()
		}
	})

	t.Run("forward error from c.registry.ChangePassword, e.g. on an unknown login", func(t *testing.T) {
		unknownLogin := vit.NextName()
		body := fmt.Sprintf(`{"login":"%s","oldPassword": "1","newPassword": "2"}`, unknownLogin)
		vit.POST("api/v2/apps/test1/app1/users/change-password", body, httpu.Expect401()).Println()
	})
}

func TestChangePasswordErrors(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	prn := vit.GetPrincipal(istructs.AppQName_test1_app1, "login") // from VIT config

	t.Run("login not found", func(t *testing.T) {
		unexistingLogin := vit.NextName()
		body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s"},"unloggedArgs":{"OldPassword":"1","NewPassword":"2"}}`,
			unexistingLogin, istructs.AppQName_test1_app1)
		vit.PostApp(istructs.AppQName_sys_registry, prn.PseudoProfileWSID, "c.registry.ChangePassword", body, httpu.Expect401())
	})

	t.Run("wrong password", func(t *testing.T) {
		vit.TimeAdd(time.Minute) // proceed to the next minute to avoid 429 too many requests
		body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s"},"unloggedArgs":{"OldPassword":"2","NewPassword":"3"}}`,
			prn.Login.Name, istructs.AppQName_test1_app1)
		vit.PostApp(istructs.AppQName_sys_registry, prn.PseudoProfileWSID, "c.registry.ChangePassword", body, httpu.Expect401())
	})

	t.Run("rate limit exceed", func(t *testing.T) {
		vit.TimeAdd(time.Minute) // proceed to the next minute to avoid 429 too many requests

		body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s"},"unloggedArgs":{"OldPassword":"2","NewPassword":"3"}}`,
			prn.Login.Name, istructs.AppQName_test1_app1)
		vit.PostApp(istructs.AppQName_sys_registry, prn.PseudoProfileWSID, "c.registry.ChangePassword", body, httpu.Expect401()) // not 429, wrong password

		// >1 calls per minute -> 429
		vit.PostApp(istructs.AppQName_sys_registry, prn.PseudoProfileWSID, "c.registry.ChangePassword", body, httpu.Expect429())

		// proceed to the next minute -> able to change the password again
		vit.TimeAdd(time.Minute)
		body = fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s"},"unloggedArgs":{"OldPassword":"2","NewPassword":"3"}}`,
			prn.Login.Name, istructs.AppQName_test1_app1)
		vit.PostApp(istructs.AppQName_sys_registry, prn.PseudoProfileWSID, "c.registry.ChangePassword", body, httpu.Expect401()) // again not 429, wrong password
	})
}
