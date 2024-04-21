/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package apppartsctl_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/apppartsctl"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/istorage/mem"
	"github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
)

func Example() {
	buildAppDef := func(verInfo ...string) (appdef.IAppDefBuilder, appdef.IAppDef) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")
		adb.AddCDoc(appdef.NewQName("test", "verInfo")).SetComment(verInfo...)
		app, err := adb.Build()
		if err != nil {
			panic(err)
		}
		return adb, app
	}

	appConfigs := istructsmem.AppConfigsType{}
	adb_1_v1, app_1_v1 := buildAppDef("app-1 ver.1")
	adb_2_v1, app_2_v1 := buildAppDef("app-2 ver.1")
	appConfigs.AddConfig(istructs.AppQName_test1_app1, adb_1_v1).SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)
	appConfigs.AddConfig(istructs.AppQName_test1_app2, adb_2_v1).SetNumAppWorkspaces(istructs.DefaultNumAppWorkspaces)

	appStructs := istructsmem.Provide(
		appConfigs,
		iratesce.TestBucketsFactory,
		payloads.TestAppTokensFactory(itokensjwt.TestTokensJWT()),
		provider.Provide(mem.Provide(), ""))

	appParts, cleanupParts, err := appparts.New(appStructs)
	if err != nil {
		panic(err)
	}
	defer cleanupParts()

	appPartsCtl, cleanupCtl, err := apppartsctl.New(appParts, []cluster.BuiltInApp{
		{Name: istructs.AppQName_test1_app1,
			Def: app_1_v1,
			AppDeploymentDescriptor: cluster.AppDeploymentDescriptor{
				NumParts:         2,
				EnginePoolSize:   [cluster.ProcessorKind_Count]int{2, 2, 2},
				NumAppWorkspaces: 3,
			},
		},
		{Name: istructs.AppQName_test1_app2,
			Def: app_2_v1,
			AppDeploymentDescriptor: cluster.AppDeploymentDescriptor{
				NumParts:         3,
				EnginePoolSize:   [cluster.ProcessorKind_Count]int{2, 2, 2},
				NumAppWorkspaces: 4,
			},
		},
	})

	if err != nil {
		panic(err)
	}
	defer cleanupCtl()

	err = appPartsCtl.Prepare()
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go appPartsCtl.Run(ctx)

	borrow_work_release := func(appName istructs.AppQName, partID istructs.PartitionID, proc cluster.ProcessorKind) {
		part, err := appParts.Borrow(appName, partID, proc)
		for errors.Is(err, appparts.ErrNotFound) {
			time.Sleep(time.Nanosecond)
			part, err = appParts.Borrow(appName, partID, proc) // Service lag, retry until found
		}
		if err != nil {
			panic(err)
		}

		defer part.Release()

		fmt.Println(part.App(), "part", part.ID())
		part.AppStructs().AppDef().Types(
			func(typ appdef.IType) {
				if !typ.IsSystem() {
					fmt.Println("-", typ, typ.Comment())
				}
			})
	}

	borrow_work_release(istructs.AppQName_test1_app1, 1, cluster.ProcessorKind_Command)
	borrow_work_release(istructs.AppQName_test1_app2, 1, cluster.ProcessorKind_Query)

	cancel()

	// Output:
	// test1/app1 part 1
	// - CDoc «test.verInfo» app-1 ver.1
	// test1/app2 part 1
	// - CDoc «test.verInfo» app-2 ver.1
}
