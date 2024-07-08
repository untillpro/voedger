/*
 * Copyright (c) 2024-present unTill Software Development Group B. V.
 * @author Michael Saigachenko
 */
package teststate

import (
	"context"
	"embed"
	"fmt"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istorage/mem"
	wsdescutil "github.com/voedger/voedger/pkg/utils/testwsdesc"

	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	"github.com/voedger/voedger/pkg/parser"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

type TestState struct {
	state.IState

	ctx                   context.Context
	appStructs            istructs.IAppStructs
	appDef                appdef.IAppDef
	cud                   istructs.ICUD
	event                 istructs.IPLogEvent
	plogGen               istructs.IIDGenerator
	wsOffsets             map[istructs.WSID]istructs.Offset
	plogOffset            istructs.Offset
	secretReader          isecrets.ISecretReader
	httpHandler           HttpHandlerFunc
	federationCmdHandler  state.FederationCommandHandler
	federationBlobHandler state.FederationBlobHandler
	uniquesHandler        state.UniquesHandler
	principals            []iauthnz.Principal
	token                 string
	queryWsid             istructs.WSID
	queryName             appdef.FullQName
	processorKind         int
	readObjects           []istructs.IObject
	queryObject           istructs.IObject

	t             *testing.T
	extensionFunc func()
	funcRunner    *sync.Once
	commandWSID   istructs.WSID

	// recordItems to store records
	recordItems []recordItem
	// argumentObject to pass to argument
	argumentType   appdef.FullQName
	argumentObject map[string]any
}

func NewTestState(processorKind int, packagePath string, createWorkspaces ...TestWorkspace) ITestState {
	ts := &TestState{}
	ts.ctx = context.Background()
	ts.processorKind = processorKind
	ts.secretReader = &secretReader{secrets: make(map[string][]byte)}
	ts.buildAppDef(packagePath, "..", createWorkspaces...)
	ts.buildState(processorKind)
	return ts
}

type secretReader struct {
	secrets map[string][]byte
}

func (s *secretReader) ReadSecret(name string) (bb []byte, err error) {
	if bb, ok := s.secrets[name]; ok {
		return bb, nil
	}
	return nil, fmt.Errorf("secret not found: %s", name)
}

func (ts *TestState) WSID() istructs.WSID {
	if ts.processorKind == ProcKind_QueryProcessor {
		return ts.queryWsid
	}
	if ts.event != nil {
		return ts.event.Workspace()
	}

	return ts.commandWSID
}

func (ts *TestState) GetReadObjects() []istructs.IObject {
	return ts.readObjects
}

func (ts *TestState) Arg() istructs.IObject {
	if ts.queryObject != nil {
		return ts.queryObject
	}
	if ts.event == nil {
		panic("no current event")
	}
	return ts.event.ArgumentObject()
}

func (ts *TestState) ResultBuilder() istructs.IObjectBuilder {
	if ts.event == nil {
		panic("no current event")
	}
	qname := ts.event.QName()
	command := ts.appDef.Command(qname)
	if command == nil {
		panic(fmt.Sprintf("%v is not a command", qname))
	}
	return ts.appStructs.ObjectBuilder(command.Result().QName())
}

func (ts *TestState) Request(timeout time.Duration, method, url string, body io.Reader, headers map[string]string) (statusCode int, resBody []byte, resHeaders map[string][]string, err error) {
	if ts.httpHandler == nil {
		panic("http handler not set")
	}
	req := HttpRequest{
		Timeout: timeout,
		Method:  method,
		URL:     url,
		Body:    body,
		Headers: headers,
	}
	resp, err := ts.httpHandler(req)
	if err != nil {
		return 0, nil, nil, err
	}
	return resp.Status, resp.Body, resp.Headers, nil
}

func (ts *TestState) PutQuery(wsid istructs.WSID, name appdef.FullQName, argb QueryArgBuilderCallback) {
	ts.queryWsid = wsid
	ts.queryName = name

	if argb != nil {
		localPkgName := ts.appDef.PackageLocalName(ts.queryName.PkgPath())
		query := ts.appDef.Query(appdef.NewQName(localPkgName, ts.queryName.Entity()))
		if query == nil {
			panic(fmt.Sprintf("query not found: %v", ts.queryName))
		}
		ab := ts.appStructs.ObjectBuilder(query.Param().QName())
		argb(ab)
		qo, err := ab.Build()
		if err != nil {
			panic(err)
		}
		ts.queryObject = qo
	}
}

func (ts *TestState) PutRequestSubject(principals []iauthnz.Principal, token string) {
	ts.principals = principals
	ts.token = token
}

func (ts *TestState) PutFederationCmdHandler(emu state.FederationCommandHandler) {
	ts.federationCmdHandler = emu
}

func (ts *TestState) PutFederationBlobHandler(emu state.FederationBlobHandler) {
	ts.federationBlobHandler = emu
}

func (ts *TestState) PutUniquesHandler(emu state.UniquesHandler) {
	ts.uniquesHandler = emu
}

func (ts *TestState) emulateUniquesHandler(entity appdef.QName, wsid istructs.WSID, data map[string]interface{}) (istructs.RecordID, error) {
	if ts.uniquesHandler == nil {
		panic("uniques handler not set")
	}
	return ts.uniquesHandler(entity, wsid, data)
}

func (ts *TestState) emulateFederationCmd(owner, appname string, wsid istructs.WSID, command appdef.QName, body string) (statusCode int, newIDs map[string]int64, result string, err error) {
	if ts.federationCmdHandler == nil {
		panic("federation command handler not set")
	}
	return ts.federationCmdHandler(owner, appname, wsid, command, body)
}

func (ts *TestState) emulateFederationBlob(owner, appname string, wsid istructs.WSID, blobId int64) ([]byte, error) {
	if ts.federationBlobHandler == nil {
		panic("federation blob handler not set")
	}
	return ts.federationBlobHandler(owner, appname, wsid, blobId)
}

func (ts *TestState) buildState(processorKind int) {

	appFunc := func() istructs.IAppStructs { return ts.appStructs }
	eventFunc := func() istructs.IPLogEvent { return ts.event }
	partitionIDFunc := func() istructs.PartitionID { return TestPartition }
	cudFunc := func() istructs.ICUD { return ts.cud }
	commandPrepareArgs := func() istructs.CommandPrepareArgs {
		return istructs.CommandPrepareArgs{
			PrepareArgs: istructs.PrepareArgs{
				Workpiece:      nil,
				ArgumentObject: ts.Arg(),
				WSID:           ts.WSID(),
				Workspace:      nil,
			},
			ArgumentUnloggedObject: nil,
		}
	}
	argFunc := func() istructs.IObject { return ts.Arg() }
	unloggedArgFunc := func() istructs.IObject { return nil }
	wlogOffsetFunc := func() istructs.Offset { return ts.event.WLogOffset() }
	wsidFunc := func() istructs.WSID {
		return ts.WSID()
	}
	resultBuilderFunc := func() istructs.IObjectBuilder {
		return ts.ResultBuilder()
	}
	principalsFunc := func() []iauthnz.Principal {
		return ts.principals
	}
	tokenFunc := func() string {
		return ts.token
	}
	execQueryArgsFunc := func() istructs.PrepareArgs {
		return istructs.PrepareArgs{
			Workpiece:      nil,
			ArgumentObject: ts.Arg(),
			WSID:           ts.WSID(),
			Workspace:      nil,
		}
	}
	qryResultBuilderFunc := func() istructs.IObjectBuilder {
		localPkgName := ts.appDef.PackageLocalName(ts.queryName.PkgPath())
		query := ts.appDef.Query(appdef.NewQName(localPkgName, ts.queryName.Entity()))
		if query == nil {
			panic(fmt.Sprintf("query not found: %v", ts.queryName))
		}
		return ts.appStructs.ObjectBuilder(query.Result().QName())
	}
	execQueryCallback := func() istructs.ExecQueryCallback {
		return func(o istructs.IObject) error {
			ts.readObjects = append(ts.readObjects, o)
			return nil
		}
	}

	switch processorKind {
	case ProcKind_Actualizer:
		ts.IState = state.ProvideAsyncActualizerStateFactory()(ts.ctx, appFunc, partitionIDFunc, wsidFunc, nil, ts.secretReader, eventFunc, nil, nil,
			IntentsLimit, BundlesLimit, state.WithCustomHttpClient(ts), state.WithFedearationCommandHandler(ts.emulateFederationCmd), state.WithUniquesHandler(ts.emulateUniquesHandler), state.WithFederationBlobHandler(ts.emulateFederationBlob))
	case ProcKind_CommandProcessor:
		ts.IState = state.ProvideCommandProcessorStateFactory()(ts.ctx, appFunc, partitionIDFunc, wsidFunc, ts.secretReader, cudFunc, principalsFunc, tokenFunc,
			IntentsLimit, resultBuilderFunc, commandPrepareArgs, argFunc, unloggedArgFunc, wlogOffsetFunc, state.WithUniquesHandler(ts.emulateUniquesHandler))
	case ProcKind_QueryProcessor:
		ts.IState = state.ProvideQueryProcessorStateFactory()(ts.ctx, appFunc, partitionIDFunc, wsidFunc, ts.secretReader, principalsFunc, tokenFunc, nil,
			execQueryArgsFunc, argFunc, qryResultBuilderFunc, nil, execQueryCallback,
			state.WithCustomHttpClient(ts), state.WithFedearationCommandHandler(ts.emulateFederationCmd), state.WithUniquesHandler(ts.emulateUniquesHandler), state.WithFederationBlobHandler(ts.emulateFederationBlob))
	}
}

//go:embed testsys/*.sql
var fsTestSys embed.FS

func (ts *TestState) buildAppDef(packagePath string, packageDir string, createWorkspaces ...TestWorkspace) {

	absPath, err := filepath.Abs(packageDir)
	if err != nil {
		panic(err)
	}

	pkgAst, err := parser.ParsePackageDir(packagePath, coreutils.NewPathReader(absPath), "")
	if err != nil {
		panic(err)
	}
	sysPackageAST, err := parser.ParsePackageDir(appdef.SysPackage, fsTestSys, "testsys")
	if err != nil {
		panic(err)
	}

	app, err := parser.FindApplication(pkgAst)
	if err != nil {
		panic(err)
	}

	packagesAST := []*parser.PackageSchemaAST{pkgAst, sysPackageAST}

	var dummyAppPkgAST *parser.PackageSchemaAST
	if app == nil {
		PackageName = "tstpkg"
		dummyAppFileAST, err := parser.ParseFile("dummy.sql", fmt.Sprintf(`
			IMPORT SCHEMA '%s' AS %s;
			APPLICATION test(
				USE %s;
			);
		`, packagePath, PackageName, PackageName))
		if err != nil {
			panic(err)
		}
		dummyAppPkgAST, err = parser.BuildPackageSchema(packagePath+"_app", []*parser.FileSchemaAST{dummyAppFileAST})
		if err != nil {
			panic(err)
		}
		packagesAST = append(packagesAST, dummyAppPkgAST)
	} else {
		PackageName = parser.GetPackageName(packagePath)
	}

	appSchema, err := parser.BuildAppSchema(packagesAST)
	if err != nil {
		panic(err)
	}

	// TODO: obtain app name from packages
	// appName := appSchema.AppQName()

	appName := istructs.AppQName_test1_app1

	adb := appdef.New()
	err = parser.BuildAppDefs(appSchema, adb)
	if err != nil {
		panic(err)
	}

	adf, err := adb.Build()
	if err != nil {
		panic(err)
	}

	ts.appDef = adf

	cfgs := make(istructsmem.AppConfigsType, 1)
	cfg := cfgs.AddBuiltInAppConfig(appName, adb)
	cfg.SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)
	ts.appDef.Extensions(func(i appdef.IExtension) {
		if i.QName().Pkg() == PackageName {
			if proj, ok := i.(appdef.IProjector); ok {
				if proj.Sync() {
					cfg.AddSyncProjectors(istructs.Projector{Name: i.QName()})
				} else {
					cfg.AddAsyncProjectors(istructs.Projector{Name: i.QName()})
				}
			} else if cmd, ok := i.(appdef.ICommand); ok {
				cfg.Resources.Add(istructsmem.NewCommandFunction(cmd.QName(), istructsmem.NullCommandExec))
			} else if q, ok := i.(appdef.IQuery); ok {
				cfg.Resources.Add(istructsmem.NewCommandFunction(q.QName(), istructsmem.NullCommandExec))
			}
		}
	})

	asf := mem.Provide()
	storageProvider := istorageimpl.Provide(asf)
	prov := istructsmem.Provide(
		cfgs,
		iratesce.TestBucketsFactory,
		payloads.ProvideIAppTokensFactory(itokensjwt.TestTokensJWT()),
		storageProvider)
	structs, err := prov.BuiltIn(appName)
	if err != nil {
		panic(err)
	}
	ts.appStructs = structs
	ts.plogGen = istructsmem.NewIDGenerator()
	ts.wsOffsets = make(map[istructs.WSID]istructs.Offset)

	for _, ws := range createWorkspaces {
		err = wsdescutil.CreateCDocWorkspaceDescriptorStub(ts.appStructs, TestPartition, ws.WSID, appdef.NewQName(PackageName, ws.WorkspaceDescriptor), ts.nextPLogOffs(), ts.nextWSOffs(ws.WSID))
		if err != nil {
			panic(err)
		}
	}
}

func (ts *TestState) nextPLogOffs() istructs.Offset {
	ts.plogOffset += 1
	return ts.plogOffset
}

func (ts *TestState) nextWSOffs(ws istructs.WSID) istructs.Offset {
	offs, ok := ts.wsOffsets[ws]
	if !ok {
		offs = istructs.Offset(0)
	}
	offs += 1
	ts.wsOffsets[ws] = offs
	return offs
}

func (ts *TestState) PutHttpHandler(handler HttpHandlerFunc) {
	ts.httpHandler = handler

}

func (ts *TestState) PutRecords(wsid istructs.WSID, cb NewRecordsCallback) (wLogOffs istructs.Offset, newRecordIds []istructs.RecordID) {
	return ts.PutEvent(wsid, appdef.NewFullQName(istructs.QNameCommandCUD.Pkg(), istructs.QNameCommandCUD.Entity()), func(argBuilder istructs.IObjectBuilder, cudBuilder istructs.ICUD) {
		cb(cudBuilder)
	})
}

func (ts *TestState) GetRecord(wsid istructs.WSID, id istructs.RecordID) istructs.IRecord {
	var rec istructs.IRecord
	rec, err := ts.appStructs.Records().Get(wsid, false, id)
	if err != nil {
		panic(err)
	}
	return rec
}

func (ts *TestState) PutEvent(wsid istructs.WSID, name appdef.FullQName, cb NewEventCallback) (wLogOffs istructs.Offset, newRecordIds []istructs.RecordID) {
	var localPkgName string
	if name.PkgPath() == appdef.SysPackage {
		localPkgName = name.PkgPath()
	} else {
		localPkgName = ts.appDef.PackageLocalName(name.PkgPath())
	}
	wLogOffs = ts.nextWSOffs(wsid)
	reb := ts.appStructs.Events().GetNewRawEventBuilder(istructs.NewRawEventBuilderParams{
		GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
			Workspace:         wsid,
			HandlingPartition: TestPartition,
			QName:             appdef.NewQName(localPkgName, name.Entity()),
			WLogOffset:        wLogOffs,
			PLogOffset:        ts.nextPLogOffs(),
		},
	})
	if cb != nil {
		ts.cud = reb.CUDBuilder()
		cb(reb.ArgumentObjectBuilder(), ts.cud)
	}
	rawEvent, err := reb.BuildRawEvent()
	if err != nil {
		panic(err)
	}
	event, err := ts.appStructs.Events().PutPlog(rawEvent, nil, ts.plogGen)
	if err != nil {
		panic(err)
	}

	err = ts.appStructs.Events().PutWlog(event)
	if err != nil {
		panic(err)
	}

	newRecordIds = make([]istructs.RecordID, 0)
	err = ts.appStructs.Records().Apply2(event, func(r istructs.IRecord) {
		newRecordIds = append(newRecordIds, r.ID())
	})

	if err != nil {
		panic(err)
	}

	ts.event = event
	return wLogOffs, newRecordIds
}

func (ts *TestState) PutView(wsid istructs.WSID, entity appdef.FullQName, callback ViewValueCallback) {
	localPkgName := ts.appDef.PackageLocalName(entity.PkgPath())
	v := TestViewValue{
		wsid: wsid,
		vr:   ts.appStructs.ViewRecords(),
		Key:  ts.appStructs.ViewRecords().KeyBuilder(appdef.NewQName(localPkgName, entity.Entity())),
		Val:  ts.appStructs.ViewRecords().NewValueBuilder(appdef.NewQName(localPkgName, entity.Entity())),
	}
	callback(v.Key, v.Val)
	err := ts.appStructs.ViewRecords().Put(wsid, v.Key, v.Val)
	if err != nil {
		panic(err)
	}
}

func (ts *TestState) PutSecret(name string, secret []byte) {
	ts.secretReader.(*secretReader).secrets[name] = secret
}

type intentAssertions struct {
	t   *testing.T
	kb  istructs.IStateKeyBuilder
	vb  istructs.IStateValueBuilder
	ctx *TestState
}

func (ia *intentAssertions) NotExists() {
	if ia.vb != nil {
		require.Fail(ia.t, "expected intent not to exist")
	}
}

func (ia *intentAssertions) Exists() {
	if ia.vb == nil {
		require.Fail(ia.t, "expected intent to exist")
	}
}

func (ia *intentAssertions) Assert(cb IntentAssertionsCallback) {
	if ia.vb == nil {
		require.Fail(ia.t, "expected intent to exist")
		return
	}
	value := ia.vb.BuildValue()
	if value == nil {
		require.Fail(ia.t, "value builder does not support Assert operation")
		return
	}
	cb(require.New(ia.t), value)
}

func (ia *intentAssertions) Equal(vbc ValueBuilderCallback) {
	if ia.vb == nil {
		panic("intent not found")
	}

	vb, err := ia.ctx.IState.NewValue(ia.kb)
	if err != nil {
		panic(err)
	}
	vbc(vb)

	if !ia.vb.Equal(vb) {
		require.Fail(ia.t, "expected intents to be equal")
	}
}

func (ts *TestState) RequireNoIntents(t *testing.T) {
	if ts.IState.IntentsCount() > 0 {
		require.Fail(t, "expected no intents")
	}
}

func (ts *TestState) RequireIntent(t *testing.T, storage appdef.QName, entity appdef.FullQName, kbc KeyBuilderCallback) IIntentAssertions {
	localPkgName := ts.appDef.PackageLocalName(entity.PkgPath())
	localEntity := appdef.NewQName(localPkgName, entity.Entity())
	kb, err := ts.IState.KeyBuilder(storage, localEntity)
	if err != nil {
		panic(err)
	}
	kbc(kb)
	return &intentAssertions{
		t:   t,
		kb:  kb,
		vb:  ts.IState.FindIntent(kb),
		ctx: ts,
	}
}
