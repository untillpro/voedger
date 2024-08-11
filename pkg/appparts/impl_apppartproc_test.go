/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/testingu/require"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/istorage/mem"
	"github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
)

type mockProcessorRunner struct {
	IProcessorRunner
	mock.Mock
}

func (t *mockProcessorRunner) NewAndRun(ctx context.Context, app appdef.AppQName, partID istructs.PartitionID, name appdef.QName) {
	t.Called(ctx, app, partID, name)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func (t *mockProcessorRunner) SetAppPartitions(ap IAppPartitions) {
	t.Called(ap)
}

func Test_partitionProcessors_deploy(t *testing.T) {
	require := require.New(t)

	prjName1 := appdef.NewQName("test", "projector1")

	ctx, stop := context.WithCancel(context.Background())

	adb1, appDef1 := func() (appdef.IAppDefBuilder, appdef.IAppDef) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		prj := adb.AddProjector(prjName1)
		prj.SetSync(false)
		prj.Events().Add(appdef.QNameAnyCommand, appdef.ProjectorEventKind_Execute)

		return adb, adb.MustBuild()
	}()

	appConfigs := istructsmem.AppConfigsType{}
	appConfigs.AddBuiltInAppConfig(istructs.AppQName_test1_app1, adb1).SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)

	appStructs := istructsmem.Provide(
		appConfigs,
		iratesce.TestBucketsFactory,
		payloads.TestAppTokensFactory(itokensjwt.TestTokensJWT()),
		provider.Provide(mem.Provide(), ""))

	mockProc := &mockProcessorRunner{}
	mockProc.On("SetAppPartitions", mock.Anything).Once()

	appParts, cleanupParts, err := New2(ctx, appStructs, NullSyncActualizerFactory,
		mockProc,
		NullExtensionEngineFactories)
	require.NoError(err)

	defer cleanupParts()

	appParts.DeployApp(istructs.AppQName_test1_app1, nil, appDef1, 1, PoolSize(2, 2, 2), istructs.DefaultNumAppWorkspaces)

	t.Run("deploy 10 partitions", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			mockProc.On("NewAndRun", mock.Anything, istructs.AppQName_test1_app1, istructs.PartitionID(i), prjName1).Once()
		}
		appParts.DeployAppPartitions(istructs.AppQName_test1_app1, []istructs.PartitionID{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
		mockProc.AssertExpectations(t)
	})

	t.Run("redeploy odd partitions", func(t *testing.T) {

		prjName2 := appdef.NewQName("test", "projector2")
		appDef2 := func() appdef.IAppDef {
			adb := appdef.New()
			adb.AddPackage("test", "test.com/test")

			prj := adb.AddProjector(prjName2)
			prj.SetSync(false)
			prj.Events().Add(appdef.QNameAnyCommand, appdef.ProjectorEventKind_Execute)

			return adb.MustBuild()
		}()

		// hack to update appDef
		appParts.(*apps).apps[istructs.AppQName_test1_app1].lastestVersion.def = appDef2

		for i := 0; i < 10; i++ {
			if i%2 == 1 {
				mockProc.On("NewAndRun", mock.Anything, istructs.AppQName_test1_app1, istructs.PartitionID(i), prjName2).Once()
			}
		}
		appParts.DeployAppPartitions(istructs.AppQName_test1_app1, []istructs.PartitionID{1, 3, 5, 7, 9})
		mockProc.AssertExpectations(t)
	})

	stop()
}
