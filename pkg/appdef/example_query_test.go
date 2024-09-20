/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef_test

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
)

func ExampleIAppDefBuilder_AddQuery() {

	var app appdef.IAppDef

	qryName := appdef.NewQName("test", "query")
	parName := appdef.NewQName("test", "param")
	resName := appdef.NewQName("test", "res")

	// how to build AppDef with query
	{
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		qry := adb.AddQuery(qryName)
		qry.SetEngine(appdef.ExtensionEngineKind_WASM)
		qry.
			SetParam(parName).
			SetResult(resName)

		_ = adb.AddObject(parName)
		_ = adb.AddObject(resName)

		app = adb.MustBuild()
	}

	// how to enum queries
	{
		cnt := 0
		app.Queries(func(q appdef.IQuery) bool {
			cnt++
			fmt.Println(cnt, q)
			return true
		})
		fmt.Println("overall:", cnt)
	}

	// how to inspect builded AppDef with query
	{
		qry := app.Query(qryName)
		fmt.Println(qry, ":")
		fmt.Println(" - parameter:", qry.Param())
		fmt.Println(" - result   :", qry.Result())
	}

	// Output:
	// 1 WASM-Query «test.query»
	// overall: 1
	// WASM-Query «test.query» :
	//  - parameter: Object «test.param»
	//  - result   : Object «test.res»
}
