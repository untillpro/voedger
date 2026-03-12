/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package commandprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/builder"
	"github.com/voedger/voedger/pkg/appdef/constraints"
	"github.com/voedger/voedger/pkg/appdef/filter"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	wsdescutil "github.com/voedger/voedger/pkg/coreutils/testwsdesc"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/testingu"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iauthnzimpl"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/in10nmem"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/isecretsimpl"
	"github.com/voedger/voedger/pkg/isequencer"
	"github.com/voedger/voedger/pkg/istorage/mem"
	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/processors/actualizers"
	"github.com/voedger/voedger/pkg/vvm/engines"
)

var (
	testCRecord = appdef.NewQName("test", "TestCRecord")
	testCDoc    = appdef.NewQName("test", "TestCDoc")
	testWDoc    = appdef.NewQName("test", "TestWDoc")
)

func TestBasicUsage(t *testing.T) {
	require := require.New(t)
	check := make(chan interface{}, 1)

	testCmdQName := appdef.NewQName(appdef.SysPackage, "Test")
	// schema of parameters of the test command
	testCmdQNameParams := appdef.NewQName(appdef.SysPackage, "TestParams")
	// schema of unlogged parameters of the test command
	testCmdQNameParamsUnlogged := appdef.NewQName(appdef.SysPackage, "TestParamsUnlogged")
	prepareWS := func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		pars := wsb.AddObject(testCmdQNameParams)
		pars.AddField("Text", appdef.DataKind_string, true)

		unloggedPars := wsb.AddObject(testCmdQNameParamsUnlogged)
		unloggedPars.AddField("Password", appdef.DataKind_string, true)

		wsb.AddCDoc(testCDoc).AddContainer("TestCRecord", testCRecord, 0, 1)
		wsb.AddCRecord(testCRecord)

		wsb.AddCommand(testCmdQName).SetUnloggedParam(testCmdQNameParamsUnlogged).SetParam(testCmdQNameParams)

		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)

		// the test command itself
		testExec := func(args istructs.ExecCommandArgs) (err error) {
			require.Equal(istructs.WSID(1), args.PrepareArgs.WSID)
			require.NotNil(args.State)

			// check that we received exactly what client sent
			text := args.ArgumentObject.AsString("Text")
			if text == "fire error" {
				return errors.New(text)
			} else {
				require.Equal("hello", text)
			}
			require.Equal("pass", args.ArgumentUnloggedObject.AsString("Password"))

			check <- 1 // signal that the checking is done
			return
		}
		testCmd := istructsmem.NewCommandFunction(testCmdQName, testExec)
		cfg.Resources.Add(testCmd)
	}

	app := setUp(t, prepareWS)
	defer tearDown(app)

	channelID, channelCleanup, err := app.n10nBroker.NewChannel("test", 24*time.Hour)
	require.NoError(err)
	defer channelCleanup()
	projectionKey := in10n.ProjectionKey{
		App:        istructs.AppQName_untill_airs_bp,
		Projection: actualizers.PLogUpdatesQName,
		WS:         1,
	}
	go app.n10nBroker.WatchChannel(app.ctx, channelID, func(projection in10n.ProjectionKey, _ istructs.Offset) {
		require.Equal(projectionKey, projection)
		check <- 1
	})
	err = app.n10nBroker.Subscribe(channelID, projectionKey)
	require.NoError(err)
	defer func() { require.NoError(app.n10nBroker.Unsubscribe(channelID, projectionKey)) }()

	t.Run("basic usage", func(t *testing.T) {
		// command processor works through ibus.SendResponse -> we need a sender -> let's test using ibus.SendRequest2()
		request := bus.Request{
			Body:     []byte(`{"args":{"Text":"hello"},"unloggedArgs":{"Password":"pass"}}`),
			AppQName: istructs.AppQName_untill_airs_bp,
			WSID:     1,
			Resource: "c.sys.Test",
			// need to authorize, otherwise execute will be forbidden
			Header: app.sysAuthHeader,
		}

		respCh, respMeta, respErr, err := app.requestSender.SendRequest(app.ctx, request)
		require.NoError(err)
		respData := ""
		for elem := range respCh {
			require.Empty(respData)
			respDataBytes, err := json.Marshal(elem)
			require.NoError(err)
			respData = string(respDataBytes)
		}
		require.NoError(*respErr)
		log.Println(respData)
		require.Equal(http.StatusOK, respMeta.StatusCode)
		require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
		// check that command is handled and notifications were sent
		<-check
		<-check
	})

	t.Run("500 internal server error command exec error", func(t *testing.T) {
		request := bus.Request{
			Body:     []byte(`{"args":{"Text":"fire error"},"unloggedArgs":{"Password":"pass"}}`),
			AppQName: istructs.AppQName_untill_airs_bp,
			WSID:     1,
			Resource: "c.sys.Test",
			Header:   app.sysAuthHeader,
		}
		respCh, respMeta, respErr, err := app.requestSender.SendRequest(app.ctx, request)
		require.NoError(err)
		counter := 0
		for elem := range respCh {
			require.Zero(counter)
			require.Equal(http.StatusInternalServerError, respMeta.StatusCode)
			require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
			require.Equal(coreutils.SysError{HTTPStatus: http.StatusInternalServerError, Message: "fire error"}, elem) // nolint:errorlint
			counter++
		}
		require.NoError(*respErr)
	})
}

func sendCUD(t *testing.T, wsid istructs.WSID, app testApp, expectedCode ...int) map[string]interface{} {
	require := require.New(t)
	req := bus.Request{
		WSID:     wsid,
		AppQName: istructs.AppQName_untill_airs_bp,
		Resource: "c.sys.CUD",
		Body: []byte(`{"cuds":[
			{"fields":{"sys.ID":1,"sys.QName":"test.TestCDoc"}},
			{"fields":{"sys.ID":2,"sys.QName":"test.TestWDoc"}},
			{"fields":{"sys.ID":3,"sys.QName":"test.TestCRecord","sys.ParentID":1,"sys.Container":"TestCRecord"}}
		]}`),
		Header: app.sysAuthHeader,
	}
	respCh, respMeta, respErr, err := app.requestSender.SendRequest(app.ctx, req)
	require.NoError(err)
	respDataStr := ""
	for elem := range respCh {
		require.Empty(respDataStr)
		switch typed := elem.(type) {
		case string:
			respDataStr = typed
		case coreutils.SysError:
			respDataStr = typed.ToJSON_APIV1()
		}
	}
	if len(expectedCode) == 0 {
		require.Equal(http.StatusOK, respMeta.StatusCode)
	} else {
		require.Equal(expectedCode[0], respMeta.StatusCode)
	}

	respData := map[string]interface{}{}
	if len(respDataStr) > 0 {
		require.NoError(json.Unmarshal([]byte(respDataStr), &respData))
	}
	require.NoError(*respErr)
	return respData
}

func TestRecoveryOnSyncProjectorError(t *testing.T) {
	require := require.New(t)

	cudQName := appdef.NewQName(appdef.SysPackage, "CUD")
	testErr := errors.New("test error")
	counter := 0
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCRecord(testCRecord)
		wsb.AddCDoc(testCDoc).AddContainer("TestCRecord", testCRecord, 0, 1)
		wsb.AddWDoc(testWDoc)
		wsb.AddCommand(cudQName)

		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)

		failingProjQName := appdef.NewQName(appdef.SysPackage, "Failer")
		cfg.AddSyncProjectors(
			istructs.Projector{
				Name: failingProjQName,
				Func: func(istructs.IPLogEvent, istructs.IState, istructs.IIntents) error {
					counter++
					if counter == 3 { // 1st event is insert WorkspaceDescriptor stub
						return testErr
					}
					return nil
				},
			})
		wsb.AddProjector(failingProjQName).SetSync(true).Events().Add(
			[]appdef.OperationKind{appdef.OperationKind_Execute},
			filter.QNames(cudQName))
		cfg.Resources.Add(istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec))
	})
	defer tearDown(app)

	// ok to c.sys.CUD
	respData := sendCUD(t, 1, app)
	require.Equal(2, int(respData["CurrentWLogOffset"].(float64))) // 1st is WorkspaceDescriptor stub insert
	require.Equal(istructs.FirstUserRecordID, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.FirstUserRecordID+1, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.FirstUserRecordID+2, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	// 2nd c.sys.CUD -> sync projector failure, expect 500 internal server error
	respData = sendCUD(t, 1, app, http.StatusInternalServerError)
	require.Equal(testErr.Error(), respData["sys.Error"].(map[string]interface{})["Message"].(string))

	// PLog and record is applied but WLog is not written here because sync projector is failed
	// partition is scheduled to be recovered

	// 3rd c.sys.CUD - > recovery procedure must re-apply 2nd event (PLog, records and WLog), then 3rd event is processed ok (sync projectors are ok)
	respData = sendCUD(t, 1, app)
	require.Equal(4, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.FirstUserRecordID+6, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.FirstUserRecordID+7, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.FirstUserRecordID+8, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))
}

func TestRecovery(t *testing.T) {
	require := require.New(t)

	cudQName := appdef.NewQName(appdef.SysPackage, "CUD")
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCRecord(testCRecord)
		wsb.AddCDoc(testCDoc).AddContainer("TestCRecord", testCRecord, 0, 1)
		wsb.AddWDoc(testWDoc)
		wsb.AddCommand(cudQName)
		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)
		cfg.Resources.Add(istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec))
	})
	defer tearDown(app)

	cmdCUD := istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec)
	app.cfg.Resources.Add(cmdCUD)

	respData := sendCUD(t, 1, app)
	require.Equal(2, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.FirstUserRecordID, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.FirstUserRecordID+1, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.FirstUserRecordID+2, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	restartCmdProc(&app)
	respData = sendCUD(t, 1, app)
	require.Equal(3, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.FirstUserRecordID+3, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.FirstUserRecordID+4, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.FirstUserRecordID+5, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	restartCmdProc(&app)
	respData = sendCUD(t, 2, app)
	require.Equal(2, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.FirstUserRecordID, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.FirstUserRecordID+1, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.FirstUserRecordID+2, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	restartCmdProc(&app)
	respData = sendCUD(t, 1, app)
	require.Equal(4, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.FirstUserRecordID+6, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.FirstUserRecordID+7, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.FirstUserRecordID+8, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	app.cancel()
	<-app.done
}

func restartCmdProc(app *testApp) {
	app.cancel()
	<-app.done
	app.ctx, app.cancel = context.WithCancel(context.Background())
	app.done = make(chan struct{})
	go func() {
		app.cmdProcService.Run(app.ctx)
		close(app.done)
	}()
}

func TestCUDUpdate(t *testing.T) {
	require := require.New(t)

	testQName := appdef.NewQName("test", "test")

	cudQName := appdef.NewQName(appdef.SysPackage, "CUD")
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCDoc(testQName).AddField("IntFld", appdef.DataKind_int32, false)
		wsb.AddCommand(cudQName)
		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)
		cfg.Resources.Add(istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec))
	})
	defer tearDown(app)

	cmdCUD := istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec)
	app.cfg.Resources.Add(cmdCUD)

	// insert
	req := bus.Request{
		WSID:     1,
		AppQName: istructs.AppQName_untill_airs_bp,
		Resource: "c.sys.CUD",
		Body:     []byte(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"test.test"}}]}`),
		Header:   app.sysAuthHeader,
	}
	cmdRespMeta, cmdResp, sysErr := bus.GetCommandResponse(app.ctx, app.requestSender, req)
	require.NoError(sysErr)
	require.Equal(http.StatusOK, cmdRespMeta.StatusCode)
	require.Equal(httpu.ContentType_ApplicationJSON, cmdRespMeta.ContentType)
	require.Empty(cmdResp.CmdResult)
	newID := cmdResp.NewIDs["1"]
	require.NotZero(newID)

	t.Run("update", func(t *testing.T) {
		req.Body = []byte(fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"sys.QName":"test.test", "IntFld": 42}}]}`, newID))
		cmdRespMeta, _, err := bus.GetCommandResponse(app.ctx, app.requestSender, req)
		require.NoError(err)
		require.Equal(http.StatusOK, cmdRespMeta.StatusCode)
		require.Equal(httpu.ContentType_ApplicationJSON, cmdRespMeta.ContentType)
	})

	t.Run("404 not found on update not existing", func(t *testing.T) {
		req.Body = []byte(fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"sys.QName":"test.test", "IntFld": 42}}]}`, istructs.NonExistingRecordID))
		cmdRespMeta, _, err := bus.GetCommandResponse(app.ctx, app.requestSender, req)
		require.Error(err)
		require.Equal(http.StatusNotFound, cmdRespMeta.StatusCode)
		require.Equal(httpu.ContentType_ApplicationJSON, cmdRespMeta.ContentType)
	})
}

func Test400BadRequestOnCUDErrors(t *testing.T) {
	require := require.New(t)

	testQName := appdef.NewQName("test", "test")

	cudQName := appdef.NewQName(appdef.SysPackage, "CUD")
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCDoc(testQName)
		wsb.AddCommand(cudQName)
		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)
		cfg.Resources.Add(istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec))
	})
	defer tearDown(app)

	cmdCUD := istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec)
	app.cfg.Resources.Add(cmdCUD)

	cases := []struct {
		desc                string
		bodyAdd             string
		expectedMessageLike string
	}{
		{"not an object", `"cuds":42`, `field "cuds" must be an array of objects`},
		{`element is not an object`, `"cuds":[42]`, `cuds[0]: not an object`},
		{`missing fields`, `"cuds":[{}]`, `cuds[0]: "fields" missing`},
		{`fields is not an object`, `"cuds":[{"fields":42}]`, `cuds[0]: field "fields" must be an object`},
		{`fields: sys.ID missing`, `"cuds":[{"fields":{"sys.QName":"test.Test"}}]`, `cuds[0]: "sys.ID" field missing`},
		{`fields: sys.ID is not a number (create)`, `"cuds":[{"sys.ID":"wrong","fields":{"sys.QName":"test.Test"}}]`, `cuds[0]: field "sys.ID" must be json.Number`},
		{`fields: sys.ID is not a number (update)`, `"cuds":[{"fields":{"sys.ID":"wrong","sys.QName":"test.Test"}}]`, `cuds[0]: field "sys.ID" must be json.Number`},
		{`fields: wrong qName`, `"cuds":[{"fields":{"sys.ID":1,"sys.QName":"wrong"}},{"fields":{"sys.ID":1,"sys.QName":"test.Test"}}]`, `convert error: string «wrong»`},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			req := bus.Request{
				WSID:     1,
				AppQName: istructs.AppQName_untill_airs_bp,
				Resource: "c.sys.CUD",
				Body:     []byte("{" + c.bodyAdd + "}"),
				Header:   app.sysAuthHeader,
			}
			cmdRespMeta, _, sysErr := bus.GetCommandResponse(app.ctx, app.requestSender, req)
			require.Equal(http.StatusBadRequest, cmdRespMeta.StatusCode, c.desc)
			require.Equal(httpu.ContentType_ApplicationJSON, cmdRespMeta.ContentType, c.desc)
			if sysErr != nil {
				var sysError coreutils.SysError
				require.ErrorAs(sysErr, &sysError)
				require.Contains(sysError.Message, c.expectedMessageLike, c.desc)
				require.Equal(http.StatusBadRequest, sysError.HTTPStatus, c.desc)
			} else {
				require.Empty(c.expectedMessageLike, c.desc)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	require := require.New(t)

	testCmdQNameParams := appdef.NewQName(appdef.SysPackage, "TestParams")
	testCmdQNameParamsUnlogged := appdef.NewQName(appdef.SysPackage, "TestParamsUnlogged")

	testCmdQName := appdef.NewQName(appdef.SysPackage, "Test")
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddObject(testCmdQNameParams).
			AddField("Text", appdef.DataKind_string, true)

		wsb.AddObject(testCmdQNameParamsUnlogged).
			AddField("Password", appdef.DataKind_string, true)

		wsb.AddCommand(testCmdQName).SetUnloggedParam(testCmdQNameParamsUnlogged).SetParam(testCmdQNameParams)

		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)

		cfg.Resources.Add(istructsmem.NewCommandFunction(testCmdQName, istructsmem.NullCommandExec))
	})
	defer tearDown(app)

	baseReq := bus.Request{
		WSID:     1,
		AppQName: istructs.AppQName_untill_airs_bp,
		Resource: "c.sys.Test",
		Body:     []byte(`{"args":{"Text":"hello"},"unloggedArgs":{"Password":"123"}}`),
		Header:   app.sysAuthHeader,
	}

	cases := []struct {
		desc string
		bus.Request
		expectedMessageLike string
		expectedStatusCode  int
	}{
		{"unknown app", bus.Request{AppQName: appdef.NewAppQName("untill", "unknown")}, "application untill/unknown not found", http.StatusServiceUnavailable},
		{"bad request body", bus.Request{Body: []byte("{wrong")}, "failed to unmarshal request body: invalid character 'w' looking for beginning of object key string", http.StatusBadRequest},
		{"unknown func", bus.Request{Resource: "c.sys.Unknown"}, "unknown function", http.StatusBadRequest},
		{"args: field of wrong type provided", bus.Request{Body: []byte(`{"args":{"Text":42}}`)}, "wrong field type", http.StatusBadRequest},
		{"args: not an object", bus.Request{Body: []byte(`{"args":42}`)}, `field "args" must be an object`, http.StatusBadRequest},
		{"args: missing at all with a required field", bus.Request{Body: []byte(`{}`)}, "", http.StatusBadRequest},
		{"unloggedArgs: not an object", bus.Request{Body: []byte(`{"unloggedArgs":42,"args":{"Text":"txt"}}`)}, `field "unloggedArgs" must be an object`, http.StatusBadRequest},
		{"unloggedArgs: field of wrong type provided", bus.Request{Body: []byte(`{"unloggedArgs":{"Password":42},"args":{"Text":"txt"}}`)}, "wrong field type", http.StatusBadRequest},
		{"unloggedArgs: missing required field of unlogged args, no unlogged args at all", bus.Request{Body: []byte(`{"args":{"Text":"txt"}}`)}, "", http.StatusBadRequest},
		{"cuds: not an object", bus.Request{Body: []byte(`{"args":{"Text":"hello"},"unloggedArgs":{"Password":"123"},"cuds":42}`)}, `field "cuds" must be an array of objects`, http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			req := baseReq
			req.Body = make([]byte, len(baseReq.Body))
			copy(req.Body, baseReq.Body)
			if c.AppQName != appdef.NullAppQName {
				req.AppQName = c.AppQName
			}
			if len(c.Body) > 0 {
				req.Body = make([]byte, len(c.Body))
				copy(req.Body, c.Body)
			}
			if len(c.Resource) > 0 {
				req.Resource = c.Resource
			}
			cmdRespMeta, _, sysErr := bus.GetCommandResponse(app.ctx, app.requestSender, req)
			require.Equal(c.expectedStatusCode, cmdRespMeta.StatusCode, c.desc)
			require.Equal(httpu.ContentType_ApplicationJSON, cmdRespMeta.ContentType, c.desc)
			if sysErr != nil {
				var sysError coreutils.SysError
				require.ErrorAs(sysErr, &sysError)
				require.Contains(sysError.Message, c.expectedMessageLike, c.desc)
				require.Equal(c.expectedStatusCode, sysError.HTTPStatus, c.desc)
			} else {
				require.Empty(c.expectedMessageLike, c.desc)
			}
		})
	}
}

func TestAuthnz(t *testing.T) {
	require := require.New(t)

	qNameTestDeniedCDoc := appdef.NewQName("app1pkg", "TestDeniedCDoc") // the same in core/iauthnzimpl

	qNameAllowedCmd := appdef.NewQName(appdef.SysPackage, "TestAllowedCmd")
	qNameDeniedCmd := appdef.NewQName(appdef.SysPackage, "TestDeniedCmd") // the same in core/iauthnzimpl
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCDoc(qNameTestDeniedCDoc)
		wsb.AddCommand(qNameAllowedCmd)
		wsb.AddCommand(qNameDeniedCmd)
		wsb.AddCommand(istructs.QNameCommandCUD)
		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)
		wsb.AddRole(iauthnz.QNameRoleProfileOwner)
		wsb.AddRole(iauthnz.QNameRoleAnonymous)
		wsb.AddRole(iauthnz.QNameRoleWorkspaceOwner)
		cfg.Resources.Add(istructsmem.NewCommandFunction(qNameAllowedCmd, istructsmem.NullCommandExec))
		cfg.Resources.Add(istructsmem.NewCommandFunction(qNameDeniedCmd, istructsmem.NullCommandExec))
		cfg.Resources.Add(istructsmem.NewCommandFunction(istructs.QNameCommandCUD, istructsmem.NullCommandExec))

		wsb.Revoke([]appdef.OperationKind{appdef.OperationKind_Execute}, filter.QNames(qNameDeniedCmd), nil, iauthnz.QNameRoleWorkspaceOwner)
	})
	defer tearDown(app)

	pp := payloads.PrincipalPayload{
		Login:       "testlogin",
		SubjectKind: istructs.SubjectKind_User,
		ProfileWSID: 1,
	}
	token, err := app.appTokens.IssueToken(10*time.Second, &pp)
	require.NoError(err)

	type testCase struct {
		desc               string
		req                bus.Request
		expectedStatusCode int
	}
	cases := []testCase{
		{
			desc: "403 on cmd EXECUTE forbidden", req: bus.Request{
				Body:     []byte(`{}`),
				AppQName: istructs.AppQName_untill_airs_bp,
				WSID:     1,
				Resource: "c.sys.TestDeniedCmd",
				Header:   getAuthHeader(token),
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			desc: "403 on INSERT CUD forbidden", req: bus.Request{
				Body:     []byte(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.TestDeniedCDoc"}}]}`),
				AppQName: istructs.AppQName_untill_airs_bp,
				WSID:     1,
				Resource: "c.sys.CUD",
				Header:   getAuthHeader(token),
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			desc: "403 if no token for a func that requires authentication", req: bus.Request{
				Body:     []byte(`{}`),
				AppQName: istructs.AppQName_untill_airs_bp,
				WSID:     1,
				Resource: "c.sys.TestAllowedCmd",
			},
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			cmdRespMeta, _, err := bus.GetCommandResponse(app.ctx, app.requestSender, c.req)
			require.Error(err)
			require.Equal(c.expectedStatusCode, cmdRespMeta.StatusCode, c.desc)
		})
	}
}

func getAuthHeader(token string) map[string]string {
	return map[string]string{
		httpu.Authorization: "Bearer " + token,
	}
}

func TestBasicUsage_FuncWithRawArg(t *testing.T) {
	require := require.New(t)
	testCmdQName := appdef.NewQName(appdef.SysPackage, "Test")
	ch := make(chan interface{})
	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCommand(testCmdQName).SetParam(istructs.QNameRaw)
		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)
		cfg.Resources.Add(istructsmem.NewCommandFunction(testCmdQName, func(args istructs.ExecCommandArgs) (err error) {
			require.Equal("custom content", args.ArgumentObject.AsString(processors.Field_RawObject_Body))
			close(ch)
			return
		}))
	})
	defer tearDown(app)

	request := bus.Request{
		Body:     []byte(`custom content`),
		AppQName: istructs.AppQName_untill_airs_bp,
		WSID:     1,
		Resource: "c.sys.Test",
		Header:   app.sysAuthHeader,
	}
	cmdRespMeta, _, err := bus.GetCommandResponse(app.ctx, app.requestSender, request)
	require.NoError(err)
	require.Equal(http.StatusOK, cmdRespMeta.StatusCode)
	require.Equal(httpu.ContentType_ApplicationJSON, cmdRespMeta.ContentType)
	<-ch
}

func TestRateLimit(t *testing.T) {
	require := require.New(t)

	qName := appdef.NewQName(appdef.SysPackage, "MyCmd")
	parsQName := appdef.NewQName(appdef.SysPackage, "Params")

	app := setUp(t,
		func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
			wsb.AddObject(parsQName)
			wsb.AddCommand(qName).SetParam(parsQName)
			wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
			wsb.AddRole(iauthnz.QNameRoleEveryone)
			wsb.AddRole(iauthnz.QNameRoleSystem)
			cfg.Resources.Add(istructsmem.NewCommandFunction(qName, istructsmem.NullCommandExec))

			cfg.FunctionRateLimits.AddWorkspaceLimit(qName, istructs.RateLimit{
				Period:                time.Minute,
				MaxAllowedPerDuration: 2,
			})
		})
	defer tearDown(app)

	request := bus.Request{
		Body:     []byte(`{"args":{}}`),
		AppQName: istructs.AppQName_untill_airs_bp,
		WSID:     1,
		Resource: "c.sys.MyCmd",
		Header:   app.sysAuthHeader,
	}

	// first 2 calls are ok
	for i := 0; i < 2; i++ {
		cmdRespMeta, _, err := bus.GetCommandResponse(app.ctx, app.requestSender, request)
		require.NoError(err)
		require.Equal(http.StatusOK, cmdRespMeta.StatusCode)
	}

	// 3rd exceeds rate limits
	cmdRespMeta, _, err := bus.GetCommandResponse(app.ctx, app.requestSender, request)
	require.Error(err)
	require.Equal(http.StatusTooManyRequests, cmdRespMeta.StatusCode)
}

type testApp struct {
	ctx               context.Context
	cfg               *istructsmem.AppConfigType
	cancel            context.CancelFunc
	done              chan struct{}
	cmdProcService    pipeline.IService
	serviceChannel    CommandChannel
	n10nBroker        in10n.IN10nBroker
	n10nBrokerCleanup func()
	requestSender     bus.IRequestSender

	appTokens     istructs.IAppTokens
	sysAuthHeader map[string]string
}

func tearDown(app testApp) {
	// finish the command processors IService
	app.n10nBrokerCleanup()
	app.cancel()
	<-app.done
}

// test app deployment constants
var (
	testAppName                                = istructs.AppQName_untill_airs_bp
	testAppEngines                             = [appparts.ProcessorKind_Count]uint{10, 10, 10, 0}
	testAppPartID    istructs.PartitionID      = 1
	testAppPartCount istructs.NumAppPartitions = 1
)

func setUp(t *testing.T, prepare func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType)) testApp {
	require := require.New(t)
	// command processor is a IService working through CommandChannel(iprocbus.ServiceChannel). Let's prepare that channel
	serviceChannel := make(CommandChannel)
	done := make(chan struct{})

	vvmCtx, cancel := context.WithCancel(context.Background())

	cfgs := istructsmem.AppConfigsType{}
	asf := mem.Provide(testingu.MockTime)
	appStorageProvider := istorageimpl.Provide(asf)

	// build application
	adb := builder.New()
	adb.AddPackage("test", "test.com/test")

	qNameTestWS, qNameTestWSKind := appdef.NewQName(appdef.SysPackage, "TestWS"), appdef.NewQName(appdef.SysPackage, "TestWSKind")
	wsb := adb.AddWorkspace(qNameTestWS)
	wsb.AddCDoc(qNameTestWSKind).SetSingleton()
	wsb.SetDescriptor(qNameTestWSKind)

	wsdescutil.AddWorkspaceDescriptorStubDef(wsb)

	wsb.AddObject(istructs.QNameRaw).AddField(processors.Field_RawObject_Body, appdef.DataKind_string, true, constraints.MaxLen(appdef.MaxFieldLength))

	statelessResources := istructsmem.NewStatelessResources()
	cfg := cfgs.AddBuiltInAppConfig(istructs.AppQName_untill_airs_bp, adb)
	cfg.SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)
	if prepare != nil {
		prepare(wsb, cfg)
	}

	appDef, err := adb.Build()
	require.NoError(err)

	appStructsProvider := istructsmem.Provide(cfgs, iratesce.TestBucketsFactory,
		payloads.ProvideIAppTokensFactory(itokensjwt.TestTokensJWT()), appStorageProvider, isequencer.SequencesTrustLevel_0, nil)

	secretReader := isecretsimpl.ProvideSecretReader()
	n10nBroker, n10nBrokerCleanup := in10nmem.NewN10nBroker(in10n.Quotas{
		Channels:                1000,
		ChannelsPerSubject:      10,
		Subscriptions:           1000,
		SubscriptionsPerSubject: 10,
	}, timeu.NewITime())

	// prepare the AppParts to borrow AppStructs
	appParts, appPartsClean, err := appparts.New2(vvmCtx, appStructsProvider,
		actualizers.NewSyncActualizerFactoryFactory(actualizers.ProvideSyncActualizerFactory(), secretReader, n10nBroker, statelessResources),
		appparts.NullActualizerRunner,
		appparts.NullSchedulerRunner,
		engines.ProvideExtEngineFactories(
			engines.ExtEngineFactoriesConfig{
				AppConfigs:         cfgs,
				StatelessResources: statelessResources,
				WASMConfig:         iextengine.WASMFactoryConfig{Compile: false},
			}, "", imetrics.Provide()),
		iratesce.TestBucketsFactory)
	require.NoError(err)

	appParts.DeployApp(testAppName, nil, appDef, testAppPartCount, testAppEngines, cfg.NumAppWorkspaces())
	appParts.DeployAppPartitions(testAppName, []istructs.PartitionID{testAppPartID})

	// command processor works through ibus.SendResponse -> we need ibus implementation

	requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
		// simulate handling the command request be a real application
		cmdQName, err := appdef.ParseQName(request.Resource[2:])
		require.NoError(err)
		tp := appDef.Type(cmdQName)
		if tp.Kind() == appdef.TypeKind_null {
			bus.ReplyBadRequest(responder, "unknown function")
			return
		}
		token := ""
		if authHeader, ok := request.Header[httpu.Authorization]; ok {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
		icm := NewCommandMessage(vvmCtx, request.Body, request.AppQName, request.WSID, responder, testAppPartID, cmdQName, token, "", 0, 0, "", "")
		serviceChannel <- icm
	})

	tokens := itokensjwt.TestTokensJWT()
	appTokens := payloads.ProvideIAppTokensFactory(tokens).New(testAppName)
	systemToken, err := payloads.GetSystemPrincipalTokenApp(appTokens)
	require.NoError(err)
	cmdProcessorFactory := ProvideServiceFactory(appParts, timeu.NewITime(), n10nBroker, imetrics.Provide(), "vvm",
		iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter, iauthnzimpl.TestIsDeviceAllowedFuncs), secretReader)
	cmdProcService := cmdProcessorFactory(serviceChannel)

	go func() {
		cmdProcService.Run(vvmCtx)
		close(done)
	}()

	as, err := appStructsProvider.BuiltIn(istructs.AppQName_untill_airs_bp)
	require.NoError(err)
	err = wsdescutil.CreateCDocWorkspaceDescriptorStub(as, testAppPartID, 1, qNameTestWSKind, 1, 1)
	require.NoError(err)
	err = wsdescutil.CreateCDocWorkspaceDescriptorStub(as, testAppPartID, 2, qNameTestWSKind, 2, 1)
	require.NoError(err)

	return testApp{
		cfg:               cfg,
		requestSender:     requestSender,
		cancel:            func() { cancel(); appPartsClean() },
		ctx:               vvmCtx,
		done:              done,
		cmdProcService:    cmdProcService,
		serviceChannel:    serviceChannel,
		n10nBroker:        n10nBroker,
		n10nBrokerCleanup: n10nBrokerCleanup,
		appTokens:         appTokens,
		sysAuthHeader:     getAuthHeader(systemToken),
	}
}

func TestLogEventAndCUDs(t *testing.T) {
	testQName := appdef.NewQName("test", "Article")
	cudQName := appdef.NewQName(appdef.SysPackage, "CUD")

	var buf syncBuffer
	logger.SetCtxWriters(&buf, &buf)
	defer logger.SetCtxWriters(os.Stdout, os.Stderr)

	app := setUp(t, func(wsb appdef.IWorkspaceBuilder, cfg *istructsmem.AppConfigType) {
		wsb.AddCDoc(testQName).AddField("Name", appdef.DataKind_string, false)
		wsb.AddCommand(cudQName)
		wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
		wsb.AddRole(iauthnz.QNameRoleEveryone)
		wsb.AddRole(iauthnz.QNameRoleSystem)
		cfg.Resources.Add(istructsmem.NewCommandFunction(cudQName, istructsmem.NullCommandExec))
	})
	defer tearDown(app)

	t.Run("verbose level emits event and cud log entries", func(t *testing.T) {
		require := require.New(t)
		defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()

		buf.Reset()

		// insert
		req := bus.Request{
			WSID:     1,
			AppQName: istructs.AppQName_untill_airs_bp,
			Resource: "c.sys.CUD",
			Body:     []byte(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"test.Article","Name":"hello"}}]}`),
			Header:   app.sysAuthHeader,
		}
		cmdRespMeta, cmdResp, sysErr := bus.GetCommandResponse(app.ctx, app.requestSender, req)
		require.NoError(sysErr)
		require.Equal(http.StatusOK, cmdRespMeta.StatusCode)
		newID := cmdResp.NewIDs["1"]
		require.NotZero(newID)

		out := buf.String()
		require.Contains(out, "evqname=sys.CUD")
		require.Contains(out, fmt.Sprintf("rectype=%s", testQName))
		require.Contains(out, fmt.Sprintf("recid=%d", newID))
		require.Contains(out, "op=create")

		// update: newfields=world, oldfields=hello
		buf.Reset()
		req.Body = []byte(fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"sys.QName":"test.Article","Name":"world"}}]}`, newID))
		cmdRespMeta, _, sysErr = bus.GetCommandResponse(app.ctx, app.requestSender, req)
		require.NoError(sysErr)
		require.Equal(http.StatusOK, cmdRespMeta.StatusCode)

		out = buf.String()
		require.Contains(out, fmt.Sprintf("recid=%d", newID))
		require.Contains(out, "op=update")
		require.Contains(out, "newfields=")
		require.Contains(out, "oldfields=")
		require.Contains(out, "world")
		require.Contains(out, "hello")
	})

	t.Run("info level emits nothing", func(t *testing.T) {
		require := require.New(t)
		defer logger.SetLogLevelWithRestore(logger.LogLevelInfo)()

		buf.Reset()

		req := bus.Request{
			WSID:     1,
			AppQName: istructs.AppQName_untill_airs_bp,
			Resource: "c.sys.CUD",
			Body:     []byte(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"test.Article"}}]}`),
			Header:   app.sysAuthHeader,
		}
		_, _, sysErr := bus.GetCommandResponse(app.ctx, app.requestSender, req)
		require.NoError(sysErr)
		require.NotContains(buf.String(), "evqname")
	})
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

func (sb *syncBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.buf.Reset()
}
