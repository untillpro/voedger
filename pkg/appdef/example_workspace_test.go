/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package appdef_test

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
)

func ExampleIWorkspace() {

	var app appdef.IAppDef
	wsName, descName, docName, recName := appdef.NewQName("test", "ws"), appdef.NewQName("test", "desc"), appdef.NewQName("test", "doc"), appdef.NewQName("test", "rec")

	// how to build AppDef with workspace
	{
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		adb.AddCDoc(descName).
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false)

		adb.AddCRecord(recName).
			AddField("r1", appdef.DataKind_int64, true).
			AddField("r2", appdef.DataKind_string, false)

		cDoc := adb.AddCDoc(docName)
		cDoc.
			AddField("d1", appdef.DataKind_int64, true).
			AddField("d2", appdef.DataKind_string, false)
		cDoc.
			AddContainer("rec", recName, 0, 100)

		adb.AddWorkspace(wsName).
			SetDescriptor(descName).
			AddType(recName).
			AddType(docName)

		if a, err := adb.Build(); err == nil {
			app = a
		} else {
			panic(err)
		}
	}

	// how to enum workspaces
	{
		cnt := 0
		app.Workspaces(func(ws appdef.IWorkspace) {
			cnt++
			fmt.Println(cnt, ws)
		})
		fmt.Println("overall:", cnt)
	}

	// how to inspect workspace
	{
		// how to find workspace by name
		ws := app.Workspace(wsName)
		fmt.Printf("workspace %q: %v\n", ws.QName(), ws.Kind())

		// how to inspect workspace
		fmt.Printf("workspace %q descriptor is %q\n", ws.QName(), ws.Descriptor())
		cnt := 0
		ws.Types(func(t appdef.IType) {
			fmt.Printf("- Type: %q, kind: %v\n", t.QName(), t.Kind())
			cnt++
		})
		fmt.Println("types count:", cnt)
	}

	// how to find workspace by descriptor
	{
		ws := app.WorkspaceByDescriptor(descName)
		fmt.Println()
		fmt.Printf("founded by descriptor %q: %v\n", descName, ws)
	}

	// Output:
	// 1 Workspace «test.ws»
	// overall: 1
	// workspace "test.ws": TypeKind_Workspace
	// workspace "test.ws" descriptor is "test.desc"
	// - Type: "test.doc", kind: TypeKind_CDoc
	// - Type: "test.rec", kind: TypeKind_CRecord
	// types count: 2
	//
	// founded by descriptor "test.desc": Workspace «test.ws»
}
