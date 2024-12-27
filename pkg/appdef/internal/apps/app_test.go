/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package apps_test

import (
	"fmt"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/builder"
	"github.com/voedger/voedger/pkg/appdef/filter"
	"github.com/voedger/voedger/pkg/goutils/testingu/require"
)

func TestNew(t *testing.T) {
	require := require.New(t)

	adb := builder.New()
	require.NotNil(adb)

	require.NotNil(adb.AppDef(), "should be ok get AppDef before build")

	app, err := adb.Build()
	require.NoError(err)
	require.NotNil(app)

	require.Equal(adb.AppDef(), app, "should be ok get AppDef after build")

	t.Run("should be ok to read sys package", func(t *testing.T) {
		require.Equal([]string{appdef.SysPackage}, slices.Collect(app.PackageLocalNames()))
		require.Equal(appdef.SysPackagePath, app.PackageFullPath(appdef.SysPackage))
		require.Equal(appdef.SysPackage, app.PackageLocalName(appdef.SysPackagePath))
	})

	t.Run("should be ok to read sys types", func(t *testing.T) {
		require.Equal(appdef.NullType, app.Type(appdef.NullQName))
		require.Equal(appdef.AnyType, app.Type(appdef.QNameANY))
	})

	t.Run("should be ok to read sys data types", func(t *testing.T) {
		require.Equal(appdef.SysData_RecordID, appdef.Data(app.Type, appdef.SysData_RecordID).QName())
		require.Equal(appdef.SysData_String, appdef.Data(app.Type, appdef.SysData_String).QName())
		require.Equal(appdef.SysData_bytes, appdef.Data(app.Type, appdef.SysData_bytes).QName())
	})
}

func testBreakable[T any](t *testing.T, name string, seq ...iter.Seq[T]) {
	for i, s := range seq {
		t.Run(fmt.Sprintf("%s[%d]", name, i), func(t *testing.T) {
			cnt := 0
			for range s {
				cnt++
				break
			}
			if cnt != 1 {
				t.Errorf("got %d iterations, expected 1", i)
			}
		})
	}
}

func testBreakable2[K, V any](t *testing.T, name string, seq ...iter.Seq2[K, V]) {
	for i, s := range seq {
		t.Run(fmt.Sprintf("%s[%d]", name, i), func(t *testing.T) {
			cnt := 0
			for range s {
				cnt++
				break
			}
			if cnt != 1 {
				t.Errorf("got %d iterations, expected 1", i)
			}
		})
	}
}

func Test_EnumsBreakable(t *testing.T) {
	require := require.New(t)

	adb := builder.New()
	adb.AddPackage("test", "test.com/test")

	wsName := appdef.NewQName("test", "workspace")
	wsb := adb.AddWorkspace(wsName)

	wsb.AddData(appdef.NewQName("test", "Data1"), appdef.DataKind_int64, appdef.NullQName)
	wsb.AddData(appdef.NewQName("test", "Data2"), appdef.DataKind_string, appdef.NullQName)

	wsb.AddGDoc(appdef.NewQName("test", "GDoc1"))
	wsb.AddGDoc(appdef.NewQName("test", "GDoc2"))
	wsb.AddGRecord(appdef.NewQName("test", "GRecord1"))
	wsb.AddGRecord(appdef.NewQName("test", "GRecord2"))

	wsb.AddCDoc(appdef.NewQName("test", "CDoc1")).
		SetSingleton()
	wsb.AddCDoc(appdef.NewQName("test", "CDoc2")).
		SetSingleton()
	wsb.AddCRecord(appdef.NewQName("test", "CRecord1"))
	wsb.AddCRecord(appdef.NewQName("test", "CRecord2"))

	wsb.AddWDoc(appdef.NewQName("test", "WDoc1")).
		SetSingleton()
	wsb.AddWDoc(appdef.NewQName("test", "WDoc2")).
		SetSingleton()
	wsb.AddWRecord(appdef.NewQName("test", "WRecord1"))
	wsb.AddWRecord(appdef.NewQName("test", "WRecord2"))

	wsb.AddODoc(appdef.NewQName("test", "ODoc1"))
	wsb.AddODoc(appdef.NewQName("test", "ODoc2"))
	wsb.AddORecord(appdef.NewQName("test", "ORecord1"))
	wsb.AddORecord(appdef.NewQName("test", "ORecord2"))

	wsb.AddObject(appdef.NewQName("test", "Object1"))
	wsb.AddObject(appdef.NewQName("test", "Object2"))

	for i := 1; i <= 2; i++ {
		v := wsb.AddView(appdef.NewQName("test", fmt.Sprintf("View%d", i)))
		v.Key().PartKey().AddField("pkf", appdef.DataKind_int64)
		v.Key().ClustCols().AddField("ccf", appdef.DataKind_string)
		v.Value().AddField("vf", appdef.DataKind_bytes, false)
	}

	cmd1Name, cmd2Name := appdef.NewQName("test", "Command1"), appdef.NewQName("test", "Command2")
	wsb.AddCommand(cmd1Name)
	wsb.AddCommand(cmd2Name)

	wsb.AddQuery(appdef.NewQName("test", "Query1"))
	wsb.AddQuery(appdef.NewQName("test", "Query2"))

	prj1name, prj2name := appdef.NewQName("test", "Projector1"), appdef.NewQName("test", "Projector2")
	prj1 := wsb.AddProjector(prj1name)
	prj1.Events().Add(
		[]appdef.OperationKind{appdef.OperationKind_Execute},
		filter.QNames(cmd1Name))
	prj1.Events().Add(
		[]appdef.OperationKind{appdef.OperationKind_ExecuteWithParam},
		filter.WSTypes(wsName, appdef.TypeKind_ODoc))
	prj2 := wsb.AddProjector(prj2name)
	prj2.Events().Add(
		[]appdef.OperationKind{appdef.OperationKind_Insert, appdef.OperationKind_Update},
		filter.Types(appdef.TypeKind_CDoc, appdef.TypeKind_CRecord))

	job1name, job2name := appdef.NewQName("test", "Job1"), appdef.NewQName("test", "Job2")
	wsb.AddJob(job1name).SetCronSchedule("@every 3s").
		States().
		Add(appdef.NewQName("test", "State1"), cmd1Name, cmd2Name).
		Add(appdef.NewQName("test", "State2"))
	wsb.AddJob(job2name).SetCronSchedule("@every 1h")

	role1Name, role2Name := appdef.NewQName("test", "Role1"), appdef.NewQName("test", "Role2")
	wsb.AddRole(role1Name).
		GrantAll(filter.QNames(cmd1Name, cmd2Name)).
		RevokeAll(filter.QNames(cmd2Name))
	wsb.AddRole(role2Name).
		GrantAll(filter.QNames(cmd1Name, cmd2Name)).
		RevokeAll(filter.QNames(cmd1Name))

	rate1Name, rate2Name := appdef.NewQName("test", "Rate1"), appdef.NewQName("test", "Rate2")
	wsb.AddRate(rate1Name, 1, time.Second, []appdef.RateScope{appdef.RateScope_AppPartition})
	wsb.AddRate(rate2Name, 2, 2*time.Second, []appdef.RateScope{appdef.RateScope_IP})
	wsb.AddLimit(appdef.NewQName("test", "Limit1"), []appdef.OperationKind{appdef.OperationKind_Execute}, appdef.LimitFilterOption_ALL, filter.QNames(cmd1Name), rate1Name)
	wsb.AddLimit(appdef.NewQName("test", "Limit2"), []appdef.OperationKind{appdef.OperationKind_Execute}, appdef.LimitFilterOption_ALL, filter.QNames(cmd2Name), rate2Name)

	app := adb.MustBuild()
	require.NotNil(app)

	t.Run("should be breakable", func(t *testing.T) {
		ws := app.Workspace(wsName)

		testBreakable2(t, "Packages", app.Packages())

		testBreakable(t, "Workspaces", app.Workspaces())

		testBreakable(t, "Types", app.Types(), ws.Types(), ws.LocalTypes())

		testBreakable(t, "DataTypes", appdef.DataTypes(app.Types()), appdef.DataTypes(ws.LocalTypes()))

		testBreakable(t, "GDocs", appdef.GDocs(app.Types()), appdef.GDocs(ws.LocalTypes()))
		testBreakable(t, "GRecords", appdef.GRecords(app.Types()), appdef.GRecords(ws.LocalTypes()))

		testBreakable(t, "CDocs", appdef.CDocs(app.Types()), appdef.CDocs(ws.LocalTypes()))
		testBreakable(t, "CRecords", appdef.CRecords(app.Types()), appdef.CRecords(ws.LocalTypes()))

		testBreakable(t, "WDocs", appdef.WDocs(app.Types()), appdef.WDocs(ws.LocalTypes()))
		testBreakable(t, "WRecords", appdef.WRecords(app.Types()), appdef.WRecords(ws.LocalTypes()))

		testBreakable(t, "Singletons", appdef.Singletons(app.Types()), appdef.Singletons(ws.LocalTypes()))

		testBreakable(t, "ODocs", appdef.ODocs(app.Types()), appdef.ODocs(ws.LocalTypes()))
		testBreakable(t, "ORecords", appdef.ORecords(app.Types()), appdef.ORecords(ws.LocalTypes()))

		testBreakable(t, "Records", appdef.Records(app.Types()), appdef.Records(ws.LocalTypes()))

		testBreakable(t, "Objects", appdef.Objects(app.Types()), appdef.Objects(ws.LocalTypes()))

		testBreakable(t, "Structures", appdef.Structures(app.Types()), appdef.Structures(ws.LocalTypes()))

		testBreakable(t, "View", appdef.Views(app.Types()), appdef.Views(ws.LocalTypes()))

		testBreakable(t, "Commands", appdef.Commands(app.Types()), appdef.Commands(ws.LocalTypes()))
		testBreakable(t, "Queries", appdef.Queries(app.Types()), appdef.Queries(ws.LocalTypes()))
		testBreakable(t, "Functions", appdef.Functions(app.Types()), appdef.Functions(ws.LocalTypes()))

		testBreakable(t, "Projectors", appdef.Projectors(app.Types()), appdef.Projectors(ws.LocalTypes()))
		testBreakable(t, "Projectors.Events", appdef.Projector(app.Type, prj1name).Events())
		for ev := range appdef.Projector(app.Type, prj2name).Events() {
			testBreakable(t, "Projectors.Events.Ops", ev.Ops())
		}

		testBreakable(t, "Jobs", appdef.Jobs(app.Types()), appdef.Jobs(ws.LocalTypes()))
		testBreakable(t, "IStorages.Enum", appdef.Job(app.Type, job1name).States().Enum)

		testBreakable(t, "Extensions", appdef.Extensions(app.Types()), appdef.Extensions(ws.LocalTypes()))

		testBreakable(t, "Roles", appdef.Roles(app.Types()), appdef.Roles(ws.LocalTypes()))
		testBreakable(t, "ACL", app.ACL(), ws.ACL(), appdef.Role(app.Type, role1Name).ACL())

		testBreakable(t, "Rates", appdef.Rates(app.Types()), appdef.Rates(ws.LocalTypes()))
		testBreakable(t, "Limits", appdef.Limits(app.Types()), appdef.Limits(ws.LocalTypes()))
	})
}

func Test_appDefBuilder_MustBuild(t *testing.T) {
	require := require.New(t)

	require.NotNil(builder.New().MustBuild(), "Should be ok if no errors in builder")

	t.Run("should panic if errors in builder", func(t *testing.T) {
		adb := builder.New()
		adb.AddWorkspace(appdef.NewQName("test", "workspace")).AddView(appdef.NewQName("test", "emptyView"))

		require.Panics(func() { _ = adb.MustBuild() },
			require.Is(appdef.ErrMissedError),
			require.Has("emptyView"),
		)
	})
}
