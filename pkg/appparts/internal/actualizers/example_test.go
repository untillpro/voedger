/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package actualizers_test

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/mock"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts/internal/actualizers"
	"github.com/voedger/voedger/pkg/istructs"
)

type mockActualizerRunner struct {
	mock.Mock
}

func (t *mockActualizerRunner) Run(ctx context.Context, app appdef.AppQName, partID istructs.PartitionID, name appdef.QName) {
	t.Called(ctx, app, partID, name)

	<-ctx.Done()
}

func Example() {
	appName := istructs.AppQName_test1_app1
	partID := istructs.PartitionID(1)

	ctx, stop := context.WithCancel(context.Background())

	runner := &mockActualizerRunner{}

	actualizers := actualizers.New(appName, partID)

	appDef := func(prjNames ...appdef.QName) appdef.IAppDef {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")
		wsb := adb.AddWorkspace(appdef.NewQName("test", "workspace"))
		for _, name := range prjNames {
			wsb.AddProjector(name).SetSync(false).Events().Add(appdef.QNameAnyCommand, appdef.ProjectorEventKind_Execute)
		}
		return adb.MustBuild()
	}

	{
		// deploy partition with appDef version 1
		prjNames := appdef.MustParseQNames("test.p1", "test.p2", "test.p3", "test.p4", "test.p5")
		appDefV1 := appDef(prjNames...)

		for _, name := range prjNames {
			runner.On("Run", mock.Anything, appName, partID, name).Once()
		}

		actualizers.Deploy(ctx, appDefV1, runner.Run)

		fmt.Println(actualizers.Enum())
	}

	{
		// redeploy partition with appDef version 2
		prjNames := appdef.MustParseQNames("test.p3", "test.p4", "test.p5", "test.p6", "test.p7")
		appDefV2 := appDef(prjNames...)

		for _, name := range prjNames {
			runner.On("Run", mock.Anything, appName, partID, name).Once()
		}

		actualizers.Deploy(ctx, appDefV2, runner.Run)

		fmt.Println(actualizers.Enum())
	}

	{
		// stop vvm from context, wait actualizers finished
		stop()
		actualizers.Wait()
		fmt.Println(actualizers.Enum())
	}

	// Output:
	// [test.p1 test.p2 test.p3 test.p4 test.p5]
	// [test.p3 test.p4 test.p5 test.p6 test.p7]
	// []
}
