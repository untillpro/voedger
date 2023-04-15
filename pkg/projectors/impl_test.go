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
	"github.com/untillpro/voedger/pkg/iratesce"
	"github.com/untillpro/voedger/pkg/istorage"
	"github.com/untillpro/voedger/pkg/istorageimpl"
	"github.com/untillpro/voedger/pkg/istructs"
	"github.com/untillpro/voedger/pkg/istructsmem"
	payloads "github.com/untillpro/voedger/pkg/itokens-payloads"
	"github.com/untillpro/voedger/pkg/itokensjwt"
	"github.com/untillpro/voedger/pkg/pipeline"
	"github.com/untillpro/voedger/pkg/state"
)

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

	app := appStructs(func(cfg *istructsmem.AppConfigType) {
		ProvideViewSchema(cfg, incProjectionView, buildProjectionSchema)
		ProvideViewSchema(cfg, decProjectionView, buildProjectionSchema)
	})
	actualizerFactory := ProvideSyncActualizerFactory()

	// create actualizer with two factories
	conf := SyncActualizerConf{
		Ctx:        context.Background(),
		Partition:  istructs.PartitionID(1),
		AppStructs: func() istructs.IAppStructs { return app },
	}
	actualizer := actualizerFactory(conf, incrementorFactory, decrementorFactory)

	// create partition processor pipeline
	processor := pipeline.NewSyncPipeline(context.Background(), "partition processor", pipeline.WireSyncOperator("actualizer", actualizer))

	// feed partition processor
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1002}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1002}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1002}))

	// now read the projection values in workspaces
	require.Equal(int32(5), getProjectionValue(require, app, incProjectionView, istructs.WSID(1001)))
	require.Equal(int32(3), getProjectionValue(require, app, incProjectionView, istructs.WSID(1002)))
	require.Equal(int32(-5), getProjectionValue(require, app, decProjectionView, istructs.WSID(1001)))
	require.Equal(int32(-3), getProjectionValue(require, app, decProjectionView, istructs.WSID(1002)))
}

var (
	incrementorName = istructs.NewQName("test", "incremenor_projector")
	decrementorName = istructs.NewQName("test", "decrementor_projector")
)

var incProjectionView = istructs.NewQName("pkg", "Incremented")
var decProjectionView = istructs.NewQName("pkg", "Decremented")

var (
	incrementorFactory = func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{Name: incrementorName, Func: incrementor}
	}
	decrementorFactory = func(partition istructs.PartitionID) istructs.Projector {
		return istructs.Projector{Name: decrementorName, Func: decrementor}
	}
)

var (
	incrementor = func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
		wsid := event.Workspace()
		if wsid == 1099 {
			return errors.New("test err")
		}
		key, err := s.KeyBuilder(state.ViewRecordsStorage, incProjectionView)
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
		key, err := s.KeyBuilder(state.ViewRecordsStorage, decProjectionView)
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

var buildProjectionSchema = func(builder IViewSchemaBuilder) {
	builder.PartitionKeyField("pk", istructs.DataKind_int32, false)
	builder.ClusteringColumnField("cc", istructs.DataKind_int32, false)
	builder.ValueField(colValue, istructs.DataKind_int32, true)
}

type cfgCallback func(cfg *istructsmem.AppConfigType)

func appStructs(cb cfgCallback) istructs.IAppStructs {
	cfgs := make(istructsmem.AppConfigsType, 1)
	cfg := cfgs.AddConfig(istructs.AppQName_test1_app1)
	cb(cfg)
	cfg.Schemas.Add(incrementorName, istructs.SchemaKind_Object)
	cfg.Schemas.Add(decrementorName, istructs.SchemaKind_Object)
	asf := istorage.ProvideMem()
	storageProvider := istorageimpl.Provide(asf)
	prov, _ := istructsmem.Provide(
		cfgs,
		iratesce.TestBucketsFactory,
		payloads.ProvideIAppTokensFactory(itokensjwt.TestTokensJWT()),
		storageProvider)
	structs, _ := prov.AppStructs(istructs.AppQName_test1_app1)
	return structs
}

func Test_ErrorInSyncActualizer(t *testing.T) {
	require := require.New(t)

	app := appStructs(func(cfg *istructsmem.AppConfigType) {
		ProvideViewSchema(cfg, incProjectionView, buildProjectionSchema)
		ProvideViewSchema(cfg, decProjectionView, buildProjectionSchema)
	})
	actualizerFactory := ProvideSyncActualizerFactory()

	// create actualizer with two factories
	conf := SyncActualizerConf{
		Ctx:        context.Background(),
		Partition:  istructs.PartitionID(1),
		AppStructs: func() istructs.IAppStructs { return app },
	}
	actualizer := actualizerFactory(conf, incrementorFactory, decrementorFactory)

	// create partition processor pipeline
	processor := pipeline.NewSyncPipeline(context.Background(), "partition processor", pipeline.WireSyncOperator("actualizer", actualizer))

	// feed partition processor
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1001}))
	require.NoError(processor.SendSync(&plogEvent{wsid: 1002}))
	err := processor.SendSync(&plogEvent{wsid: 1099})
	require.NotNil(err)
	require.Equal("[actualizer/doSync] [ErrorHandler/doSync] [SyncActualizer/doSync] [Projector/doSync] test err", err.Error())

	// now read the projection values in workspaces
	require.Equal(int32(2), getProjectionValue(require, app, incProjectionView, istructs.WSID(1001)))
	require.Equal(int32(1), getProjectionValue(require, app, incProjectionView, istructs.WSID(1002)))
	require.Equal(int32(-2), getProjectionValue(require, app, decProjectionView, istructs.WSID(1001)))
	require.Equal(int32(-1), getProjectionValue(require, app, decProjectionView, istructs.WSID(1002)))
	require.Equal(int32(0), getProjectionValue(require, app, incProjectionView, istructs.WSID(1099)))
	require.Equal(int32(0), getProjectionValue(require, app, decProjectionView, istructs.WSID(1099)))
}
