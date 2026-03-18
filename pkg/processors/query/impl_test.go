/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package queryprocessor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/builder"
	"github.com/voedger/voedger/pkg/appdef/filter"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	wsdescutil "github.com/voedger/voedger/pkg/coreutils/testwsdesc"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/testingu"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iauthnzimpl"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/iprocbus"
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
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/vvm/engines"
)

var (
	appName    appdef.AppQName           = istructs.AppQName_test1_app1
	appEngines                           = appparts.PoolSize(10, 100, 10, 0)
	partCount  istructs.NumAppPartitions = 10
	partID     istructs.PartitionID      = 5
	wsID       istructs.WSID             = 15

	pkgBo                 = "bo"
	pkgBoPath             = "test1_app1/bo"
	qNameFunction         = appdef.NewQName("bo", "FindArticlesByModificationTimeStampRange")
	qNameQryDenied        = appdef.NewQName(appdef.SysPackage, "TestDeniedQry") // same as in ACL
	qNameTestWSDescriptor = appdef.NewQName(appdef.SysPackage, "test_ws")
	qNameTestWS           = appdef.NewQName(appdef.SysPackage, "test_wsWS")
)

func TestBasicUsage_RowsProcessorFactory(t *testing.T) {
	require := require.New(t)
	department := func(name string) istructs.IStateValue {
		r := &mockRecord{}
		r.
			On("AsString", "name").Return(name).
			On("QName").Return(qNamePosDepartment)
		sv := &mockStateValue{}
		sv.On("AsRecord", "").Return(r)
		return sv
	}
	skb := &mockStateKeyBuilder{}
	skb.On("PutRecordID", mock.Anything, mock.Anything)
	s := &mockState{}
	s.
		On("KeyBuilder", sys.Storage_Record, appdef.NullQName).Return(skb).
		On("MustExist", mock.Anything).Return(department("Soft drinks")).Once().
		On("MustExist", mock.Anything).Return(department("Alcohol drinks")).Once().
		On("MustExist", mock.Anything).Return(department("Alcohol drinks")).Once().
		On("MustExist", mock.Anything).Return(department("Sweet")).Once()

	var (
		appDef     appdef.IAppDef
		resultMeta appdef.IObject
	)
	t.Run("should be ok to build appDef and resultMeta", func(t *testing.T) {
		adb := builder.New()
		wsb := adb.AddWorkspace(qNameTestWS)
		wsb.AddObject(qNamePosDepartment).
			AddField("name", appdef.DataKind_string, false)
		resBld := wsb.AddObject(qNamePosDepartmentResult)
		resBld.
			AddField("id", appdef.DataKind_int64, true).
			AddField("name", appdef.DataKind_string, false)
		app, err := adb.Build()
		require.NoError(err)

		appDef = app
		resultMeta = appdef.Object(app.Type, qNamePosDepartmentResult)
	})

	params := queryParams{
		elements: []IElement{
			element{
				path: path{rootDocument},
				fields: []IResultField{
					resultField{"id"},
					resultField{"name"},
				},
				refs: []IRefField{
					refField{field: "id_department", ref: "name", key: "id_department/name"},
				},
			},
		},
		count:     1,
		startFrom: 1,
		filters: []IFilter{
			&EqualsFilter{
				field: "id_department/name",
				value: "Alcohol drinks",
			},
		},
		orderBy: []IOrderBy{
			orderBy{
				field: "name",
				desc:  false,
			},
		},
	}
	work := func(id int64, name string, idDepartment int64) pipeline.IWorkpiece {
		return rowsWorkpiece{
			object: &coreutils.TestObject{
				Name: appdef.NewQName("pos", "Article"),
				Data: map[string]interface{}{"id": id, "name": name, "id_department": istructs.RecordID(idDepartment)},
			},
			outputRow: &outputRow{
				keyToIdx: map[string]int{rootDocument: 0},
				values:   make([]interface{}, 1),
			},
			enrichedRootFieldsKinds: make(map[string]appdef.DataKind),
		}
	}

	result := ""

	rowsProcessorErrCh := make(chan error, 1)
	requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
		go func() {
			// SendToBus op will send to the respCh chan so let's handle in a separate goroutine
			processor, respWriterGetter := ProvideRowsProcessorFactory()(context.Background(), appDef, s, params,
				resultMeta, responder, &testMetrics{}, rowsProcessorErrCh)

			require.NoError(processor.SendAsync(work(1, "Cola", 10)))
			require.NoError(processor.SendAsync(work(3, "White wine", 20)))
			require.NoError(processor.SendAsync(work(2, "Amaretto", 20)))
			require.NoError(processor.SendAsync(work(4, "Cake", 40)))
			processor.Close()
			respWriterGetter().Close(nil)
		}()
	})
	responseCh, respMeta, responseErr, err := requestSender.SendRequest(context.Background(), bus.Request{})
	require.NoError(err)
	require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
	require.Equal(http.StatusOK, respMeta.StatusCode)
	for elem := range responseCh {
		bb, err := json.Marshal(elem)
		require.NoError(err)
		result = string(bb)
	}
	require.NoError(*responseErr)
	select {
	case err := <-rowsProcessorErrCh:
		t.Fatal(err)
	default:
	}

	require.Equal(`[[[3,"White wine","Alcohol drinks"]]]`, result)

}

func deployTestAppWithSecretToken(require *require.Assertions,
	prepareAppDef func(appdef.IAppDefBuilder, appdef.IWorkspaceBuilder),
	cfgFunc ...func(*istructsmem.AppConfigType)) (appParts appparts.IAppPartitions, cl func(),
	appTokens istructs.IAppTokens, statelessResources istructsmem.IStatelessResources) {
	cfgs := make(istructsmem.AppConfigsType)
	asf := mem.Provide(testingu.MockTime)
	storageProvider := istorageimpl.Provide(asf)

	qNameFindArticlesByModificationTimeStampRangeParams := appdef.NewQName("bo", "FindArticlesByModificationTimeStampRangeParamsDef")
	qNameDepartment := appdef.NewQName("bo", "Department")
	qNameArticle := appdef.NewQName("bo", "Article")

	adb := builder.New()
	adb.AddPackage(pkgBo, pkgBoPath)

	wsb := adb.AddWorkspace(qNameTestWS)
	wsb.AddCDoc(qNameTestWSDescriptor)
	wsb.SetDescriptor(qNameTestWSDescriptor)

	wsb.AddObject(qNameFindArticlesByModificationTimeStampRangeParams).
		AddField("from", appdef.DataKind_int64, false).
		AddField("till", appdef.DataKind_int64, false)
	wsb.AddCDoc(qNameDepartment).
		AddField("name", appdef.DataKind_string, true)
	wsb.AddObject(qNameArticle).
		AddField("sys.ID", appdef.DataKind_RecordID, true).
		AddField("name", appdef.DataKind_string, true).
		AddField("id_department", appdef.DataKind_int64, true)

	wsdescutil.AddWorkspaceDescriptorStubDef(wsb)

	wsb.AddQuery(qNameFunction).SetParam(qNameFindArticlesByModificationTimeStampRangeParams).SetResult(appdef.NewQName("bo", "Article"))
	wsb.AddCommand(istructs.QNameCommandCUD)
	wsb.AddQuery(qNameQryDenied)

	wsb.AddRole(iauthnz.QNameRoleAuthenticatedUser)
	wsb.AddRole(iauthnz.QNameRoleEveryone)
	wsb.AddRole(iauthnz.QNameRoleSystem)
	wsb.AddRole(iauthnz.QNameRoleAnonymous)
	wsb.AddRole(iauthnz.QNameRoleProfileOwner)
	wsb.AddRole(iauthnz.QNameRoleWorkspaceOwner)

	wsb.Revoke([]appdef.OperationKind{appdef.OperationKind_Execute}, filter.QNames(qNameQryDenied), nil, iauthnz.QNameRoleWorkspaceOwner)

	if prepareAppDef != nil {
		prepareAppDef(adb, wsb)
	}

	statelessResources = istructsmem.NewStatelessResources()
	cfg := cfgs.AddBuiltInAppConfig(appName, adb)
	cfg.SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)

	atf := payloads.ProvideIAppTokensFactory(itokensjwt.TestTokensJWT())
	asp := istructsmem.Provide(cfgs, iratesce.TestBucketsFactory, atf, storageProvider, isequencer.SequencesTrustLevel_0, nil)

	article := func(id, idDepartment istructs.RecordID, name string) istructs.IObject {
		return &coreutils.TestObject{
			Name: appdef.NewQName("bo", "Article"),
			Data: map[string]interface{}{"sys.ID": id, "name": name, "id_department": idDepartment},
		}
	}
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		qNameFunction,
		func(_ context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
			require.Equal(int64(1257894000), args.ArgumentObject.AsInt64("from"))
			require.Equal(int64(2257894000), args.ArgumentObject.AsInt64("till"))
			objects := []istructs.IObject{
				article(1, istructs.MaxRawRecordID+10, "Cola"),
				article(3, istructs.MaxRawRecordID+20, "White wine"),
				article(2, istructs.MaxRawRecordID+20, "Amaretto"),
				article(4, istructs.MaxRawRecordID+40, "Cake"),
			}
			for _, object := range objects {
				err = callback(object)
				if err != nil {
					return err
				}
			}
			return err
		},
	))
	cfg.Resources.Add(istructsmem.NewCommandFunction(istructs.QNameCommandCUD, istructsmem.NullCommandExec))
	cfg.Resources.Add(istructsmem.NewQueryFunction(qNameQryDenied, istructsmem.NullQueryExec))

	for _, f := range cfgFunc {
		f(cfg)
	}

	as, err := asp.BuiltIn(appName)
	require.NoError(err)

	appDef := as.AppDef()

	plogOffset := istructs.FirstOffset
	wlogOffset := istructs.FirstOffset
	grebp := istructs.GenericRawEventBuilderParams{
		HandlingPartition: partID,
		Workspace:         wsID,
		QName:             istructs.QNameCommandCUD,
		RegisteredAt:      istructs.UnixMilli(testingu.MockTime.Now().UnixMilli()),
		PLogOffset:        plogOffset,
		WLogOffset:        wlogOffset,
	}
	reb := as.Events().GetSyncRawEventBuilder(
		istructs.SyncRawEventBuilderParams{
			GenericRawEventBuilderParams: grebp,
			SyncedAt:                     istructs.UnixMilli(testingu.MockTime.Now().UnixMilli()),
		},
	)

	namedDoc := func(qName appdef.QName, id istructs.RecordID, name string) {
		doc := reb.CUDBuilder().Create(qName)
		doc.PutRecordID(appdef.SystemField_ID, id)
		doc.PutString("name", name)
	}
	namedDoc(qNameDepartment, istructs.MaxRawRecordID+10, "Soft drinks")
	namedDoc(qNameDepartment, istructs.MaxRawRecordID+20, "Alcohol drinks")
	namedDoc(qNameDepartment, istructs.MaxRawRecordID+40, "Sweet")

	rawEvent, err := reb.BuildRawEvent()
	require.NoError(err)
	pLogEvent, err := as.Events().PutPlog(rawEvent, nil, istructsmem.NewIDGenerator())
	require.NoError(err)
	require.NoError(as.Records().Apply(pLogEvent))
	err = as.Events().PutWlog(pLogEvent)
	require.NoError(err)
	plogOffset++
	wlogOffset++

	// create stub for cdoc.sys.WorkspaceDescriptor to make query processor work
	require.NoError(err)
	now := testingu.MockTime.Now()
	grebp = istructs.GenericRawEventBuilderParams{
		HandlingPartition: partID,
		Workspace:         wsID,
		QName:             istructs.QNameCommandCUD,
		RegisteredAt:      istructs.UnixMilli(now.UnixMilli()),
		PLogOffset:        plogOffset,
		WLogOffset:        wlogOffset,
	}
	reb = as.Events().GetSyncRawEventBuilder(
		istructs.SyncRawEventBuilderParams{
			GenericRawEventBuilderParams: grebp,
			SyncedAt:                     istructs.UnixMilli(now.UnixMilli()),
		},
	)
	cdocWSDesc := reb.CUDBuilder().Create(authnz.QNameCDocWorkspaceDescriptor)
	cdocWSDesc.PutRecordID(appdef.SystemField_ID, 1)
	cdocWSDesc.PutInt32(authnz.Field_Status, int32(authnz.WorkspaceStatus_Active))
	cdocWSDesc.PutQName(authnz.Field_WSKind, qNameTestWSDescriptor)
	rawEvent, err = reb.BuildRawEvent()
	require.NoError(err)
	pLogEvent, err = as.Events().PutPlog(rawEvent, nil, istructsmem.NewIDGenerator())
	require.NoError(err)
	defer pLogEvent.Release()
	require.NoError(as.Records().Apply(pLogEvent))
	require.NoError(as.Events().PutWlog(pLogEvent))

	vvmCtx, cancel := context.WithCancel(context.Background())
	appParts, cleanup, err := appparts.New2(vvmCtx, asp,
		func(istructs.IAppStructs, istructs.PartitionID) pipeline.ISyncOperator { return &pipeline.NOOP{} }, // no projectors
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
	appParts.DeployApp(appName, nil, appDef, partCount, appEngines, cfg.NumAppWorkspaces())
	appParts.DeployAppPartitions(appName, []istructs.PartitionID{partID})

	appTokens = atf.New(appName)

	combinedCleanup := func() {
		cancel()
		cleanup()
	}
	return appParts, combinedCleanup, appTokens, statelessResources
}

func TestBasicUsage_ServiceFactory(t *testing.T) {
	require := require.New(t)
	result := ""
	body := []byte(`{
						"args":{"from":1257894000,"till":2257894000},
						"elements":[
							{"path":"","fields":["sys.ID","name"],"refs":[["id_department","name"]]}
						],
						"filters":[
							{"expr":"and","args":[{"expr":"eq","args":{"field":"id_department/name","value":"Alcohol drinks"}}]},
							{"expr":"or","args":[{"expr":"eq","args":{"field":"id_department/name","value":"Alcohol drinks"}}]}
						],
						"orderBy":[{"field":"name"}],
						"count":1,
						"startFrom":1
					}`)
	serviceChannel := make(iprocbus.ServiceChannel)

	metrics := imetrics.Provide()
	metricNames := make([]string, 0)

	appParts, cleanAppParts, appTokens, statelessResources := deployTestAppWithSecretToken(require, nil)
	defer cleanAppParts()

	authn := iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter, iauthnzimpl.TestIsDeviceAllowedFuncs)
	queryProcessor := ProvideServiceFactory()(
		serviceChannel,
		appParts,
		3, // max concurrent queries
		metrics, "vvm", authn, itokensjwt.TestTokensJWT(), nil, statelessResources, isecretsimpl.TestSecretReader, state.StateOpts{}, nil)
	processorCtx, processorCtxCancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		queryProcessor.Run(processorCtx)
		wg.Done()
	}()
	systemToken := getSystemToken(appTokens)
	requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
		serviceChannel <- NewQueryMessage(context.Background(), appName, partID, wsID, responder, body, qNameFunction, "127.0.0.1", systemToken)
	})
	respCh, respMeta, respErr, err := requestSender.SendRequest(processorCtx, bus.Request{})
	require.NoError(err)
	require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
	require.Equal(http.StatusOK, respMeta.StatusCode)
	for elem := range respCh {
		bb, err := json.Marshal(elem)
		require.NoError(err)
		result = string(bb)
	}
	require.NoError(*respErr)

	processorCtxCancel()
	wg.Wait()

	_ = metrics.List(func(metric imetrics.IMetric, metricValue float64) (err error) {
		metricNames = append(metricNames, metric.Name())
		return err
	})

	require.Equal(`[[[3,"White wine","Alcohol drinks"]]]`, result)
	require.Contains(metricNames, Metric_QueriesTotal)
	require.Contains(metricNames, Metric_QueriesSeconds)
	require.Contains(metricNames, Metric_BuildSeconds)
	require.Contains(metricNames, Metric_ExecSeconds)
	require.Contains(metricNames, Metric_ExecFieldsSeconds)
	require.Contains(metricNames, Metric_ExecEnrichSeconds)
	require.Contains(metricNames, Metric_ExecFilterSeconds)
	require.Contains(metricNames, Metric_ExecOrderSeconds)
	require.Contains(metricNames, Metric_ExecCountSeconds)
	require.Contains(metricNames, Metric_ExecSendSeconds)
}

func TestRawMode(t *testing.T) {
	require := require.New(t)

	var (
		appDef     appdef.IAppDef
		resultMeta appdef.IObject
	)
	t.Run("should be ok to build appDef and resultMeta", func(t *testing.T) {
		adb := builder.New()
		wsb := adb.AddWorkspace(qNameTestWS)
		wsb.AddObject(istructs.QNameRaw)
		app, err := adb.Build()
		require.NoError(err)

		appDef = app
		resultMeta = appdef.Object(app.Type, istructs.QNameRaw)
	})

	result := ""
	rowsProcessorErrCh := make(chan error, 1)
	requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
		go func() {
			// SendToBus op will send to the respCh chan so let's handle in a separate goroutine
			processor, respWriterGetter := ProvideRowsProcessorFactory()(context.Background(), appDef, &mockState{},
				queryParams{}, resultMeta, responder, &testMetrics{}, rowsProcessorErrCh)

			require.NoError(processor.SendAsync(rowsWorkpiece{
				object: &coreutils.TestObject{
					Data: map[string]interface{}{
						processors.Field_RawObject_Body: `[accepted]`,
					},
				},
				outputRow: &outputRow{
					keyToIdx: map[string]int{rootDocument: 0},
					values:   make([]interface{}, 1),
				},
			}))
			processor.Close()
			respWriterGetter().Close(nil)
		}()
	})

	responseCh, respMeta, responseErr, err := requestSender.SendRequest(context.Background(), bus.Request{})
	require.NoError(err)
	require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
	require.Equal(http.StatusOK, respMeta.StatusCode)
	for elem := range responseCh {
		bb, err := json.Marshal(elem)
		require.NoError(err)
		result = string(bb)
	}
	require.NoError(*responseErr)
	select {
	case err := <-rowsProcessorErrCh:
		t.Fatal(err)
	default:
	}

	require.Equal(`[[["[accepted]"]]]`, result)
}

func Test_epsilon(t *testing.T) {
	options := func(epsilon interface{}) map[string]interface{} {
		options := make(map[string]interface{})
		if epsilon != nil {
			options["epsilon"] = epsilon
		}
		return options
	}
	args := func(options map[string]interface{}) coreutils.MapObject {
		args := make(map[string]interface{})
		if options != nil {
			args["options"] = options
		}
		return args
	}
	t.Run("Should return epsilon", func(t *testing.T) {
		epsilon, err := epsilon(args(options(json.Number(fmt.Sprint(math.E)))))

		require.Equal(t, math.E, epsilon)
		require.NoError(t, err)
	})
	t.Run("Should return error when options is nil", func(t *testing.T) {
		//TODO (FILTER0001)
		t.Skip("//TODO (FILTER0001)")
		epsilon, err := epsilon(args(nil))

		require.Equal(t, 0.0, epsilon)
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("Should return error when epsilon is nil", func(t *testing.T) {
		//TODO (FILTER0001)
		t.Skip("//TODO (FILTER0001)")
		epsilon, err := epsilon(args(options(nil)))

		require.Equal(t, 0.0, epsilon)
		require.ErrorIs(t, err, ErrNotFound)
	})
	t.Run("Should return error when epsilon has wrong type", func(t *testing.T) {
		epsilon, err := epsilon(args(options("0.00000001")))

		require.Equal(t, 0.0, epsilon)
		require.ErrorIs(t, err, coreutils.ErrFieldTypeMismatch)
	})
}

func Test_nearlyEqual(t *testing.T) {
	t.Skip("temp skip")
	tests := []struct {
		name    string
		first   float64
		second  float64
		epsilon float64
		want    bool
	}{
		{
			name:    "Regular large numbers 1",
			first:   1000000.0,
			second:  1000001.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Regular large numbers 2",
			first:   1000001.0,
			second:  1000000.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Regular large numbers 3",
			first:   10000.0,
			second:  10001.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Regular large numbers 4",
			first:   10001.0,
			second:  10000.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Negative large numbers 1",
			first:   -1000000.0,
			second:  -1000001.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Negative large numbers 2",
			first:   -1000001.0,
			second:  -1000000.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Negative large numbers 3",
			first:   -10000.0,
			second:  -10001.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Negative large numbers 4",
			first:   -10001.0,
			second:  -10000.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers around one 1",
			first:   1.0000001,
			second:  1.0000002,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers around one 2",
			first:   1.0000002,
			second:  1.0000001,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers around one 3",
			first:   1.0002,
			second:  1.0001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers around one 4",
			first:   1.0001,
			second:  1.0002,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers around minus one 1",
			first:   -1.0000001,
			second:  -1.0000002,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers around minus one 2",
			first:   -1.0000002,
			second:  -1.0000001,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers around minus one 3",
			first:   -1.0002,
			second:  -1.0001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers around minus one 4",
			first:   -1.0001,
			second:  -1.0002,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers between one and zero 1",
			first:   0.000000001000001,
			second:  0.000000001000002,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers between one and zero 2",
			first:   0.000000001000002,
			second:  0.000000001000001,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers between one and zero 3",
			first:   0.000000000001002,
			second:  0.000000000001001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers between one and zero 4",
			first:   0.000000000001001,
			second:  0.000000000001002,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers between minus one and zero 1",
			first:   -0.000000001000001,
			second:  -0.000000001000002,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers between minus one and zero 2",
			first:   -0.000000001000002,
			second:  -0.000000001000001,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Numbers between minus one and zero 3",
			first:   -0.000000000001002,
			second:  -0.000000000001001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Numbers between minus one and zero 4",
			first:   -0.000000000001001,
			second:  -0.000000000001002,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Small differences away from zero 1",
			first:   0.3,
			second:  0.30000003,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Small differences away from zero 2",
			first:   -0.3,
			second:  -0.30000003,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons involving zero 1",
			first:   0.0,
			second:  0.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons involving zero 2",
			first:   0.00000001,
			second:  0.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 3",
			first:   0.0,
			second:  0.00000001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 4",
			first:   -0.00000001,
			second:  0.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 5",
			first:   0.0,
			second:  -0.00000001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 6",
			first:   0.0,
			second:  1e-40,
			epsilon: 0.01,
			want:    true,
		},
		{
			name:    "Comparisons involving zero 7",
			first:   1e-40,
			second:  0.0,
			epsilon: 0.01,
			want:    true,
		},
		{
			name:    "Comparisons involving zero 8",
			first:   0.0,
			second:  1e-40,
			epsilon: 0.000001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 9",
			first:   1e-40,
			second:  0.0,
			epsilon: 0.000001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 10",
			first:   0.0,
			second:  -1e-40,
			epsilon: 0.01,
			want:    true,
		},
		{
			name:    "Comparisons involving zero 11",
			first:   -1e-40,
			second:  0.0,
			epsilon: 0.01,
			want:    true,
		},
		{
			name:    "Comparisons involving zero 12",
			first:   0.0,
			second:  -1e-40,
			epsilon: 0.000001,
			want:    false,
		},
		{
			name:    "Comparisons involving zero 13",
			first:   -1e-40,
			second:  0.0,
			epsilon: 0.000001,
			want:    false,
		},
		{
			name:    "Comparisons involving extreme values 1",
			first:   math.MaxFloat64,
			second:  math.MaxFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons involving extreme values 2",
			first:   math.MaxFloat64,
			second:  -math.MaxFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving extreme values 3",
			first:   -math.MaxFloat64,
			second:  math.MaxFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving extreme values 4",
			first:   math.MaxFloat64,
			second:  math.MaxFloat64 / 2,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving extreme values 5",
			first:   math.MaxFloat64,
			second:  -math.MaxFloat64 / 2,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving extreme values 6",
			first:   -math.MaxFloat64,
			second:  math.MaxFloat64 / 2,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving infinities 1",
			first:   math.Inf(+1),
			second:  math.Inf(+1),
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons involving infinities 2",
			first:   math.Inf(-1),
			second:  math.Inf(-1),
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons involving infinities 3",
			first:   math.Inf(-1),
			second:  math.Inf(+1),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving infinities 4",
			first:   math.Inf(+1),
			second:  math.MaxFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving infinities 5",
			first:   math.Inf(-1),
			second:  -math.MaxFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 1",
			first:   math.NaN(),
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 2",
			first:   math.NaN(),
			second:  0.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 3",
			first:   0.0,
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 4",
			first:   math.NaN(),
			second:  math.Inf(+1),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 5",
			first:   math.Inf(+1),
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 6",
			first:   math.NaN(),
			second:  math.Inf(-1),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 7",
			first:   math.Inf(-1),
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 8",
			first:   math.NaN(),
			second:  math.MaxFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 9",
			first:   math.MaxFloat64,
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 10",
			first:   math.NaN(),
			second:  -math.MaxFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 11",
			first:   -math.MaxFloat64,
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 12",
			first:   math.NaN(),
			second:  math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 13",
			first:   math.SmallestNonzeroFloat64,
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 14",
			first:   math.NaN(),
			second:  -math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons involving NaN values 15",
			first:   -math.SmallestNonzeroFloat64,
			second:  math.NaN(),
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers on opposite sides of zero 1",
			first:   1.000000001,
			second:  -1.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers on opposite sides of zero 2",
			first:   -1.0,
			second:  1.000000001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers on opposite sides of zero 3",
			first:   -1.000000001,
			second:  1.0,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers on opposite sides of zero 4",
			first:   1.0,
			second:  -1.000000001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers on opposite sides of zero 5",
			first:   10 * math.SmallestNonzeroFloat64,
			second:  10 * -math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers on opposite sides of zero 6",
			first:   10000 * math.SmallestNonzeroFloat64,
			second:  10000 * -math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers very close to zero 1",
			first:   math.SmallestNonzeroFloat64,
			second:  math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 2",
			first:   math.SmallestNonzeroFloat64,
			second:  -math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 3",
			first:   -math.SmallestNonzeroFloat64,
			second:  math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 4",
			first:   math.SmallestNonzeroFloat64,
			second:  0.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 5",
			first:   0.0,
			second:  math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 6",
			first:   -math.SmallestNonzeroFloat64,
			second:  0.0,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 7",
			first:   0.0,
			second:  -math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    true,
		},
		{
			name:    "Comparisons of numbers very close to zero 8",
			first:   0.000000001,
			second:  -math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers very close to zero 9",
			first:   0.000000001,
			second:  math.SmallestNonzeroFloat64,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers very close to zero 10",
			first:   math.SmallestNonzeroFloat64,
			second:  0.000000001,
			epsilon: 0.00001,
			want:    false,
		},
		{
			name:    "Comparisons of numbers very close to zero 11",
			first:   -math.SmallestNonzeroFloat64,
			second:  0.000000001,
			epsilon: 0.00001,
			want:    false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, nearlyEqual(test.first, test.second, test.epsilon))
		})
	}
}

func TestRateLimiter(t *testing.T) {
	require := require.New(t)

	var buf syncBuffer
	logger.SetCtxWriters(&buf, &buf)
	defer logger.SetCtxWriters(os.Stdout, os.Stderr)
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()

	serviceChannel := make(iprocbus.ServiceChannel)

	qNameMyFuncParams := appdef.NewQName(appdef.SysPackage, "myFuncParams")
	qNameMyFuncResults := appdef.NewQName(appdef.SysPackage, "results")
	qName := appdef.NewQName(appdef.SysPackage, "myFunc")
	appParts, cleanAppParts, appTokens, statelessResources := deployTestAppWithSecretToken(require,
		func(_ appdef.IAppDefBuilder, wsb appdef.IWorkspaceBuilder) {
			wsb.AddObject(qNameMyFuncParams)
			wsb.AddObject(qNameMyFuncResults).
				AddField("fld", appdef.DataKind_string, false)
			qry := wsb.AddQuery(qName)
			qry.SetParam(qNameMyFuncParams).SetResult(qNameMyFuncResults)
		},
		func(cfg *istructsmem.AppConfigType) {
			myFunc := istructsmem.NewQueryFunction(qName, istructsmem.NullQueryExec)
			// declare a test func

			cfg.Resources.Add(myFunc)

			// declare rate limits
			cfg.FunctionRateLimits.AddWorkspaceLimit(qName, istructs.RateLimit{
				Period:                time.Minute,
				MaxAllowedPerDuration: 2,
			})
		})

	defer cleanAppParts()

	// create aquery processor
	metrics := imetrics.Provide()
	authn := iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter, iauthnzimpl.TestIsDeviceAllowedFuncs)
	queryProcessor := ProvideServiceFactory()(
		serviceChannel,
		appParts,
		3, // max concurrent queries
		metrics, "vvm", authn, itokensjwt.TestTokensJWT(), nil, statelessResources, isecretsimpl.TestSecretReader, state.StateOpts{}, nil)
	go queryProcessor.Run(context.Background())
	systemToken := getSystemToken(appTokens)
	body := []byte(`{
		"args":{},
		"elements":[{"path":"","fields":["fld"]}]
	}`)
	requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
		serviceChannel <- NewQueryMessage(context.Background(), appName, partID, wsID, responder, body, qName, "127.0.0.1", systemToken)
	})

	// execute query
	for i := 0; i < 3; i++ {
		buf.Reset()
		respCh, respMeta, respErr, err := requestSender.SendRequest(context.Background(), bus.Request{})
		require.NoError(err)
		require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)

		for range respCh {
		}
		out := buf.String()
		if i != 2 {
			// first 2 - ok
			require.NoError(*respErr)
			require.Equal(http.StatusOK, respMeta.StatusCode)
			require.Contains(out, "stage=qp.success")
			require.NotContains(out, "stage=qp.error")
		} else {
			// 3rd exceeds the limit - not often than twice per minute
			require.Error(*respErr)
			require.Equal(http.StatusTooManyRequests, respMeta.StatusCode)
			require.Contains(out, "stage=qp.error")
			require.NotContains(out, "stage=qp.success")
		}
	}
}

func TestAuthnz(t *testing.T) {
	require := require.New(t)
	body := []byte(`{}`)
	serviceChannel := make(iprocbus.ServiceChannel)

	metrics := imetrics.Provide()

	appParts, cleanAppParts, appTokens, statelessResources := deployTestAppWithSecretToken(require, nil)
	defer cleanAppParts()

	authn := iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter, iauthnzimpl.TestIsDeviceAllowedFuncs)
	queryProcessor := ProvideServiceFactory()(
		serviceChannel,
		appParts,
		3, // max concurrent queries
		metrics, "vvm", authn, itokensjwt.TestTokensJWT(), nil, statelessResources, isecretsimpl.TestSecretReader, state.StateOpts{}, nil)
	go queryProcessor.Run(context.Background())

	t.Run("no token for a query that requires authorization -> 403 unauthorized", func(t *testing.T) {
		requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
			serviceChannel <- NewQueryMessage(context.Background(), appName, partID, wsID, responder, body, qNameFunction, "127.0.0.1", "")
		})
		respCh, respMeta, respErr, err := requestSender.SendRequest(context.Background(), bus.Request{})

		require.NoError(err)
		require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
		require.Equal(http.StatusForbidden, respMeta.StatusCode)
		for range respCh {
		}
		var se coreutils.SysError
		require.ErrorAs(*respErr, &se)
		require.Equal(http.StatusForbidden, se.HTTPStatus)
	})

	t.Run("expired token -> 401 unauthorized", func(t *testing.T) {
		systemToken := getSystemToken(appTokens)
		// make the token be expired
		testingu.MockTime.Add(2 * time.Minute)
		requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
			serviceChannel <- NewQueryMessage(context.Background(), appName, partID, wsID, responder, body, qNameFunction, "127.0.0.1", systemToken)
		})
		respCh, respMeta, respErr, err := requestSender.SendRequest(context.Background(), bus.Request{})
		require.NoError(err)
		require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
		require.Equal(http.StatusUnauthorized, respMeta.StatusCode)
		for range respCh {
		}
		var se coreutils.SysError
		require.ErrorAs(*respErr, &se)
		require.Equal(http.StatusUnauthorized, se.HTTPStatus)
	})

	t.Run("token provided, query a denied func -> 403 forbidden", func(t *testing.T) {
		token := getTestToken(appTokens, wsID)
		requestSender := bus.NewIRequestSender(testingu.MockTime, func(requestCtx context.Context, request bus.Request, responder bus.IResponder) {
			serviceChannel <- NewQueryMessage(context.Background(), appName, partID, wsID, responder, body, qNameQryDenied, "127.0.0.1", token)
		})
		respCh, respMeta, respErr, err := requestSender.SendRequest(context.Background(), bus.Request{})
		require.NoError(err)
		require.Equal(httpu.ContentType_ApplicationJSON, respMeta.ContentType)
		require.Equal(http.StatusForbidden, respMeta.StatusCode)
		for range respCh {
		}
		var se coreutils.SysError
		require.ErrorAs(*respErr, &se)
		require.Equal(http.StatusForbidden, se.HTTPStatus)
	})
}

type testOutputRow struct {
	fields     []string
	fieldToIdx map[string]int
	values     []interface{}
}

func (r *testOutputRow) Set(alias string, value interface{}) {
	if r.values == nil {
		r.values = make([]interface{}, len(r.fields))
		r.fieldToIdx = make(map[string]int)
		for i, field := range r.fields {
			r.fieldToIdx[field] = i
		}
	}
	r.values[r.fieldToIdx[alias]] = value
}

func (r testOutputRow) Value(alias string) interface{} { return r.values[r.fieldToIdx[alias]] }
func (r testOutputRow) Values() []interface{}          { return r.values }

type testFilter struct {
	match bool
	err   error
}

func (f testFilter) IsMatch(FieldsKinds, IOutputRow) (bool, error) {
	return f.match, f.err
}

type testWorkpiece struct {
	object    istructs.IObject
	outputRow IOutputRow
	release   func()
}

func (w testWorkpiece) Object() istructs.IObject { return w.object }
func (w testWorkpiece) OutputRow() IOutputRow    { return w.outputRow }
func (w testWorkpiece) EnrichedRootFieldsKinds() FieldsKinds {
	return FieldsKinds{}
}
func (w testWorkpiece) PutEnrichedRootFieldKind(string, appdef.DataKind) {
	panic("implement me")
}
func (w testWorkpiece) Release() {
	if w.release != nil {
		w.release()
	}
}

type testMetrics struct{}

func (m *testMetrics) Increase(string, float64) {}

func getTestToken(appTokens istructs.IAppTokens, wsid istructs.WSID) string {
	pp := payloads.PrincipalPayload{
		Login:       "syslogin",
		SubjectKind: istructs.SubjectKind_User,
		ProfileWSID: wsid,
	}
	token, err := appTokens.IssueToken(time.Minute, &pp)
	if err != nil {
		panic(err)
	}
	return token
}

func getSystemToken(appTokens istructs.IAppTokens) string {
	pp := payloads.PrincipalPayload{
		Login:       "syslogin",
		SubjectKind: istructs.SubjectKind_User,
		ProfileWSID: istructs.NullWSID,
	}
	token, err := appTokens.IssueToken(time.Minute, &pp)
	if err != nil {
		panic(err)
	}
	return token
}

type mockState struct {
	istructs.IState
	mock.Mock
}

func (s *mockState) KeyBuilder(storage, entity appdef.QName) (builder istructs.IStateKeyBuilder, err error) {
	return s.Called(storage, entity).Get(0).(istructs.IStateKeyBuilder), err
}

func (s *mockState) MustExist(key istructs.IStateKeyBuilder) (value istructs.IStateValue, err error) {
	return s.Called(key).Get(0).(istructs.IStateValue), err
}

type mockStateKeyBuilder struct {
	istructs.IStateKeyBuilder
	mock.Mock
}

func (b *mockStateKeyBuilder) String() string { return "" }
func (b *mockStateKeyBuilder) PutRecordID(name string, value istructs.RecordID) {
	b.Called(name, value)
}

type mockStateValue struct {
	istructs.IStateValue
	istructs.IStateRecordValue
	mock.Mock
}

func (o *mockStateValue) AsRecord() istructs.IRecord {
	return o.Called("").Get(0).(istructs.IRecord)
}

type mockRecord struct {
	istructs.IRecord
	mock.Mock
}

func (r *mockRecord) AsString(name string) string { return r.Called(name).String(0) }
func (r *mockRecord) QName() appdef.QName         { return r.Called().Get(0).(appdef.QName) }

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
