/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package filter_test

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/filter"
)

func ExampleQNames() {
	fmt.Println("This example demonstrates how to work with the QNames filter")

	wsName := appdef.NewQName("test", "workspace")
	doc1, doc2, doc3 := appdef.NewQName("test", "doc1"), appdef.NewQName("test", "doc2"), appdef.NewQName("test", "doc3")

	app := func() appdef.IAppDef {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		wsb := adb.AddWorkspace(wsName)

		_ = wsb.AddODoc(doc1)
		_ = wsb.AddODoc(doc2)
		_ = wsb.AddODoc(doc3)

		return adb.MustBuild()
	}()

	ws := app.Workspace(wsName)

	example := func(flt appdef.IFilter) {
		fmt.Println()
		fmt.Println("Testing", flt, "in", ws)

		for doc := range appdef.ODocs(ws.Types) {
			fmt.Println("-", doc, "is matched:", flt.Match(doc))
		}

		fmt.Println("List of all matched types from", ws, ":", flt.Matches(ws))
	}

	example(filter.QNames(doc1, doc2))
	example(filter.QNames(appdef.NewQName("test", "unknown")))

	// Output:
	// This example demonstrates how to work with the QNames filter
	//
	// Testing filter QNames [test.doc1 test.doc2] in Workspace «test.workspace»
	// - ODoc «test.doc1» is matched: true
	// - ODoc «test.doc2» is matched: true
	// - ODoc «test.doc3» is matched: false
	// List of all matched types from Workspace «test.workspace» : [ODoc «test.doc1», ODoc «test.doc2»]
	//
	// Testing filter QNames [test.unknown] in Workspace «test.workspace»
	// - ODoc «test.doc1» is matched: false
	// - ODoc «test.doc2» is matched: false
	// - ODoc «test.doc3» is matched: false
	// List of all matched types from Workspace «test.workspace» : []
}
