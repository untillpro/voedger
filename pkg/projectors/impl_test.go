/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package projectors

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/mem"
	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istoragecache"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/authnz"
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

	_, cleanup, _, appStructs := deployTestApp(
		istructs.AppQName_test1_app1, 1, []istructs.PartitionID{1}, false,
		func(appDef appdef.IAppDefBuilder) {
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
			cfg.AddSyncProjectors(
				func(istructs.PartitionID) istructs.Projector {
					return istructs.Projector{Name: incrementorName, Func: incrementor}
				},
				func(istructs.PartitionID) istructs.Projector {
					return istructs.Projector{Name: decrementorName, Func: decrementor}
				},
			)
			cfg.Resources.Add(istructsmem.NewCommandFunction(testQName, istructsmem.NullCommandExec))
		})
	defer cleanup()
	createWS(appStructs, istructs.WSID(1001), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	createWS(appStructs, istructs.WSID(1002), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	actualizerFactory := ProvideSyncActualizerFactory()

	// create actualizer with two factories
	conf := SyncActualizerConf{
		Ctx:        context.Background(),
		Partition:  istructs.PartitionID(1),
		AppStructs: func() istructs.IAppStructs { return appStructs },
	}
	projectors := appStructs.SyncProjectors()
	require.Len(projectors, 2)
	actualizer := actualizerFactory(conf, projectors[0], projectors[1:]...)

	// create partition processor pipeline
	processor := pipeline.NewSyncPipeline(context.Background(), "partition processor", pipeline.WireSyncOperator("actualizer", actualizer))

	// feed partition processor
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1002}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1002}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1002}))

	// now read the projection values in workspaces
	require.Equal(int32(5), getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1001)))
	require.Equal(int32(3), getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1002)))
	require.Equal(int32(-5), getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1001)))
	require.Equal(int32(-3), getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1002)))
}

var (
	incrementorName = appdef.NewQName("test", "incremenor_projector")
	decrementorName = appdef.NewQName("test", "decrementor_projector")
)

var incProjectionView = appdef.NewQName("pkg", "Incremented")
var decProjectionView = appdef.NewQName("pkg", "Decremented")
var testWorkspace = appdef.NewQName("pkg", "TestWorkspace")
var testWorkspaceDescriptor = appdef.NewQName("pkg", "TestWorkspaceDescriptor")

var (
	incrementorFactory = func() istructs.Projector {
		return istructs.Projector{Name: incrementorName, Func: incrementor}
	}
	decrementorFactory = func() istructs.Projector {
		return istructs.Projector{Name: decrementorName, Func: decrementor}
	}
)

var (
	incrementor = func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
		wsid := event.Workspace()
		if wsid == 1099 {
			return errors.New("test err")
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
	}
	decrementor = func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
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
	appName istructs.AppQName,
	appPartsCount int,
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
	appDefBuilder := appdef.New()
	if prepareAppDef != nil {
		prepareAppDef(appDefBuilder)
	}
	provideOffsetsDefImpl(appDefBuilder)
	appDefBuilder.AddCommand(newWorkspaceCmd)

	cfgs := make(istructsmem.AppConfigsType, 1)
	cfg := cfgs.AddConfig(appName, appDefBuilder)
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

	appParts, cleanup, err = appparts.New(appStructsProvider)
	if err != nil {
		panic(err)
	}

	appParts.DeployApp(appName, appDef, appPartsCount, cluster.PoolSize(10, 10, 10))
	appParts.DeployAppPartitions(appName, partID)

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

	_, cleanup, _, appStructs := deployTestApp(
		istructs.AppQName_test1_app1, 1, []istructs.PartitionID{1}, false,
		func(appDef appdef.IAppDefBuilder) {
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
			cfg.AddSyncProjectors(
				func(istructs.PartitionID) istructs.Projector {
					return istructs.Projector{Name: incrementorName, Func: incrementor}
				},
				func(istructs.PartitionID) istructs.Projector {
					return istructs.Projector{Name: decrementorName, Func: decrementor}
				},
			)
			cfg.Resources.Add(istructsmem.NewCommandFunction(testQName, istructsmem.NullCommandExec))
		})
	defer cleanup()
	actualizerFactory := ProvideSyncActualizerFactory()

	// create actualizer with two factories
	conf := SyncActualizerConf{
		Ctx:        context.Background(),
		Partition:  istructs.PartitionID(1),
		AppStructs: func() istructs.IAppStructs { return appStructs },
	}
	projectors := appStructs.SyncProjectors()
	require.Len(projectors, 2)
	actualizer := actualizerFactory(conf, projectors[0], projectors[1:]...)

	createWS(appStructs, istructs.WSID(1001), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	createWS(appStructs, istructs.WSID(1002), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))
	createWS(appStructs, istructs.WSID(1099), testWorkspaceDescriptor, istructs.PartitionID(1), istructs.Offset(1))

	// create partition processor pipeline
	processor := pipeline.NewSyncPipeline(context.Background(), "partition processor", pipeline.WireSyncOperator("actualizer", actualizer))

	// feed partition processor
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEventMock{wsid: 1002}))
	err := processor.SendSync(&plogEventMock{wsid: 1099})
	require.Error(err)
	require.Equal("test err", err.Error())

	// now read the projection values in workspaces
	require.Equal(int32(2), getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1001)))
	require.Equal(int32(1), getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1002)))
	require.Equal(int32(-2), getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1001)))
	require.Equal(int32(-1), getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1002)))
	require.Equal(int32(0), getProjectionValue(require, appStructs, incProjectionView, istructs.WSID(1099)))
	require.Equal(int32(0), getProjectionValue(require, appStructs, decProjectionView, istructs.WSID(1099)))
}
