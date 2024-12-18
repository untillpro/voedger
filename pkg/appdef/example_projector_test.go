/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef_test

import (
	"fmt"
	"slices"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/filter"
)

func ExampleProjectors() {

	var app appdef.IAppDef

	sysRecords, sysViews := appdef.NewQName(appdef.SysPackage, "records"), appdef.NewQName(appdef.SysPackage, "views")

	prjName := appdef.NewQName("test", "projector")
	recName := appdef.NewQName("test", "record")
	docName := appdef.NewQName("test", "doc")
	viewName := appdef.NewQName("test", "view")

	// how to build AppDef with projectors
	{
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		wsb := adb.AddWorkspace(appdef.NewQName("test", "workspace"))

		wsb.AddCRecord(recName).SetComment("record is trigger for projector")
		wsb.AddCDoc(docName).SetComment("doc is state for projector")

		v := wsb.AddView(viewName)
		v.Key().PartKey().AddDataField("id", appdef.SysData_RecordID)
		v.Key().ClustCols().AddDataField("name", appdef.SysData_String)
		v.Value().AddDataField("data", appdef.SysData_bytes, false, appdef.MaxLen(1024))
		v.SetComment("view is intent for projector")

		prj := wsb.AddProjector(
			prjName,
			[]appdef.OperationKind{appdef.OperationKind_Insert, appdef.OperationKind_Update, appdef.OperationKind_Activate, appdef.OperationKind_Deactivate},
			filter.QNames(recName),
			fmt.Sprintf("run projector every time when %v is changed", recName))
		prj.SetWantErrors()
		prj.States().
			Add(sysRecords, docName).
			SetComment(sysRecords, "projector needs to read «test.doc» from «sys.records» storage")
		prj.Intents().
			Add(sysViews, viewName).
			SetComment(sysViews, "projector needs to update «test.view» from «sys.views» storage")

		app = adb.MustBuild()
	}

	// how to inspect builded AppDef with projector
	{
		prj := appdef.Projector(app.Type, prjName)
		fmt.Println(prj, ":")
		fmt.Println(" - ops:", slices.Collect(prj.Ops()))
		fmt.Println(" - filter:", prj.Filter())
		if prj.WantErrors() {
			fmt.Println(" - want sys.error events")
		}
		for s := range prj.States().Enum {
			fmt.Println(" - state:", s, s.Comment())
		}
		for i := range prj.Intents().Enum {
			fmt.Println(" - intent:", i, i.Comment())
		}

		fmt.Println(appdef.Projector(app.Type, appdef.NewQName("test", "unknown")))
	}

	// How to enum all projectors in AppDef
	{
		cnt := 0
		for prj := range appdef.Projectors(app.Types()) {
			cnt++
			fmt.Println(cnt, prj)
		}
	}

	// Output:
	// BuiltIn-Projector «test.projector» :
	//  - ops: [OperationKind_Insert OperationKind_Update OperationKind_Activate OperationKind_Deactivate]
	//  - filter: QNAMES(test.record)
	//  - want sys.error events
	//  - state: Storage «sys.records» [test.doc] projector needs to read «test.doc» from «sys.records» storage
	//  - intent: Storage «sys.views» [test.view] projector needs to update «test.view» from «sys.views» storage
	// <nil>
	// 1 BuiltIn-Projector «test.projector»
}
