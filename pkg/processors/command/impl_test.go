/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package commandprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	in10n "github.com/heeus/core-in10n"
	in10nmem "github.com/heeus/core-in10nmem"
	pipeline "github.com/heeus/core-pipeline"
	projectors "github.com/heeus/core-projectors"
	"github.com/stretchr/testify/require"

	ibus "github.com/untillpro/airs-ibus"
	"github.com/untillpro/ibusmem"
	"github.com/untillpro/voedger/pkg/iauthnzimpl"
	"github.com/untillpro/voedger/pkg/iratesce"
	"github.com/untillpro/voedger/pkg/isecretsimpl"
	"github.com/untillpro/voedger/pkg/istorage"
	"github.com/untillpro/voedger/pkg/istorageimpl"
	"github.com/untillpro/voedger/pkg/istructs"
	istructsmem "github.com/untillpro/voedger/pkg/istructsmem"
	payloads "github.com/untillpro/voedger/pkg/itokens-payloads"
	"github.com/untillpro/voedger/pkg/itokensjwt"
	imetrics "github.com/untillpro/voedger/pkg/metrics"
	coreutils "github.com/untillpro/voedger/pkg/utils"
)

var (
	testCRecord = istructs.NewQName("test", "TestCRecord")
	testCDoc    = istructs.NewQName("test", "TestCDoc")
	testWDoc    = istructs.NewQName("test", "TestWDoc")
	testTimeout = ibus.DefaultTimeout
)

func TestBasicUsage(t *testing.T) {
	require := require.New(t)
	check := make(chan interface{}, 1)
	cudsCheck := make(chan interface{})
	app := setUp(t)
	defer tearDown(app)

	channelID, err := app.n10nBroker.NewChannel("test", 24*time.Hour)
	require.Nil(err)
	projectionKey := in10n.ProjectionKey{
		App:        istructs.AppQName_untill_airs_bp,
		Projection: projectors.PlogQName,
		WS:         1,
	}
	go app.n10nBroker.WatchChannel(app.ctx, channelID, func(projection in10n.ProjectionKey, _ istructs.Offset) {
		require.Equal(projectionKey, projection)
		check <- 1
	})
	app.n10nBroker.Subscribe(channelID, projectionKey)
	defer app.n10nBroker.Unsubscribe(channelID, projectionKey)

	// схема параметров тестовой команды
	testCmdQNameParams := istructs.NewQName(istructs.SysPackage, "TestParams")
	testCmdParamsScheme := app.cfg.Schemas.Add(testCmdQNameParams, istructs.SchemaKind_Object)
	testCmdParamsScheme.AddField("Text", istructs.DataKind_string, true)

	// схема unloged-параметров тестовой команды
	testCmdQNameParamsUnlogged := istructs.NewQName(istructs.SysPackage, "TestParamsUnlogged")
	testCmdParamsUnloggedScheme := app.cfg.Schemas.Add(testCmdQNameParamsUnlogged, istructs.SchemaKind_Object)
	testCmdParamsUnloggedScheme.AddField("Password", istructs.DataKind_string, true)

	// сама тестовая команда
	testCmdQName := istructs.NewQName(istructs.SysPackage, "Test")
	testExec := func(cf istructs.ICommandFunction, args istructs.ExecCommandArgs) (err error) {
		cuds := args.Workpiece.(*cmdWorkpiece).parsedCUDs
		if len(cuds) > 0 {
			require.True(len(cuds) == 1)
			require.Equal(float64(1), cuds[0].fields[istructs.SystemField_ID])
			require.Equal(testCDoc.String(), cuds[0].fields[istructs.SystemField_QName])
			close(cudsCheck)
		}
		require.Equal(istructs.WSID(1), args.PrepareArgs.Workspace)
		require.NotNil(args.State)

		// просто проверим, что мы получили то, что передал клиент
		text := args.ArgumentObject.AsString("Text")
		if text == "fire error" {
			return errors.New(text)
		} else {
			require.Equal("hello", text)
		}
		require.Equal("pass", args.ArgumentUnloggedObject.AsString("Password"))

		check <- 1 // сигнал: проверки случились
		return
	}
	testCmd := istructsmem.NewCommandFunction(testCmdQName, testCmdQNameParams, testCmdQNameParamsUnlogged, istructs.NullQName, testExec)
	app.cfg.Resources.Add(testCmd)
	app.cfg.Schemas.Add(testCDoc, istructs.SchemaKind_CDoc).AddContainer("TestCRecord", testCRecord, 0, 1)
	app.cfg.Schemas.Add(testCRecord, istructs.SchemaKind_CRecord)

	t.Run("basic usage", func(t *testing.T) {
		// commandprocessor работает через ibus.SendResponse -> нам нужен sender -> тестируем через ibus.SendRequest2()
		request := ibus.Request{
			Body:     []byte(`{"args":{"Text":"hello"},"unloggedArgs":{"Password":"pass"},"cuds":[{"fields":{"sys.ID":1,"sys.QName":"test.TestCDoc"}}]}`),
			AppQName: istructs.AppQName_untill_airs_bp.String(),
			WSID:     1,
			Resource: "c.sys.Test",
			// need to authorize, otherwise execute will be forbidden
			Header: app.sysAuthHeader,
		}
		resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, request, testTimeout)
		require.Nil(err, err)
		require.Nil(secErr, secErr)
		require.Nil(sections)
		log.Println(string(resp.Data))
		require.Equal(http.StatusOK, resp.StatusCode)
		require.Equal(coreutils.ApplicationJSON, resp.ContentType)
		// убедимся, что команда действительно отработала и нотификации отправились
		<-check
		<-check

		// убедимся, что CUD'ы проверились
		<-cudsCheck
	})

	t.Run("500 internal server error command exec error", func(t *testing.T) {
		request := ibus.Request{
			Body:     []byte(`{"args":{"Text":"fire error"},"unloggedArgs":{"Password":"pass"}}`),
			AppQName: istructs.AppQName_untill_airs_bp.String(),
			WSID:     1,
			Resource: "c.sys.Test",
			Header:   app.sysAuthHeader,
		}
		resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, request, testTimeout)
		require.Nil(err, err)
		require.Nil(secErr, secErr)
		require.Nil(sections)
		require.Equal(http.StatusInternalServerError, resp.StatusCode)
		require.Equal(coreutils.ApplicationJSON, resp.ContentType)
		require.Equal(`{"sys.Error":{"HTTPStatus":500,"Message":"fire error"}}`, string(resp.Data))
		require.Contains(string(resp.Data), "fire error")
		log.Println(string(resp.Data))
	})
}

func sendCUD(t *testing.T, wsid istructs.WSID, app testApp) map[string]interface{} {
	require := require.New(t)
	req := ibus.Request{
		WSID:     int64(wsid),
		AppQName: istructs.AppQName_untill_airs_bp.String(),
		Resource: "c.sys.CUD",
		Body: []byte(`{"cuds":[
			{"fields":{"sys.ID":1,"sys.QName":"test.TestCDoc"}},
			{"fields":{"sys.ID":2,"sys.QName":"test.TestWDoc"}},
			{"fields":{"sys.ID":3,"sys.QName":"test.TestCRecord","sys.ParentID":1,"sys.Container":"TestCRecord"}}
		]}`),
		Header: app.sysAuthHeader,
	}
	resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, req, testTimeout)
	require.Nil(err, err)
	require.Nil(secErr, secErr)
	require.Nil(sections)
	require.Equal(http.StatusOK, resp.StatusCode)
	respData := map[string]interface{}{}
	require.Nil(json.Unmarshal(resp.Data, &respData))
	return respData
}

func TestRecovery(t *testing.T) {
	require := require.New(t)
	app := setUp(t)
	defer tearDown(app)

	cudQName := istructs.NewQName(istructs.SysPackage, "CUD")
	cmdCUD := istructsmem.NewCommandFunction(cudQName, istructs.NullQName, istructs.NullQName, istructs.NullQName, istructsmem.NullCommandExec)
	app.cfg.Resources.Add(cmdCUD)

	_ = app.cfg.Schemas.Add(testCRecord, istructs.SchemaKind_CRecord)
	_ = app.cfg.Schemas.Add(testCDoc, istructs.SchemaKind_CDoc).AddContainer("TestCRecord", testCRecord, 0, 1)
	_ = app.cfg.Schemas.Add(testWDoc, istructs.SchemaKind_WDoc)

	respData := sendCUD(t, 1, app)
	require.Equal(1, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID), istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.NewRecordID(istructs.FirstBaseRecordID), istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID)+1, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	restartCmdProc(&app)
	respData = sendCUD(t, 1, app)
	require.Equal(2, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID)+2, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.NewRecordID(istructs.FirstBaseRecordID)+1, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID)+3, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	restartCmdProc(&app)
	respData = sendCUD(t, 2, app)
	require.Equal(1, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID), istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.NewRecordID(istructs.FirstBaseRecordID), istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID)+1, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

	restartCmdProc(&app)
	respData = sendCUD(t, 1, app)
	require.Equal(3, int(respData["CurrentWLogOffset"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID)+4, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["1"].(float64)))
	require.Equal(istructs.NewRecordID(istructs.FirstBaseRecordID)+2, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["2"].(float64)))
	require.Equal(istructs.NewCDocCRecordID(istructs.FirstBaseRecordID)+5, istructs.RecordID(respData["NewIDs"].(map[string]interface{})["3"].(float64)))

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
	app := setUp(t)
	defer tearDown(app)

	cudQName := istructs.NewQName(istructs.SysPackage, "CUD")
	cmdCUD := istructsmem.NewCommandFunction(cudQName, istructs.NullQName, istructs.NullQName, istructs.NullQName, istructsmem.NullCommandExec)
	app.cfg.Resources.Add(cmdCUD)

	testQName := istructs.NewQName("test", "test")
	_ = app.cfg.Schemas.Add(testQName, istructs.SchemaKind_CDoc).AddField("IntFld", istructs.DataKind_int32, false)

	// insert
	req := ibus.Request{
		WSID:     1,
		AppQName: istructs.AppQName_untill_airs_bp.String(),
		Resource: "c.sys.CUD",
		Body:     []byte(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"test.test"}}]}`),
		Header:   app.sysAuthHeader,
	}
	resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, req, testTimeout)
	require.Nil(err, err)
	require.Nil(secErr, secErr)
	require.Nil(sections)
	require.Equal(http.StatusOK, resp.StatusCode)
	require.Equal(coreutils.ApplicationJSON, resp.ContentType)
	m := map[string]interface{}{}
	require.NoError(json.Unmarshal(resp.Data, &m))

	t.Run("update", func(t *testing.T) {
		id := int64(m["NewIDs"].(map[string]interface{})["1"].(float64))
		req.Body = []byte(fmt.Sprintf(`{"cuds":[{"sys.ID":%d,"fields":{"sys.QName":"test.test", "IntFld": 42}}]}`, id))
		resp, sections, secErr, err = app.bus.SendRequest2(app.ctx, req, testTimeout)
		require.Nil(err, err)
		require.Nil(secErr, secErr)
		require.Nil(sections)
		require.Equal(http.StatusOK, resp.StatusCode)
		require.Equal(coreutils.ApplicationJSON, resp.ContentType)
	})

	t.Run("404 not found on update unexisting", func(t *testing.T) {
		req.Body = []byte(`{"cuds":[{"sys.ID":2,"fields":{"sys.QName":"test.test", "IntFld": 42}}]}`)
		resp, sections, secErr, err = app.bus.SendRequest2(app.ctx, req, testTimeout)
		require.Nil(err, err)
		require.Nil(secErr, secErr)
		require.Nil(sections)
		require.Equal(http.StatusNotFound, resp.StatusCode)
		require.Equal(coreutils.ApplicationJSON, resp.ContentType)
	})
}

func Test400BadRequestOnCUDErrors(t *testing.T) {
	require := require.New(t)
	app := setUp(t)
	defer tearDown(app)

	cudQName := istructs.NewQName(istructs.SysPackage, "CUD")
	cmdCUD := istructsmem.NewCommandFunction(cudQName, istructs.NullQName, istructs.NullQName, istructs.NullQName, istructsmem.NullCommandExec)
	app.cfg.Resources.Add(cmdCUD)

	testQName := istructs.NewQName("test", "test")
	_ = app.cfg.Schemas.Add(testQName, istructs.SchemaKind_CDoc)

	cases := []struct {
		desc                string
		bodyAdd             string
		expectedMessageLike string
	}{
		{"not an object", `"cuds":42`, `'cuds' must be an array of objects`},
		{`element is not an object`, `"cuds":[42]`, `cuds[0]: not an object`},
		{`missing fields`, `"cuds":[{}]`, `cuds[0]: "fields" missing`},
		{`fields is not an object`, `"cuds":[{"fields":42}]`, `cuds[0]: field 'fields' must be an object`},
		{`fields: sys.ID missing`, `"cuds":[{"fields":{"sys.QName":"test.Test"}}]`, `cuds[0]: "sys.ID" missing`},
		{`fields: sys.ID is not a number (create)`, `"cuds":[{"sys.ID":"wrong","fields":{"sys.QName":"test.Test"}}]`, `cuds[0]: field 'sys.ID' must be an int64`},
		{`fields: sys.ID is not a number (update)`, `"cuds":[{"fields":{"sys.ID":"wrong","sys.QName":"test.Test"}}]`, `cuds[0]: field 'sys.ID' must be an int64`},
		{`fields: wrong qName`, `"cuds":[{"fields":{"sys.ID":1,"sys.QName":"wrong"}},{"fields":{"sys.ID":1,"sys.QName":"test.Test"}}]`, `invalid string representation of qualified name: wrong`},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			req := ibus.Request{
				WSID:     1,
				AppQName: istructs.AppQName_untill_airs_bp.String(),
				Resource: "c.sys.CUD",
				Body:     []byte("{" + c.bodyAdd + "}"),
				Header:   app.sysAuthHeader,
			}
			resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, req, testTimeout)
			require.Nil(err, err)
			require.Nil(secErr, secErr)
			require.Nil(sections)
			require.Equal(http.StatusBadRequest, resp.StatusCode, c.desc)
			require.Equal(coreutils.ApplicationJSON, resp.ContentType, c.desc)
			require.Contains(string(resp.Data), jsonEscape(c.expectedMessageLike), c.desc)
			require.Contains(string(resp.Data), `"HTTPStatus":400`, c.desc)
		})
	}
}

func Test400BadRequests(t *testing.T) {
	require := require.New(t)
	app := setUp(t)
	defer tearDown(app)

	testCmdQNameParams := istructs.NewQName(istructs.SysPackage, "TestParams")
	testCmdParamsScheme := app.cfg.Schemas.Add(testCmdQNameParams, istructs.SchemaKind_Object)
	testCmdParamsScheme.AddField("Text", istructs.DataKind_string, true)

	testCmdQNameParamsUnlogged := istructs.NewQName(istructs.SysPackage, "TestParamsUnlogged")
	testCmdParamsUnloggedScheme := app.cfg.Schemas.Add(testCmdQNameParamsUnlogged, istructs.SchemaKind_Object)
	testCmdParamsUnloggedScheme.AddField("Password", istructs.DataKind_string, true)

	testCmdQName := istructs.NewQName(istructs.SysPackage, "Test")
	qryGreeter := istructsmem.NewCommandFunction(testCmdQName, testCmdQNameParams, testCmdQNameParamsUnlogged, istructs.NullQName, func(cf istructs.ICommandFunction, args istructs.ExecCommandArgs) (err error) {
		_ = args.ArgumentObject.AsString("Text")
		_ = args.ArgumentUnloggedObject.AsString("Password")
		return nil
	})
	app.cfg.Resources.Add(qryGreeter)

	baseReq := ibus.Request{
		WSID:     1,
		AppQName: istructs.AppQName_untill_airs_bp.String(),
		Resource: "c.sys.Test",
		Body:     []byte(`{"args":{"Text":"hello"},"unloggedArgs":{"Password":"123"}}`),
		Header:   app.sysAuthHeader,
	}

	cases := []struct {
		desc string
		ibus.Request
		expectedMessageLike string
	}{
		{"unknown app", ibus.Request{AppQName: "untill/unknown"}, "application not found"}, // TODO: simplify
		{"bad request body", ibus.Request{Body: []byte("{wrong")}, "failed to unmarshal request body: invalid character 'w' looking for beginning of object key string"},
		{"unknown func", ibus.Request{Resource: "c.sys.Unknown"}, "unknown function"},
		{"args: field of wrong type provided", ibus.Request{Body: []byte(`{"args":{"Text":42}}`)}, "field type mismatch"},
		{"args: not an object", ibus.Request{Body: []byte(`{"args":42}`)}, `"args" field must be an object`},
		{"args: missing at all with a required field", ibus.Request{Body: []byte(`{}`)}, ""},
		{"unloggedArgs: not an object", ibus.Request{Body: []byte(`{"unloggedArgs":42,"args":{"Text":"txt"}}`)}, `"unloggedArgs" field must be an object`},
		{"unloggedArgs: field of wrong type provided", ibus.Request{Body: []byte(`{"unloggedArgs":{"Password":42},"args":{"Text":"txt"}}`)}, "field type mismatch"},
		{"unloggedArgs: missing required field of unlogged args, no unlogged args at all", ibus.Request{Body: []byte(`{"args":{"Text":"txt"}}`)}, ""},
		{"cuds: not an object", ibus.Request{Body: []byte(`{"args":{"Text":"hello"},"unloggedArgs":{"Password":"123"},"cuds":42}`)}, `field 'cuds' must be an array of objects`},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			req := baseReq
			req.Body = make([]byte, len(baseReq.Body))
			copy(req.Body, baseReq.Body)
			if len(c.AppQName) > 0 {
				req.AppQName = c.AppQName
			}
			if len(c.Body) > 0 {
				req.Body = make([]byte, len(c.Body))
				copy(req.Body, c.Body)
			}
			if len(c.Resource) > 0 {
				req.Resource = c.Resource
			}
			resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, req, testTimeout)
			require.Nil(err, err)
			require.Nil(secErr, secErr)
			require.Nil(sections)
			require.Equal(http.StatusBadRequest, resp.StatusCode)
			require.Equal(coreutils.ApplicationJSON, resp.ContentType)
			require.Contains(string(resp.Data), jsonEscape(c.expectedMessageLike))
			require.Contains(string(resp.Data), `"HTTPStatus":400`, c.desc)
		})
	}
}

func TestAuthnz(t *testing.T) {
	require := require.New(t)
	app := setUp(t)
	defer tearDown(app)

	qNameAllowedCmd := istructs.NewQName(istructs.SysPackage, "TestAllowedCmd")
	qNameDeniedCmd := istructs.NewQName(istructs.SysPackage, "TestDeniedCmd")       // the same in core/iauthnzimpl
	qNameTestDeniedCDoc := istructs.NewQName(istructs.SysPackage, "TestDeniedCDoc") // the same in core/iauthnzimpl
	app.cfg.Resources.Add(istructsmem.NewCommandFunction(qNameDeniedCmd, istructs.NullQName, istructs.NullQName, istructs.NullQName, istructsmem.NullCommandExec))
	app.cfg.Resources.Add(istructsmem.NewCommandFunction(qNameAllowedCmd, istructs.NullQName, istructs.NullQName, istructs.NullQName, istructsmem.NullCommandExec))
	app.cfg.Schemas.Add(qNameTestDeniedCDoc, istructs.SchemaKind_CDoc)

	pp := payloads.PrincipalPayload{
		Login:       "testlogin",
		SubjectKind: istructs.SubjectKind_User,
		ProfileWSID: 1,
	}
	token, err := app.appTokens.IssueToken(10*time.Second, &pp)
	require.NoError(err)

	type testCase struct {
		desc               string
		req                ibus.Request
		expectedStatusCode int
	}
	cases := []testCase{
		{
			desc: "403 on cmd EXECUTE forbidden", req: ibus.Request{
				Body:     []byte(`{}`),
				AppQName: istructs.AppQName_untill_airs_bp.String(),
				WSID:     1,
				Resource: "c.sys.TestDeniedCmd",
				Header:   getAuthHeader(token),
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			desc: "403 on INSERT CUD forbidden", req: ibus.Request{
				Body:     []byte(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"sys.TestDeniedCDoc"}}]}`),
				AppQName: istructs.AppQName_untill_airs_bp.String(),
				WSID:     1,
				Resource: "c.sys.TestAllowedCmd",
				Header:   getAuthHeader(token),
			},
			expectedStatusCode: http.StatusForbidden,
		},
		{
			desc: "403 if no token for a func that requires authentication", req: ibus.Request{
				Body:     []byte(`{}`),
				AppQName: istructs.AppQName_untill_airs_bp.String(),
				WSID:     1,
				Resource: "c.sys.TestAllowedCmd",
			},
			expectedStatusCode: http.StatusForbidden,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, c.req, testTimeout)
			require.Nil(err, err)
			require.Nil(secErr, secErr)
			require.Nil(sections)
			log.Println(string(resp.Data))
			require.Equal(c.expectedStatusCode, resp.StatusCode)
		})
	}
}

func getAuthHeader(token string) map[string][]string {
	return map[string][]string{
		coreutils.Authorization: {
			"Bearer " + token,
		},
	}
}

func TestBasicUsage_QNameJSONFunc(t *testing.T) {
	require := require.New(t)
	app := setUp(t)
	defer tearDown(app)

	ch := make(chan interface{})
	testCmdQName := istructs.NewQName(istructs.SysPackage, "Test")
	testExec := func(cf istructs.ICommandFunction, args istructs.ExecCommandArgs) (err error) {
		require.Equal("custom content", args.ArgumentObject.AsString(Field_JSONSchemaBody))
		close(ch)
		return
	}
	testCmd := istructsmem.NewCommandFunction(testCmdQName, istructs.QNameJSON, istructs.NullQName, istructs.NullQName, testExec)

	app.cfg.Resources.Add(testCmd)

	request := ibus.Request{
		Body:     []byte(`custom content`),
		AppQName: istructs.AppQName_untill_airs_bp.String(),
		WSID:     1,
		Resource: "c.sys.Test",
		Header:   app.sysAuthHeader,
	}
	resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, request, testTimeout)
	require.Nil(err, err)
	require.Nil(secErr)
	require.Nil(sections)
	require.Equal(http.StatusOK, resp.StatusCode)
	require.Equal(coreutils.ApplicationJSON, resp.ContentType)
	<-ch
}

func TestRateLimit(t *testing.T) {
	require := require.New(t)
	qName := istructs.NewQName(istructs.SysPackage, "MyCmd")
	app := setUp(t, func(cfg *istructsmem.AppConfigType) {
		cfg.Resources.Add(istructsmem.NewCommandFunction(
			qName,
			cfg.Schemas.Add(istructs.NewQName(istructs.SysPackage, "Params"), istructs.SchemaKind_Object).QName(),
			istructs.NullQName,
			istructs.NullQName,
			istructsmem.NullCommandExec,
		))

		cfg.FunctionRateLimits.AddWorkspaceLimit(qName, istructs.RateLimit{
			Period:                time.Minute,
			MaxAllowedPerDuration: 2,
		})
	})
	defer tearDown(app)

	request := ibus.Request{
		Body:     []byte(`{"args":{}}`),
		AppQName: istructs.AppQName_untill_airs_bp.String(),
		WSID:     1,
		Resource: "c.sys.MyCmd",
		Header:   app.sysAuthHeader,
	}

	// first 2 calls are ok
	for i := 0; i < 2; i++ {
		resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, request, testTimeout)
		require.Nil(err, err)
		require.Nil(secErr, secErr)
		require.Nil(sections)
		require.Equal(http.StatusOK, resp.StatusCode)
	}

	// 3rd exceeds rate limits
	resp, sections, secErr, err := app.bus.SendRequest2(app.ctx, request, testTimeout)
	require.Nil(err, err)
	require.Nil(secErr, secErr)
	require.Nil(sections)
	require.Equal(http.StatusTooManyRequests, resp.StatusCode)
}

type testApp struct {
	ctx            context.Context
	cfg            *istructsmem.AppConfigType
	bus            ibus.IBus
	cancel         context.CancelFunc
	done           chan struct{}
	cmdProcService pipeline.IService
	serviceChannel CommandChannel
	n10nBroker     in10n.IN10nBroker
	appTokens      istructs.IAppTokens
	sysAuthHeader  map[string][]string
}

func tearDown(app testApp) {
	// завершим command processor IService
	app.cancel()
	<-app.done
}

// simulate airs-bp3 behaviour
func replyBadRequest(bus ibus.IBus, sender interface{}, message string) {
	res := coreutils.NewHTTPErrorf(http.StatusBadRequest, message)
	bus.SendResponse(sender, ibus.Response{
		ContentType: coreutils.ApplicationJSON,
		StatusCode:  http.StatusBadRequest,
		Data:        []byte(res.ToJSON()),
	})
}

func setUp(t *testing.T, cfgFuncs ...func(*istructsmem.AppConfigType)) testApp {
	if coreutils.IsDebug() {
		testTimeout = time.Hour
	}
	// command processor - это IService, работающий через CommandChannel(iprocbus.ServiceChannel). Подготовим этот channel
	serviceChannel := make(CommandChannel)
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())

	cfgs := istructsmem.AppConfigsType{}
	asf := istorage.ProvideMem()
	appStorageProvider := istorageimpl.Provide(asf)
	// конфиг приложения airs-bp
	cfg := cfgs.AddConfig(istructs.AppQName_untill_airs_bp)
	for _, cfgFunc := range cfgFuncs {
		cfgFunc(cfg)
	}

	appStructsProvider, err := istructsmem.Provide(cfgs, iratesce.TestBucketsFactory,
		payloads.ProvideIAppTokensFactory(itokensjwt.TestTokensJWT()), appStorageProvider)
	require.Nil(t, err, err)

	// command processor работает через ibus.SendResponse -> нам нужна реализация ibus
	var bus ibus.IBus
	bus = ibusmem.Provide(func(ctx context.Context, sender interface{}, request ibus.Request) {
		// сымитируем работу airs-bp3 при приеме запроса-команды
		cmdQName, err := istructs.ParseQName(request.Resource[2:])
		require.Nil(t, err, err)
		appQName, err := istructs.ParseAppQName(request.AppQName)
		require.Nil(t, err, err)
		as, err := appStructsProvider.AppStructs(appQName)
		if err != nil {
			replyBadRequest(bus, sender, err.Error())
			return
		}
		resource := as.Resources().QueryResource(cmdQName)
		if resource.Kind() == istructs.ResourceKind_null {
			replyBadRequest(bus, sender, "unknown function")
			return
		}
		token := ""
		if authHeaders, ok := request.Header[coreutils.Authorization]; ok {
			token = strings.TrimPrefix(authHeaders[0], "Bearer ")
		}
		icm := NewCommandMessage(ctx, request.Body, appQName, istructs.WSID(request.WSID), sender, 1, resource, token, "")
		serviceChannel <- icm
	})
	n10nBroker, err := in10nmem.Provide(in10n.Quotas{
		Channels:               1000,
		ChannelsPerSubject:     10,
		Subsciptions:           1000,
		SubsciptionsPerSubject: 10,
	})
	require.Nil(t, err)
	ProvideJSONFuncParamsSchema(cfg)

	tokens := itokensjwt.ProvideITokens(itokensjwt.SecretKeyExample, time.Now)
	appTokens := payloads.ProvideIAppTokensFactory(tokens).New(istructs.AppQName_untill_airs_bp)
	systemToken, err := payloads.GetSystemPrincipalTokenApp(appTokens)
	require.NoError(t, err)
	cmdProcessorFactory := ProvideServiceFactory(bus, appStructsProvider, time.Now, func(ctx context.Context, partitionID istructs.PartitionID) pipeline.ISyncOperator {
		return &pipeline.NOOP{}
	}, n10nBroker, imetrics.Provide(), "hvm", iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter), iauthnzimpl.NewDefaultAuthorizer(), isecretsimpl.ProvideSecretReader())
	cmdProcService := cmdProcessorFactory(serviceChannel, 1)

	go func() {
		cmdProcService.Run(ctx)
		close(done)
	}()

	// skip checking workspace initialization
	AddDummyWS(1)
	AddDummyWS(2)

	return testApp{
		cfg:            cfg,
		bus:            bus,
		cancel:         cancel,
		ctx:            ctx,
		done:           done,
		cmdProcService: cmdProcService,
		serviceChannel: serviceChannel,
		n10nBroker:     n10nBroker,
		appTokens:      appTokens,
		sysAuthHeader:  getAuthHeader(systemToken),
	}
}

func jsonEscape(i string) string {
	b, err := json.Marshal(i)
	if err != nil {
		panic(err)
	}
	s := string(b)
	return s[1 : len(s)-1]
}
