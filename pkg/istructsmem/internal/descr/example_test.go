/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package descr_test

import (
	"encoding/json"
	"fmt"

	"github.com/stretchr/testify/mock"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem/internal/descr"
)

func Example() {
	appDef := func() appdef.IAppDef {
		adb := appdef.New()
		adb.AddPackage("test", "test/path")

		numName := appdef.NewQName("test", "number")
		strName := appdef.NewQName("test", "string")

		sysRecords := appdef.NewQName("sys", "records")
		sysViews := appdef.NewQName("sys", "views")

		docName, recName := appdef.NewQName("test", "doc"), appdef.NewQName("test", "rec")

		n := adb.AddData(numName, appdef.DataKind_int64, appdef.NullQName, appdef.MinIncl(1))
		n.SetComment("natural (positive) integer")

		s := adb.AddData(strName, appdef.DataKind_string, appdef.NullQName)
		s.AddConstraints(appdef.MinLen(1), appdef.MaxLen(100), appdef.Pattern(`^\w+$`, "only word characters allowed"))

		doc := adb.AddCDoc(docName)
		doc.SetSingleton()
		doc.
			AddField("f1", appdef.DataKind_int64, true).
			SetFieldComment("f1", "field comment").
			AddField("f2", appdef.DataKind_string, false, appdef.MinLen(4), appdef.MaxLen(4), appdef.Pattern(`^\w+$`)).
			AddDataField("numField", numName, false).
			AddRefField("mainChild", false, recName)
		doc.AddContainer("rec", recName, 0, 100, "container comment")
		doc.AddUnique(appdef.UniqueQName(docName, "unique1"), []appdef.FieldName{"f1", "f2"})
		doc.SetComment(`comment 1`, `comment 2`)

		rec := adb.AddCRecord(recName)
		rec.
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false).
			AddField("phone", appdef.DataKind_string, true, appdef.MinLen(1), appdef.MaxLen(25)).
			SetFieldVerify("phone", appdef.VerificationKind_Any...)
		rec.
			SetUniqueField("phone").
			AddUnique(appdef.UniqueQName(recName, "uniq1"), []appdef.FieldName{"f1"})

		viewName := appdef.NewQName("test", "view")
		view := adb.AddView(viewName)
		view.Key().PartKey().
			AddField("pk_1", appdef.DataKind_int64)
		view.Key().ClustCols().
			AddField("cc_1", appdef.DataKind_string, appdef.MaxLen(100))
		view.Value().
			AddDataField("vv_code", strName, true).
			AddRefField("vv_1", true, docName)

		objName := appdef.NewQName("test", "obj")
		obj := adb.AddObject(objName)
		obj.AddField("f1", appdef.DataKind_string, true)

		cmdName := appdef.NewQName("test", "cmd")
		adb.AddCommand(cmdName).
			SetUnloggedParam(objName).
			SetParam(objName).
			SetEngine(appdef.ExtensionEngineKind_WASM)

		queryName := appdef.NewQName("test", "query")
		adb.AddQuery(queryName).
			SetParam(objName).
			SetResult(appdef.QNameANY)

		prj := adb.AddProjector(appdef.NewQName("test", "projector"))
		prj.
			SetWantErrors().
			SetEngine(appdef.ExtensionEngineKind_WASM)
		prj.Events().
			Add(recName, appdef.ProjectorEventKind_AnyChanges...).SetComment(recName, "run projector every time when «test.rec» is changed").
			Add(cmdName).SetComment(cmdName, "run projector every time when «test.cmd» command is executed").
			Add(objName).SetComment(objName, "run projector every time when any command with «test.obj» argument is executed")
		prj.States().
			Add(sysRecords, docName, recName).SetComment(sysRecords, "needs to read «test.doc» and «test.rec» from «sys.records» storage")
		prj.Intents().
			Add(sysViews, viewName).SetComment(sysViews, "needs to update «test.view» from «sys.views» storage")

		reader := adb.AddRole(appdef.NewQName("test", "reader"))
		reader.SetComment("read-only role")
		reader.Grant(
			[]appdef.PrivilegeKind{appdef.PrivilegeKind_Select},
			[]appdef.QName{docName, recName}, []appdef.FieldName{"f1", "f2"},
			"allow reader to select some fields from test.doc and test.rec")
		reader.Grant(
			[]appdef.PrivilegeKind{appdef.PrivilegeKind_Select},
			[]appdef.QName{viewName}, nil,
			"allow reader to select all fields from test.view")
		reader.GrantAll([]appdef.QName{queryName}, "allow reader to execute test.query")

		writer := adb.AddRole(appdef.NewQName("test", "writer"))
		writer.SetComment("read-write role")
		writer.GrantAll([]appdef.QName{docName, recName, viewName}, "allow writer to do anything with test.doc, test.rec and test.view")
		writer.Revoke(
			[]appdef.PrivilegeKind{appdef.PrivilegeKind_Update},
			[]appdef.QName{docName},
			"disable writer to update test.doc")
		writer.GrantAll([]appdef.QName{cmdName, queryName}, "allow writer to execute all test functions")

		app, err := adb.Build()
		if err != nil {
			panic(err)
		}

		return app
	}()

	res := &mockResources{}
	res.
		On("Resources", mock.AnythingOfType("func(appdef.QName)")).Run(func(args mock.Arguments) {})

	appStr := &mockedAppStructs{}
	appStr.
		On("AppQName").Return(istructs.AppQName_test1_app1).
		On("AppDef").Return(appDef).
		On("Resources").Return(res)

	appLimits := map[appdef.QName]map[istructs.RateLimitKind]istructs.RateLimit{}

	app := descr.Provide(appStr, appLimits)

	json, err := json.MarshalIndent(app, "", "  ")

	fmt.Println("error:", err)
	fmt.Println(string(json))

	//os.WriteFile("C://temp//provide_test.json", json, 0644)

	// Output:
	// error: <nil>
	// {
	//   "Name": "test1/app1",
	//   "Packages": {
	//     "test": {
	//       "Path": "test/path",
	//       "DataTypes": {
	//         "test.number": {
	//           "Comment": "natural (positive) integer",
	//           "Ancestor": "sys.int64",
	//           "Constraints": {
	//             "MinIncl": 1
	//           }
	//         },
	//         "test.string": {
	//           "Ancestor": "sys.string",
	//           "Constraints": {
	//             "MaxLen": 100,
	//             "MinLen": 1,
	//             "Pattern": "^\\w+$"
	//           }
	//         }
	//       },
	//       "Structures": {
	//         "test.doc": {
	//           "Comment": "comment 1\ncomment 2",
	//           "Kind": "CDoc",
	//           "Fields": [
	//             {
	//               "Name": "sys.QName",
	//               "Data": "sys.QName",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.ID",
	//               "Data": "sys.RecordID",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.IsActive",
	//               "Data": "sys.bool"
	//             },
	//             {
	//               "Comment": "field comment",
	//               "Name": "f1",
	//               "Data": "sys.int64",
	//               "Required": true
	//             },
	//             {
	//               "Name": "f2",
	//               "DataType": {
	//                 "Ancestor": "sys.string",
	//                 "Constraints": {
	//                   "MaxLen": 4,
	//                   "MinLen": 4,
	//                   "Pattern": "^\\w+$"
	//                 }
	//               }
	//             },
	//             {
	//               "Name": "numField",
	//               "Data": "test.number"
	//             },
	//             {
	//               "Name": "mainChild",
	//               "Data": "sys.RecordID",
	//               "Refs": [
	//                 "test.rec"
	//               ]
	//             }
	//           ],
	//           "Containers": [
	//             {
	//               "Comment": "container comment",
	//               "Name": "rec",
	//               "Type": "test.rec",
	//               "MinOccurs": 0,
	//               "MaxOccurs": 100
	//             }
	//           ],
	//           "Uniques": {
	//             "test.doc$uniques$unique1": {
	//               "Fields": [
	//                 "f1",
	//                 "f2"
	//               ]
	//             }
	//           },
	//           "Singleton": true
	//         },
	//         "test.obj": {
	//           "Kind": "Object",
	//           "Fields": [
	//             {
	//               "Name": "sys.QName",
	//               "Data": "sys.QName",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.Container",
	//               "Data": "sys.string"
	//             },
	//             {
	//               "Name": "f1",
	//               "Data": "sys.string",
	//               "Required": true
	//             }
	//           ]
	//         },
	//         "test.rec": {
	//           "Kind": "CRecord",
	//           "Fields": [
	//             {
	//               "Name": "sys.QName",
	//               "Data": "sys.QName",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.ID",
	//               "Data": "sys.RecordID",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.ParentID",
	//               "Data": "sys.RecordID",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.Container",
	//               "Data": "sys.string",
	//               "Required": true
	//             },
	//             {
	//               "Name": "sys.IsActive",
	//               "Data": "sys.bool"
	//             },
	//             {
	//               "Name": "f1",
	//               "Data": "sys.int64",
	//               "Required": true
	//             },
	//             {
	//               "Name": "f2",
	//               "Data": "sys.string"
	//             },
	//             {
	//               "Name": "phone",
	//               "DataType": {
	//                 "Ancestor": "sys.string",
	//                 "Constraints": {
	//                   "MaxLen": 25,
	//                   "MinLen": 1
	//                 }
	//               },
	//               "Required": true,
	//               "Verifiable": true
	//             }
	//           ],
	//           "Uniques": {
	//             "test.rec$uniques$uniq1": {
	//               "Fields": [
	//                 "f1"
	//               ]
	//             }
	//           },
	//           "UniqueField": "phone"
	//         }
	//       },
	//       "Views": {
	//         "test.view": {
	//           "Key": {
	//             "Partition": [
	//               {
	//                 "Name": "pk_1",
	//                 "Data": "sys.int64",
	//                 "Required": true
	//               }
	//             ],
	//             "ClustCols": [
	//               {
	//                 "Name": "cc_1",
	//                 "DataType": {
	//                   "Ancestor": "sys.string",
	//                   "Constraints": {
	//                     "MaxLen": 100
	//                   }
	//                 }
	//               }
	//             ]
	//           },
	//           "Value": [
	//             {
	//               "Name": "sys.QName",
	//               "Data": "sys.QName",
	//               "Required": true
	//             },
	//             {
	//               "Name": "vv_code",
	//               "Data": "test.string",
	//               "Required": true
	//             },
	//             {
	//               "Name": "vv_1",
	//               "Data": "sys.RecordID",
	//               "Required": true,
	//               "Refs": [
	//                 "test.doc"
	//               ]
	//             }
	//           ]
	//         }
	//       },
	//       "Extensions": {
	//         "Commands": {
	//           "test.cmd": {
	//             "Name": "cmd",
	//             "Engine": "WASM",
	//             "Arg": "test.obj",
	//             "UnloggedArg": "test.obj"
	//           }
	//         },
	//         "Queries": {
	//           "test.query": {
	//             "Name": "query",
	//             "Engine": "BuiltIn",
	//             "Arg": "test.obj",
	//             "Result": "sys.ANY"
	//           }
	//         },
	//         "Projectors": {
	//           "test.projector": {
	//             "Name": "projector",
	//             "Engine": "WASM",
	//             "Events": {
	//               "test.cmd": {
	//                 "Comment": "run projector every time when «test.cmd» command is executed",
	//                 "Kind": [
	//                   "Execute"
	//                 ]
	//               },
	//               "test.obj": {
	//                 "Comment": "run projector every time when any command with «test.obj» argument is executed",
	//                 "Kind": [
	//                   "ExecuteWithParam"
	//                 ]
	//               },
	//               "test.rec": {
	//                 "Comment": "run projector every time when «test.rec» is changed",
	//                 "Kind": [
	//                   "Insert",
	//                   "Update",
	//                   "Activate",
	//                   "Deactivate"
	//                 ]
	//               }
	//             },
	//             "WantErrors": true,
	//             "States": {
	//               "sys.records": [
	//                 "test.doc",
	//                 "test.rec"
	//               ]
	//             },
	//             "Intents": {
	//               "sys.views": [
	//                 "test.view"
	//               ]
	//             }
	//           }
	//         }
	//       },
	//       "Roles": {
	//         "test.reader": {
	//           "Comment": "read-only role",
	//           "Privileges": [
	//             {
	//               "Comment": "allow reader to select some fields from test.doc and test.rec",
	//               "Access": "grant",
	//               "Kinds": [
	//                 "Select"
	//               ],
	//               "On": [
	//                 "test.doc",
	//                 "test.rec"
	//               ],
	//               "Fields": [
	//                 "f1",
	//                 "f2"
	//               ]
	//             },
	//             {
	//               "Comment": "allow reader to select all fields from test.view",
	//               "Access": "grant",
	//               "Kinds": [
	//                 "Select"
	//               ],
	//               "On": [
	//                 "test.view"
	//               ]
	//             },
	//             {
	//               "Comment": "allow reader to execute test.query",
	//               "Access": "grant",
	//               "Kinds": [
	//                 "Execute"
	//               ],
	//               "On": [
	//                 "test.query"
	//               ]
	//             }
	//           ]
	//         },
	//         "test.writer": {
	//           "Comment": "read-write role",
	//           "Privileges": [
	//             {
	//               "Comment": "allow writer to do anything with test.doc, test.rec and test.view",
	//               "Access": "grant",
	//               "Kinds": [
	//                 "Insert",
	//                 "Update",
	//                 "Select"
	//               ],
	//               "On": [
	//                 "test.doc",
	//                 "test.rec",
	//                 "test.view"
	//               ]
	//             },
	//             {
	//               "Comment": "disable writer to update test.doc",
	//               "Access": "revoke",
	//               "Kinds": [
	//                 "Update"
	//               ],
	//               "On": [
	//                 "test.doc"
	//               ]
	//             },
	//             {
	//               "Comment": "allow writer to execute all test functions",
	//               "Access": "grant",
	//               "Kinds": [
	//                 "Execute"
	//               ],
	//               "On": [
	//                 "test.cmd",
	//                 "test.query"
	//               ]
	//             }
	//           ]
	//         }
	//       }
	//     }
	//   }
	// }
}

type mockedAppStructs struct {
	istructs.IAppStructs
	mock.Mock
}

func (s *mockedAppStructs) AppDef() appdef.IAppDef {
	return s.Called().Get(0).(appdef.IAppDef)
}

func (s *mockedAppStructs) AppQName() appdef.AppQName {
	return s.Called().Get(0).(appdef.AppQName)
}

func (s *mockedAppStructs) Resources() istructs.IResources {
	return s.Called().Get(0).(istructs.IResources)
}

type mockResources struct {
	istructs.IResources
	mock.Mock
}

func (r *mockResources) Resources(cb func(appdef.QName)) {
	r.Called(cb)
}
