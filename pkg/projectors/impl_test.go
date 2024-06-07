/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package projectors

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/in10nmem"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/isecretsimpl"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/mem"
	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istoragecache"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/vvm/engines"
)

var newWorkspaceCmd = appdef.NewQName("sys", "NewWorkspace")

// Design: Projection Actualizers
// https://dev.heeus.io/launchpad/#!12850
//
// Test description:
//
// 1. Creates sync actualizer initialized with two
// projectors: incrementor, decrementor
// (increments/decrements counter for the event's workspace)
//
// 2. Creates command processor pipeline with
// sync actualizer operator
//
// 3. Feeds command processor with events for
// different workspaces
//
// 4. The projection values for those workspaces checked
func TestBasicUsage_SynchronousActualizer(t *testing.T) {
	require := require.New(t)

	appParts, cleanup, _, appStructs := deployTestApp(
		istructs.AppQName_test1_app1, 1, []istructs.PartitionID{1}, false,
		func(appDef appdef.IAppDefBuilder) {
			appDef.AddPackage("test", "test.com/test")
			ProvideViewDef(appDef, incProjectionView, buildProjectionView)
			ProvideViewDef(appDef, decProjectionView, buildProjectionView)
			appDef.AddCommand(testQName)
			appDef.AddProjector(incrementorName).SetSync(true).Events().Add(testQName, appdef.ProjectorEventKind_Execute)
			appDef.AddProjector(decrementorName).SetSync(true).Events().Add(testQName, appdef.ProjectorEventKind_Execute)
			ws := addWS(appDef, testWorkspace, testWorkspaceDescriptor)
			ws.AddType(incProjectionView)
			ws.AddType(decProjectionView)
		},
		func(cfg *istructsmem.AppConfigType) {
			cfg.AddSyncProjectors(testIncrementor, testDecrementor)
			cfg.Resources.Add(istructsmem.NewCommandFunction(testQName, istructsmem.NullCommandExec))
		})
	defer cleanup()
	createWS(appStructs, istructs.WSID(1001), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	createWS(appStructs, istructs.WSID(1002), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))

	t.Run("Emulate the command processor", func(t *testing.T) {
		proc := cmdProcMock{appParts}

		proc.TestEvent(1001)
		proc.TestEvent(1001)
		proc.TestEvent(1002)
		proc.TestEvent(1001)
		proc.TestEvent(1001)
		proc.TestEvent(1001)
		proc.TestEvent(1002)
		proc.TestEvent(1002)
	})

	// now read the projection values in workspaces
	require.EqualValues(5, getProjectionValue(require, appStructs, incProjectionView, 1001))
	require.EqualValues(3, getProjectionValue(require, appStructs, incProjectionView, 1002))
	require.EqualValues(-5, getProjectionValue(require, appStructs, decProjectionView, 1001))
	require.EqualValues(-3, getProjectionValue(require, appStructs, decProjectionView, 1002))
}

var (
	incrementorName = appdef.NewQName("test", "incrementor_projector")
	decrementorName = appdef.NewQName("test", "decrementor_projector")
)

var incProjectionView = appdef.NewQName("pkg", "Incremented")
var decProjectionView = appdef.NewQName("pkg", "Decremented")
var testWorkspace = appdef.NewQName("pkg", "TestWorkspace")
var testWorkspaceDescriptor = appdef.NewQName("pkg", "TestWorkspaceDescriptor")
var errTestError = errors.New("test error")

var (
	testIncrementor = istructs.Projector{
		Name: incrementorName,
		Func: func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
			wsid := event.Workspace()
			if wsid == 1099 {
				return errTestError
			}
			key, err := s.KeyBuilder(state.View, incProjectionView)
			if err != nil {
				return
			}
			key.PutInt32("pk", 0)
			key.PutInt32("cc", 0)
			el, ok, err := s.CanExist(key)
			if err != nil {
				return
			}
			eb, err := intents.NewValue(key)
			if err != nil {
				return
			}
			if ok {
				eb.PutInt32("myvalue", el.AsInt32("myvalue")+1)
			} else {
				eb.PutInt32("myvalue", 1)
			}
			return
		},
	}
	testDecrementor = istructs.Projector{
		Name: decrementorName,
		Func: func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
			key, err := s.KeyBuilder(state.View, decProjectionView)
			if err != nil {
				return
			}
			key.PutInt32("pk", 0)
			key.PutInt32("cc", 0)
			el, ok, err := s.CanExist(key)
			if err != nil {
				return
			}
			eb, err := intents.NewValue(key)
			if err != nil {
				return
			}
			if ok {
				eb.PutInt32("myvalue", el.AsInt32("myvalue")-1)
			} else {
				eb.PutInt32("myvalue", -1)
			}
			return
		},
	}
)

var buildProjectionView = func(view appdef.IViewBuilder) {
	view.Key().PartKey().AddField("pk", appdef.DataKind_int32)
	view.Key().ClustCols().AddField("cc", appdef.DataKind_int32)
	view.Value().AddField(colValue, appdef.DataKind_int32, true)
}

type (
	appDefCallback func(appDef appdef.IAppDefBuilder)
	appCfgCallback func(cfg *istructsmem.AppConfigType)
)

func deployTestApp(
	appName appdef.AppQName,
	appPartsCount istructs.NumAppPartitions,
	partID []istructs.PartitionID,
	cachedStorage bool,
	prepareAppDef appDefCallback,
	prepareAppCfg appCfgCallback,
) (
	appParts appparts.IAppPartitions,
	cleanup func(),
	metrics imetrics.IMetrics,
	appStructs istructs.IAppStructs,
) {
	appDefBuilder := appdef.New(appName)
	if prepareAppDef != nil {
		prepareAppDef(appDefBuilder)
	}
	provideOffsetsDefImpl(appDefBuilder)
	appDefBuilder.AddCommand(newWorkspaceCmd)

	cfgs := make(istructsmem.AppConfigsType, 1)
	cfg := cfgs.AddConfig(appName, appDefBuilder)
	cfg.SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)
	if prepareAppCfg != nil {
		prepareAppCfg(cfg)
		cfg.Resources.Add(istructsmem.NewCommandFunction(newWorkspaceCmd, istructsmem.NullCommandExec))
	}

	wsDescr := appDefBuilder.AddCDoc(authnz.QNameCDocWorkspaceDescriptor)
	wsDescr.AddField(authnz.Field_WSKind, appdef.DataKind_QName, true)
	wsDescr.SetSingleton()

	appDef, err := appDefBuilder.Build()
	if err != nil {
		panic(err)
	}

	var storageProvider istorage.IAppStorageProvider

	if cachedStorage {
		metrics = imetrics.Provide()
		storageProvider = istoragecache.Provide(1000000, istorageimpl.Provide(mem.Provide()), metrics, "testVM")
	} else {
		storageProvider = istorageimpl.Provide(mem.Provide())
	}

	appStructsProvider := istructsmem.Provide(
		cfgs,
		iratesce.TestBucketsFactory,
		payloads.ProvideIAppTokensFactory(itokensjwt.TestTokensJWT()),
		storageProvider)

	appStructs, err = appStructsProvider.AppStructs(appName)
	if err != nil {
		panic(err)
	}

	secretReader := isecretsimpl.ProvideSecretReader()
	n10nBroker, n10nBrokerCleanup := in10nmem.ProvideEx2(in10n.Quotas{
		Channels:                1000,
		ChannelsPerSubject:      10,
		Subscriptions:           1000,
		SubscriptionsPerSubject: 10,
	}, time.Now)

	appParts, appPartsCleanup, err := appparts.NewWithActualizerWithExtEnginesFactories(
		appStructsProvider,
		NewSyncActualizerFactoryFactory(ProvideSyncActualizerFactory(), secretReader, n10nBroker),
		engines.ProvideExtEngineFactories(
			engines.ExtEngineFactoriesConfig{
				AppConfigs:  cfgs,
				WASMCompile: false,
			}))
	if err != nil {
		panic(err)
	}

	appParts.DeployApp(appName, appDef, appPartsCount, appparts.PoolSize(10, 10, 10))
	appParts.DeployAppPartitions(appName, partID)

	cleanup = func() {
		appPartsCleanup()
		n10nBrokerCleanup()
	}

	return appParts, cleanup, metrics, appStructs
}

func addWS(appDef appdef.IAppDefBuilder, wsKind, wsDescriptorKind appdef.QName) appdef.IWorkspaceBuilder {
	descr := appDef.AddCDoc(wsDescriptorKind)
	descr.AddField("WSKind", appdef.DataKind_QName, true)
	ws := appDef.AddWorkspace(wsKind)
	ws.SetDescriptor(wsDescriptorKind)
	return ws
}

func createWS(appStructs istructs.IAppStructs, ws istructs.WSID, wsDescriptorKind appdef.QName, partition istructs.PartitionID, offset istructs.Offset) {
	// Create workspace
	rebWs := appStructs.Events().GetNewRawEventBuilder(istructs.NewRawEventBuilderParams{
		GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
			Workspace:         ws,
			HandlingPartition: partition,
			PLogOffset:        offset,
			QName:             newWorkspaceCmd,
		},
	})
	cud := rebWs.CUDBuilder().Create(authnz.QNameCDocWorkspaceDescriptor)
	cud.PutRecordID(appdef.SystemField_ID, 1)
	cud.PutQName("WSKind", wsDescriptorKind)
	rawWsEvent, err := rebWs.BuildRawEvent()
	if err != nil {
		panic(err)
	}
	wsEvent, err := appStructs.Events().PutPlog(rawWsEvent, nil, istructsmem.NewIDGenerator())
	if err != nil {
		panic(err)
	}
	appStructs.Records().Apply(wsEvent)
}

func Test_ErrorInSyncActualizer(t *testing.T) {
	require := require.New(t)

	appParts, cleanup, _, appStructs := deployTestApp(
		istructs.AppQName_test1_app1, 1, []istructs.PartitionID{1}, false,
		func(appDef appdef.IAppDefBuilder) {
			appDef.AddPackage("test", "test.com/test")
			ProvideViewDef(appDef, incProjectionView, buildProjectionView)
			ProvideViewDef(appDef, decProjectionView, buildProjectionView)
			appDef.AddCommand(testQName)
			appDef.AddProjector(incrementorName).SetSync(true).Events().Add(testQName, appdef.ProjectorEventKind_Execute)
			appDef.AddProjector(decrementorName).SetSync(true).Events().Add(testQName, appdef.ProjectorEventKind_Execute)
			ws := addWS(appDef, testWorkspace, testWorkspaceDescriptor)
			ws.AddType(incProjectionView)
			ws.AddType(decProjectionView)
		},
		func(cfg *istructsmem.AppConfigType) {
			cfg.AddSyncProjectors(testIncrementor, testDecrementor)
			cfg.Resources.Add(istructsmem.NewCommandFunction(testQName, istructsmem.NullCommandExec))
		})
	defer cleanup()

	createWS(appStructs, istructs.WSID(1001), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	createWS(appStructs, istructs.WSID(1002), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	createWS(appStructs, istructs.WSID(1099), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))

	t.Run("Emulate the command processor", func(t *testing.T) {
		proc := cmdProcMock{appParts}

		require.NoError(proc.TestEvent(1001))
		require.NoError(proc.TestEvent(1001))
		require.NoError(proc.TestEvent(1002))
		require.ErrorContains(proc.TestEvent(1099), errTestError.Error())
	})

	// now read the projection values in workspaces
	require.EqualValues(2, getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1001)))
	require.EqualValues(1, getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1002)))
	require.EqualValues(-2, getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1001)))
	require.EqualValues(-1, getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1002)))
	require.EqualValues(0, getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1099)))
	require.EqualValues(0, getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1099)))
}
