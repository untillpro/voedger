/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sys_it

import (
	"encoding/base64"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/authnz"
	coreutils "github.com/voedger/voedger/pkg/utils"
	it "github.com/voedger/voedger/pkg/vit"
	"github.com/voedger/voedger/pkg/vvm"
)

func TestAppWSAutoInitialization(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	checkCDocsWSDesc(vit.VVMConfig, vit.VVM, require)

	// further calls -> nothing happens, expect no errors
	require.NoError(vvm.BuildAppWorkspaces(vit.VVM, vit.VVMConfig))
	checkCDocsWSDesc(vit.VVMConfig, vit.VVM, require)
}

func checkCDocsWSDesc(vvmCfg *vvm.VVMConfig, vvm *vvm.VVM, require *require.Assertions) {
	for appQName := range vvmCfg.VVMAppsBuilder {
		as, err := vvm.AppStructs(appQName)
		require.NoError(err)
		for wsNum := 0; istructs.NumAppWorkspaces(wsNum) < as.NumAppWorkspaces(); wsNum++ {
			appWSID := istructs.NewWSID(istructs.MainClusterID, istructs.WSID(wsNum+int(istructs.FirstBaseAppWSID)))
			existingCDocWSDesc, err := as.Records().GetSingleton(appWSID, authnz.QNameCDocWorkspaceDescriptor)
			require.NoError(err)
			require.Equal(authnz.QNameCDocWorkspaceDescriptor, existingCDocWSDesc.QName())
		}
	}
}

func TestAuthorization(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	prn := ws.Owner

	body := `{"cuds":[{"fields":{"sys.ID": 1,"sys.QName":"app1pkg.air_table_plan"}}]}`

	t.Run("basic usage", func(t *testing.T) {
		t.Run("Bearer scheme", func(t *testing.T) {
			sys := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
			vit.PostWS(ws, "c.sys.CUD", body, coreutils.WithHeaders(coreutils.Authorization, "Bearer "+sys.Token))
		})

		t.Run("Basic scheme", func(t *testing.T) {
			t.Run("token in username only", func(t *testing.T) {
				basicAuthHeader := base64.StdEncoding.EncodeToString([]byte(prn.Token + ":"))
				vit.PostWS(ws, "c.sys.CUD", body, coreutils.WithHeaders(coreutils.Authorization, "Basic "+basicAuthHeader))
			})
			t.Run("token in password only", func(t *testing.T) {
				basicAuthHeader := base64.StdEncoding.EncodeToString([]byte(":" + prn.Token))
				vit.PostWS(ws, "c.sys.CUD", body, coreutils.WithHeaders(coreutils.Authorization, "Basic "+basicAuthHeader))
			})
			t.Run("token is splitted over username and password", func(t *testing.T) {
				basicAuthHeader := base64.StdEncoding.EncodeToString([]byte(prn.Token[:len(prn.Token)/2] + ":" + prn.Token[len(prn.Token)/2:]))
				vit.PostWS(ws, "c.sys.CUD", body, coreutils.WithHeaders(coreutils.Authorization, "Basic "+basicAuthHeader))
			})
		})
	})

	t.Run("401 unauthorized", func(t *testing.T) {
		t.Run("wrong Authorization token", func(t *testing.T) {
			vit.PostProfile(prn, "c.sys.CUD", body, coreutils.WithAuthorizeBy("wrong"), coreutils.Expect401()).Println()
		})

		t.Run("unsupported Authorization header", func(t *testing.T) {
			vit.PostProfile(prn, "c.sys.CUD", body,
				coreutils.WithHeaders(coreutils.Authorization, `whatever w\o Bearer or Basic`), coreutils.Expect401()).Println()
		})

		t.Run("Basic authorization", func(t *testing.T) {
			t.Run("non-base64 header value", func(t *testing.T) {
				vit.PostProfile(prn, "c.sys.CUD", body,
					coreutils.WithHeaders(coreutils.Authorization, `Basic non-base64-value`), coreutils.Expect401()).Println()
			})
			t.Run("no colon between username and password", func(t *testing.T) {
				headerValue := base64.RawStdEncoding.EncodeToString([]byte("some password"))
				vit.PostProfile(prn, "c.sys.CUD", body,
					coreutils.WithHeaders(coreutils.Authorization, "Basic "+headerValue), coreutils.Expect401()).Println()
			})
		})
	})

	t.Run("403 forbidden", func(t *testing.T) {
		t.Run("missing Authorization token", func(t *testing.T) {
			// c.sys.CUD has Owner authorization policy -> need to provide authorization header in PostFunc()
			vit.PostApp(istructs.AppQName_test1_app1, prn.ProfileWSID, "c.sys.CUD", body, coreutils.Expect403()).Println()
		})
	})
}

func TestUtilFuncs(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	t.Run("func Echo", func(t *testing.T) {
		body := `{"args": {"Text": "world"},"elements":[{"fields":["Res"]}]}`
		resp := vit.PostApp(istructs.AppQName_test1_app1, 1, "q.sys.Echo", body)
		require.Equal("world", resp.SectionRow()[0].(string))
		resp.Println()
	})

	t.Run("func GRCount", func(t *testing.T) {
		body := `{"args": {},"elements":[{"fields":["NumGoroutines"]}]}`
		resp := vit.PostApp(istructs.AppQName_test1_app1, 1, "q.sys.GRCount", body)
		resp.Println()
	})

	t.Run("func Modules", func(t *testing.T) {
		// should normally return nothing because there is no dpes information in tests
		// returns actual deps if the Voedger is used in some main() and built using `go build`
		body := `{"args": {},"elements":[{"fields":["Modules"]}]}`
		resp := vit.PostApp(istructs.AppQName_test1_app1, 1, "q.sys.Modules", body)
		resp.Println()
	})
}

func Test400BadRequests(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	var err error

	cases := []struct {
		desc     string
		funcName string
		appName  string
	}{
		{desc: "unknown func", funcName: "q.unknown"},
		{desc: "unknown func kind", funcName: "x.test.test"},
		{desc: "wrong resource name", funcName: "a"},
		{desc: "unknown app", funcName: "c.sys.CUD", appName: "un/known"},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			appQName := istructs.AppQName_test1_app1
			if len(c.appName) > 0 {
				appQName, err = istructs.ParseAppQName(c.appName)
				require.NoError(t, err)
			}
			vit.PostApp(appQName, 1, c.funcName, "", coreutils.Expect400()).Println()
		})
	}
}

func Test503OnNoQueryProcessorsAvailable(t *testing.T) {
	funcStarted := make(chan interface{})
	okToFinish := make(chan interface{})
	it.MockQryExec = func(input string, callback istructs.ExecQueryCallback) error {
		funcStarted <- nil
		<-okToFinish
		return nil
	}
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	body := `{"args": {"Input": "world"},"elements": [{"fields": ["Res"]}]}`
	postDone := sync.WaitGroup{}
	sys := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
	for i := 0; i < int(vit.VVMConfig.NumCommandProcessors); i++ {
		postDone.Add(1)
		go func() {
			defer postDone.Done()
			vit.PostWS(ws, "q.app1pkg.MockQry", body, coreutils.WithAuthorizeBy(sys.Token))
		}()

		<-funcStarted
	}

	// one more request to any WSID -> 503 service unavailable
	vit.PostApp(istructs.AppQName_test1_app1, 1, "q.sys.Echo", body, coreutils.Expect503(), coreutils.WithAuthorizeBy(sys.Token))

	for i := 0; i < int(vit.VVMConfig.NumQueryProcessors); i++ {
		okToFinish <- nil
	}
	postDone.Wait()
}

func TestCmdResult(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("basic usage", func(t *testing.T) {

		body := `{"args":{"Arg1": 1}}`
		resp := vit.PostWS(ws, "c.app1pkg.TestCmd", body)
		resp.Println()
		require.Equal("Str", resp.CmdResult["Str"])
		require.Equal(float64(42), resp.CmdResult["Int"])
	})

	// ok - just required field is filled
	t.Run("just required fields filled", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"Arg1":%d}}`, 2)
		resp := vit.PostWS(ws, "c.app1pkg.TestCmd", body)
		resp.Println()
		require.Equal(float64(42), resp.CmdResult["Int"])
	})

	t.Run("missing required fields -> 500", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"Arg1":%d}}`, 3)
		vit.PostWS(ws, "c.app1pkg.TestCmd", body, coreutils.Expect500()).Println()
	})

	t.Run("wrong types -> 500", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"Arg1":%d}}`, 4)
		vit.PostWS(ws, "c.app1pkg.TestCmd", body, coreutils.Expect500()).Println()
	})
}

func TestIsActiveValidation(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("deny update sys.IsActive and other fields", func(t *testing.T) {
		body := `{"cuds":[{"fields":{"sys.ID": 1,"sys.QName":"app1pkg.air_table_plan"}}]}`
		id := vit.PostWS(ws, "c.sys.CUD", body).NewID()
		body = fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"sys.IsActive":false,"name":"newName"}}]}`, id)
		vit.PostWS(ws, "c.sys.CUD", body, coreutils.Expect403()).Println()
	})

	t.Run("deny insert a deactivated record", func(t *testing.T) {
		body := `{"cuds":[{"fields":{"sys.ID": 1,"sys.QName":"app1pkg.air_table_plan","sys.IsActive":false}}]}`
		vit.PostWS(ws, "c.sys.CUD", body, coreutils.Expect403()).Println()
	})
}

func TestTakeQNamesFromWorkspace(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	t.Run("command", func(t *testing.T) {
		t.Run("existence", func(t *testing.T) {

			anotherWS := vit.WS(istructs.AppQName_test1_app1, "test_ws_another")
			body := fmt.Sprintf(`{"args":{"Arg1":%d}}`, 1)
			// c.app1pkg.TestCmd is not defined in test_ws_anotherWS workspace -> 400 bad request
			vit.PostWS(anotherWS, "c.app1pkg.TestCmd", body, coreutils.Expect400("command app1pkg.TestCmd does not exist in workspace app1pkg.test_wsWS_another"))

			ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
			body = "{}"
			// c.app1pkg.testCmd is defined in test_wsWS workspace -> 400 bad request
			vit.PostWS(ws, "c.app1pkg.testCmd", body, coreutils.Expect400("command app1pkg.testCmd does not exist in workspace app1pkg.test_wsWS"))
		})

		t.Run("type", func(t *testing.T) {
			ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
			body := "{}"
			// c.app1pkg.testCmd is defined in test_wsWS workspace -> 400 bad request
			vit.PostWS(ws, "c.app1pkg.MockQry", body, coreutils.Expect400("app1pkg.MockQry is not a command"))
		})
	})

	t.Run("query", func(t *testing.T) {
		t.Run("existensce", func(t *testing.T) {
			anotherWS := vit.WS(istructs.AppQName_test1_app1, "test_ws_another")
			body := `{"args":{"Input":"str"}}`
			// q.app1pkg.MockQry is not defined in test_ws_anotherWS workspace -> 400 bad request
			vit.PostWS(anotherWS, "q.app1pkg.MockQry", body, coreutils.Expect400("query app1pkg.MockQry does not exist in workspace app1pkg.test_wsWS_another"))
		})
		t.Run("type", func(t *testing.T) {
			anotherWS := vit.WS(istructs.AppQName_test1_app1, "test_ws_another")
			body := fmt.Sprintf(`{"args":{"Arg1":%d}}`, 1)
			vit.PostWS(anotherWS, "q.app1pkg.testCmd", body, coreutils.Expect400("app1pkg.testCmd is not a query"))
		})
	})

	t.Run("CUDs QNames", func(t *testing.T) {
		t.Run("CUD in the request", func(t *testing.T) {
			anotherWS := vit.WS(istructs.AppQName_test1_app1, "test_ws_another")
			body := `{"cuds":[{"fields":{"sys.ID": 1,"sys.QName":"app1pkg.options"}}]}`
			// try to insert a QName that does not exist in the workspace -> 403 foribidden ??? or 400 bad request ???
			vit.PostWS(anotherWS, "c.sys.CUD", body, coreutils.Expect400("app1pkg.options", "does not exist in the workspace app1pkg.test_ws_another"))
		})
		t.Run("CUD produced by a command", func(t *testing.T) {
			it.MockCmdExec = func(input string, args istructs.ExecCommandArgs) error {
				kb, err := args.State.KeyBuilder(state.Record, appdef.NewQName("app1pkg", "docInAnotherWS"))
				if err != nil {
					return err
				}
				vb, err := args.Intents.NewValue(kb)
				if err != nil {
					return err
				}
				vb.PutRecordID(appdef.SystemField_ID, 1)
				return nil
			}
			body := `{"args":{"Input":"Str"}}`
			ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
			vit.PostWS(ws, "c.app1pkg.MockCmd", body, coreutils.WithExpectedCode(500, "app1pkg.docInAnotherWS is not available in workspace with descriptor app1pkg.test_ws"))
		})
	})
}
