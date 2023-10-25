/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package appdef_test

import (
	"fmt"
	"strings"

	"github.com/voedger/voedger/pkg/appdef"
)

func ExampleIFieldsBuilder_AddField() {

	var app appdef.IAppDef
	docName := appdef.NewQName("test", "doc")

	// how to build doc with string field
	{
		appDef := appdef.New()

		doc := appDef.AddCDoc(docName)
		doc.
			AddField("code", appdef.DataKind_string, true, appdef.MinLen(1), appdef.MaxLen(4), appdef.Pattern(`^\d+$`)).
			SetFieldComment("code", "Code is string containing from one to four digits").
			AddField("barCode", appdef.DataKind_bytes, false, appdef.MaxLen(1024)).
			SetFieldComment("barCode", "Bar code scan data, up to 1 KB")

		if a, err := appDef.Build(); err == nil {
			app = a
		} else {
			panic(err)
		}
	}

	// how to inspect fields
	{
		doc := app.CDoc(docName)
		fmt.Printf("%v, user field count: %v\n", doc, doc.UserFieldCount())

		cnt := 0
		doc.UserFields(func(f appdef.IField) {
			cnt++
			fmt.Printf("%d. %v", cnt, f)
			if f.Required() {
				fmt.Print(", required")
			}
			if c := f.Comment(); c != "" {
				fmt.Print(". ", c)
			}
			str := []string{}
			f.Constraints(func(c appdef.IConstraint) {
				str = append(str, fmt.Sprint(c))
			})
			if len(str) > 0 {
				fmt.Println()
				fmt.Printf("  - constraints: [%v]", strings.Join(str, `, `))
			}
			fmt.Println()
		})
	}

	// Output:
	// CDoc «test.doc», user field count: 2
	// 1. string-field «code», required. Code is string containing from one to four digits
	//   - constraints: [MinLen: 1, MaxLen: 4, Pattern: `^\d+$`]
	// 2. bytes-field «barCode». Bar code scan data, up to 1 KB
	//   - constraints: [MaxLen: 1024]
}

func ExampleIFieldsBuilder_AddDataField() {

	var app appdef.IAppDef
	docName := appdef.NewQName("test", "doc")

	// how to build doc with string field
	{
		appDef := appdef.New()

		str10 := appDef.AddData(appdef.NewQName("test", "str10"), appdef.DataKind_string, appdef.NullQName, appdef.MinLen(10), appdef.MaxLen(10))
		str10.SetComment("string with 10 characters exact")
		dig10 := appDef.AddData(appdef.NewQName("test", "dig10"), appdef.DataKind_string, str10.QName(), appdef.Pattern(`^\d+$`, "only digits"))

		month := appDef.AddData(appdef.NewQName("test", "month"), appdef.DataKind_int32, appdef.NullQName, appdef.MinIncl(1))

		doc := appDef.AddCDoc(docName)
		doc.
			AddDataField("code", dig10.QName(), true).
			SetFieldComment("code", "Code is string containing 10 digits").
			AddDataField("month", month.QName(), true)

		if a, err := appDef.Build(); err == nil {
			app = a
		} else {
			panic(err)
		}
	}

	// how to inspect fields
	{
		doc := app.CDoc(docName)
		fmt.Printf("%v, user field count: %v\n", doc, doc.UserFieldCount())

		cnt := 0
		doc.UserFields(func(f appdef.IField) {
			cnt++
			fmt.Printf("%d. %v", cnt, f)
			if f.Required() {
				fmt.Print(", required")
			}
			if c := f.Comment(); c != "" {
				fmt.Print(". ", c)
			}
			str := []string{}
			f.Constraints(func(c appdef.IConstraint) {
				str = append(str, fmt.Sprint(c))
			})
			if len(str) > 0 {
				fmt.Println()
				fmt.Printf("  - constraints: [%v]", strings.Join(str, `, `))
			}
			fmt.Println()
		})

	}

	// Output:
	// CDoc «test.doc», user field count: 2
	// 1. string-field «code», required. Code is string containing 10 digits
	//   - constraints: [MinLen: 10, MaxLen: 10, Pattern: `^\d+$`]
	// 2. int32-field «month», required
	//   - constraints: [MinIncl: 1]
}
