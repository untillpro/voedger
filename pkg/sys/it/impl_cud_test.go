/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */
package sys_it

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
	it "github.com/voedger/voedger/pkg/vit"
)

func TestBasicUsage_CUD(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("create", func(t *testing.T) {
		body := `
			{
				"cuds": [
					{
						"fields": {
							"sys.ID": 1,
							"sys.QName": "app1pkg.articles",
							"name": "cola",
							"article_manual": 1,
							"article_hash": 2,
							"hideonhold": 3,
							"time_active": 4,
							"control_active": 5
						}
					}
				]
			}`
		vit.PostWS(ws, "c.sys.CUD", body).Println()
	})

	var id float64
	t.Run("read using collection", func(t *testing.T) {
		body := `
		{
			"args":{
				"Schema":"app1pkg.articles"
			},
			"elements":[
				{
					"fields": ["name", "control_active", "sys.ID"]
				}
			],
			"orderBy":[{"field":"name"}]
		}`
		resp := vit.PostWS(ws, "q.sys.Collection", body)
		actualName := resp.SectionRow()[0].(string)
		actualControlActive := resp.SectionRow()[1].(float64)
		id = resp.SectionRow()[2].(float64)
		require.Equal("cola", actualName)
		require.Equal(float64(5), actualControlActive)
	})

	t.Run("update", func(t *testing.T) {
		body := fmt.Sprintf(`
		{
			"cuds": [
				{
					"sys.ID": %d,
					"fields": {
						"name": "cola1",
						"article_manual": 11,
						"article_hash": 21,
						"hideonhold": 31,
						"time_active": 41,
						"control_active": 51
					}
				}
			]
		}`, int64(id))
		vit.PostWS(ws, "c.sys.CUD", body)

		body = `
		{
			"args":{
				"Schema":"app1pkg.articles"
			},
			"elements":[
				{
					"fields": ["name", "control_active", "sys.ID"]
				}
			]
		}`
		resp := vit.PostWS(ws, "q.sys.Collection", body)
		actualName := resp.SectionRow()[0].(string)
		actualControlActive := resp.SectionRow()[1].(float64)
		newID := resp.SectionRow()[2].(float64)
		require.Equal("cola1", actualName)
		require.Equal(float64(51), actualControlActive)
		require.Equal(id, newID)

		// CDoc
		body = fmt.Sprintf(`
			{
				"args":{
					"ID": %d
				},
				"elements":[
					{
						"fields": ["Result"]
					}
				]
			}`, int64(id))
		resp = vit.PostWS(ws, "q.sys.GetCDoc", body)
		jsonBytes := []byte(resp.SectionRow()[0].(string))
		cdoc := map[string]interface{}{}
		require.NoError(json.Unmarshal(jsonBytes, &cdoc))
		log.Println(string(jsonBytes))
		log.Println(cdoc)
	})

	t.Run("404 on update unexisting", func(t *testing.T) {
		body := fmt.Sprintf(`
			{
				"cuds": [
					{
						"sys.ID": %d,
						"fields": {}
					}
				]
			}`, istructs.NonExistingRecordID)
		vit.PostWS(ws, "c.sys.CUD", body, httpu.Expect404())
	})
}

// Deprecated: use c.sys.CUD. Kept to not to break the exitsing events only
func TestBasicUsage_Init(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `
		{
			"cuds": [
				{
					"fields": {
						"sys.ID": 100000,
						"sys.QName": "app1pkg.articles",
						"name": "cola",
						"article_manual": 11,
						"article_hash": 21,
						"hideonhold": 31,
						"time_active": 41,
						"control_active": 51
					}
				}
			]
		}`
	vit.PostWSSys(ws, "c.sys.Init", body)

	body = `
		{
			"args":{
				"Schema":"app1pkg.articles"
			},
			"elements":[
				{
					"fields": ["name", "control_active", "sys.ID"]
				}
			],
			"orderBy":[{"field":"name"}]
		}`
	resp := vit.PostWS(ws, "q.sys.Collection", body)
	actualName := resp.SectionRow()[0].(string)
	actualControlActive := resp.SectionRow()[1].(float64)
	id := resp.SectionRow()[2].(float64)
	require.Equal("cola", actualName)
	require.Equal(float64(51), actualControlActive)
	require.Greater(istructs.RecordID(id), istructs.MaxRawRecordID)
}

func TestBasicUsage_Singletons(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `
		{
			"cuds": [
				{
					"fields": {
						"sys.ID": 1,
						"sys.QName": "app1pkg.Config",
						"Fld1": "42"
					}
				}
			]
		}`
	resp := vit.PostWS(ws, "c.sys.CUD", body)
	require.Empty(resp.NewIDs) // nothing passed through ID generator

	// create again -> error
	vit.PostWS(ws, "c.sys.CUD", body, httpu.Expect409()).Println()

	// query ID using collection
	body = `{
		"args":{ "Schema":"app1pkg.Config" },
		"elements":[{ "fields": ["sys.ID"] }]
	}`
	resp = vit.PostWS(ws, "q.sys.Collection", body)
	singletonID := int64(resp.SectionRow()[0].(float64))
	log.Println(singletonID)
	require.True(istructs.RecordID(singletonID) >= istructs.FirstSingletonID && istructs.RecordID(singletonID) <= istructs.MaxSingletonID)
}

func TestUnlinkReference(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	// new `options` and `department` are linked to `department_options`
	body := `
		{
			"cuds": [
				{
					"fields": {
						"sys.ID": 1,
						"sys.QName": "app1pkg.options"
					}
				},
				{
					"fields": {
						"sys.ID": 2,
						"sys.QName": "app1pkg.department",
						"pc_fix_button": 1,
						"rm_fix_button": 1
					}
				},
				{
					"fields": {
						"sys.ID": 3,
						"sys.QName": "app1pkg.department_options",
						"id_options": 1,
						"id_department": 2,
						"sys.ParentID": 2,
						"sys.Container": "department_options",
						"option_type": 1
					}
				}
			]
		}`
	resp := vit.PostWS(ws, "c.sys.CUD", body)

	// unlink department_option from options
	idDep := resp.NewIDs["2"]
	idDepOpts := resp.NewIDs["3"]
	body = fmt.Sprintf(`{"cuds": [{"sys.ID": %d, "fields": {"id_options": %d}}]}`, idDepOpts, istructs.NullRecordID)
	vit.PostWS(ws, "c.sys.CUD", body)

	// read the root department
	body = fmt.Sprintf(`{"args":{"ID": %d},"elements":[{"fields": ["Result"]}]}`, idDep)
	resp = vit.PostWS(ws, "q.sys.GetCDoc", body)
	m := map[string]interface{}{}
	require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
	require.Zero(m["department_options"].([]interface{})[0].(map[string]interface{})["id_options"].(float64))
}

func TestRefIntegrity(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	appStructs, err := vit.IAppStructsProvider.BuiltIn(istructs.AppQName_test1_app1)
	require.NoError(t, err)
	appDef := appStructs.AppDef()

	t.Run("CUDs", func(t *testing.T) {
		body := `{"cuds":[
			{"fields":{"sys.ID":1,"sys.QName":"app1pkg.cdoc1"}},
			{"fields":{"sys.ID":2,"sys.QName":"app1pkg.options"}},
			{"fields":{"sys.ID":3,"sys.QName":"app1pkg.department","pc_fix_button": 1,"rm_fix_button": 1}}
		]}`
		resp := vit.PostWS(ws, "c.sys.CUD", body)
		idCdoc1 := resp.NewIDs["1"]
		idOption := resp.NewIDs["2"]
		idDep := resp.NewIDs["3"]

		t.Run("ref to unexisting -> 400 bad request", func(t *testing.T) {
			body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID": 2,"sys.QName":"app1pkg.cdoc2","field1": %d}}]}`, istructs.NonExistingRecordID)
			vit.PostWS(ws, "c.sys.CUD", body, it.Expect400RefIntegrity_Existence())

			body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID": 2,"sys.QName":"app1pkg.cdoc2","field2": %d}}]}`, istructs.NonExistingRecordID)
			vit.PostWS(ws, "c.sys.CUD", body, it.Expect400RefIntegrity_Existence())
		})

		t.Run("ref to existing, allowed QName", func(t *testing.T) {
			body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID": 2,"sys.QName":"app1pkg.cdoc2","field1": %d}}]}`, idCdoc1)
			vit.PostWS(ws, "c.sys.CUD", body)

			body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID": 2,"sys.QName":"app1pkg.cdoc2","field2": %d}}]}`, idCdoc1)
			vit.PostWS(ws, "c.sys.CUD", body)

			body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID": 2,"sys.QName":"app1pkg.cdoc2","field2": %d}}]}`, idDep)
			vit.PostWS(ws, "c.sys.CUD", body)
		})

		t.Run("ref to existing wrong QName -> 400 bad request", func(t *testing.T) {
			body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID": 2,"sys.QName":"app1pkg.cdoc2","field2": %d}}]}`, idOption)
			vit.PostWS(ws, "c.sys.CUD", body, it.Expect400RefIntegrity_QName())
		})
	})

	t.Run("ODocs", func(t *testing.T) {
		t.Run("args", func(t *testing.T) {
			testArgsRefIntegrity(t, vit, ws, appDef, `{"args":{"sys.ID": 1,%s},"unloggedArgs":{"sys.ID":2}}`)
		})

		t.Run("unloggedArgs", func(t *testing.T) {
			testArgsRefIntegrity(t, vit, ws, appDef, `{"args":{"sys.ID": 2},"unloggedArgs":{"sys.ID":1, %s}}`)
		})
	})
}

func testArgsRefIntegrity(t *testing.T, vit *it.VIT, ws *it.AppWorkspace, app appdef.IAppDef, urlTemplate string) {
	body := `{"args":{"sys.ID": 1,"orecord1":[{"sys.ID":2,"sys.ParentID":1,"orecord2":[{"sys.ID":3,"sys.ParentID":2}]}]},"unloggedArgs":{"sys.ID":4}}`
	resp := vit.PostWS(ws, "c.app1pkg.CmdODocOne", body)
	idOdoc1 := resp.NewIDs["1"]
	idOrecord1 := resp.NewIDs["2"]
	idOrecord2 := resp.NewIDs["3"]
	body = `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.cdoc1"}}]}`
	idCDoc := vit.PostWS(ws, "c.sys.CUD", body).NewID()

	t.Run("ref to unexisting -> 400 bad request", func(t *testing.T) {
		t.Run("ODoc", func(t *testing.T) {
			oDoc := appdef.ODoc(app.Type, it.QNameODoc2)
			for _, oDoc1RefField := range oDoc.RefFields() {
				t.Run(oDoc1RefField.Name(), func(t *testing.T) {
					body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"%s":%d`, oDoc1RefField.Name(), istructs.NonExistingRecordID))
					vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400(fmt.Sprintf("record ID %d referenced by app1pkg.odoc2.%s does not exist",
						istructs.NonExistingRecordID, oDoc1RefField.Name()))).Println()
				})
			}
		})
		t.Run("ref in ORecord", func(t *testing.T) {
			oRecord := appdef.ORecord(app.Type, appdef.NewQName("app1pkg", "orecord1"))
			for _, oRecord1RefField := range oRecord.RefFields() {
				t.Run(oRecord1RefField.Name(), func(t *testing.T) {
					body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"orecord1":[{"%s":%d,"sys.ID":3,"sys.ParentID":1}]`, oRecord1RefField.Name(), istructs.NonExistingRecordID))
					vit.PostWS(ws, "c.app1pkg.CmdODocOne", body, it.Expect400(fmt.Sprintf("record ID %d referenced by app1pkg.orecord1.%s does not exist",
						istructs.NonExistingRecordID, oRecord1RefField.Name()))).Println()
				})
			}
		})
	})

	t.Run("ref to existing", func(t *testing.T) {
		t.Run("ODoc", func(t *testing.T) {
			t.Run("allowed QName", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToODoc1":%d`, idOdoc1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)
			})

			t.Run("wrong QName CDoc-> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToODoc1":%d`, idCDoc))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName()).Println()
			})

			t.Run("wrong QName ORecord -> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToODoc1":%d`, idOrecord1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName()).Println()
			})
		})
		t.Run("ORecord", func(t *testing.T) {
			t.Run("allowed QName ORecord1", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToORecord1":%d`, idOrecord1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)
			})

			t.Run("allowed QName ORecord2", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToORecord2":%d`, idOrecord2))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)
			})

			t.Run("wrong QName CDoc -> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToORecord1":%d`, idCDoc))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName()).Println()
			})

			t.Run("wrong QName ODoc ORecord1 -> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToORecord1":%d`, idOdoc1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName()).Println()
			})

			t.Run("wrong QName ODoc ORecord2 -> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToORecord2":%d`, idOdoc1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName()).Println()
			})
		})
		t.Run("Any", func(t *testing.T) {
			body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToAny":%d`, idCDoc))
			vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)

			body = fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToAny":%d`, idOdoc1))
			vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)
		})

		t.Run("CDoc", func(t *testing.T) {
			t.Run("allowed QName", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToCDoc1":%d`, idCDoc))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)
			})
			t.Run("wrong QName -> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToCDoc1":%d`, idOdoc1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName())
			})
		})

		t.Run("CDoc or ODoc", func(t *testing.T) {
			t.Run("allowed QName", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToCDoc1OrODoc1":%d`, idCDoc))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)

				body = fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToCDoc1OrODoc1":%d`, idOdoc1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body)
			})
			t.Run("wrong QName -> 400 bad request", func(t *testing.T) {
				body := fmt.Sprintf(urlTemplate, fmt.Sprintf(`"refToCDoc1OrODoc1":%d`, idOrecord1))
				vit.PostWS(ws, "c.app1pkg.CmdODocTwo", body, it.Expect400RefIntegrity_QName())
			})
		})
	})
}

// https://github.com/voedger/voedger/issues/54
func TestEraseString(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	anyAirTablePlanID := vit.GetAny("app1pkg.air_table_plan", ws)

	body := fmt.Sprintf(`{"cuds":[{"sys.ID": %d,"fields":{"name":""}}]}`, anyAirTablePlanID)
	vit.PostWS(ws, "c.sys.CUD", body)

	body = fmt.Sprintf(`{"args":{"Schema":"app1pkg.air_table_plan"},"elements":[{"fields": ["name","sys.ID"]}],"filters":[{"expr":"eq","args":{"field":"sys.ID","value":%d}}]}`, anyAirTablePlanID)
	resp := vit.PostWS(ws, "q.sys.Collection", body)

	require.Empty(t, resp.SectionRow()[0].(string))
}

func TestEraseString1(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	body := `{"cuds": [{"fields": {"sys.ID": 1,"sys.QName": "app1pkg.articles","name": "cola","article_manual": 1,"article_hash": 2,"hideonhold": 3,"time_active": 4,"control_active": 5}}]}`
	id := vit.PostWS(ws, "c.sys.CUD", body).NewID()

	body = fmt.Sprintf(`{"cuds":[{"sys.ID": %d,"fields":{"name":""}}]}`, id)
	vit.PostWS(ws, "c.sys.CUD", body)

	body = fmt.Sprintf(`{"args":{"Schema":"app1pkg.articles"},"elements":[{"fields": ["name","sys.ID"]}],"filters":[{"expr":"eq","args":{"field":"sys.ID","value":%d}}]}`, id)
	resp := vit.PostWS(ws, "q.sys.Collection", body)

	require.Empty(t, resp.SectionRow()[0].(string))
}

func TestDenyCreateNonRawIDs(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	body := fmt.Sprintf(`{"cuds": [{"fields": {"sys.ID": %d,"sys.QName": "app1pkg.options"}}]}`, istructs.FirstBaseUserWSID)
	vit.PostWS(ws, "c.sys.CUD", body, httpu.Expect400())
}

func TestSelectFromNestedTables(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	body := `{"cuds": [
		{"fields":{"sys.ID": 1,"sys.QName": "app1pkg.Root", "FldRoot": 2}},
		{"fields":{"sys.ID": 2,"sys.QName": "app1pkg.Nested", "sys.ParentID":1,"sys.Container": "Nested","FldNested":3}},
		{"fields":{"sys.ID": 3,"sys.QName": "app1pkg.Third", "Fld1": 42,"sys.ParentID":2,"sys.Container": "Third"}}
	]}`
	vit.PostWS(ws, "c.sys.CUD", body)

	t.Run("normal select", func(t *testing.T) {
		body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
			{"fields": ["FldRoot"]},
			{"path": "Nested","fields": ["FldNested"]},
			{"path": "Nested/Third","fields": ["Fld1"]}
		]}`
		resp := vit.PostWS(ws, "q.sys.Collection", body)

		actualRow := resp.Sections[0].Elements[len(resp.Sections[0].Elements)-1]
		require.EqualValues(2, actualRow[0][0][0])
		require.EqualValues(3, actualRow[1][0][0])
		require.EqualValues(42, actualRow[2][0][0])
	})

	t.Run("unknown nested table", func(t *testing.T) {
		t.Run("2nd level", func(t *testing.T) {
			body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
				{"fields": ["FldRoot"]},
				{"path": "unknownNested","fields": ["FldNested"]},
				{"path": "Nested/Third","fields": ["Fld1"]}
			]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("unknown nested table unknownNested"))
		})
		t.Run("3rd level", func(t *testing.T) {
			body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
				{"fields": ["FldRoot"]},
				{"path": "Nested","fields": ["FldNested"]},
				{"path": "Nested/unknownThird","fields": ["Fld1"]}
			]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("unknown nested table unknownThird"))

			body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
				{"fields": ["FldRoot"]},
				{"path": "Nested","fields": ["FldNested"]},
				{"path": "unknownNested/Third","fields": ["Fld1"]}
			]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("unknown nested table unknownNested"))
		})
	})

	t.Run("unknown field in nested table", func(t *testing.T) {
		t.Run("2nd level", func(t *testing.T) {
			body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
				{"fields": ["FldRoot"]},
				{"path": "Nested","fields": ["unknown"]},
				{"path": "Nested/Third","fields": ["Fld1"]}
			]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("'unknown' that is unexpected among fields of app1pkg.Nested"))
		})
		t.Run("3rd level", func(t *testing.T) {
			body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
				{"fields": ["FldRoot"]},
				{"path": "Nested","fields": ["FldNested"]},
				{"path": "Nested/Third","fields": ["unknown"]}
			]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("'unknown' that is unexpected among fields of app1pkg.Third"))
		})
	})

	t.Run("nested requested in a table that has no nested tables", func(t *testing.T) {
		t.Run("in root", func(t *testing.T) {
			// cdoc2.field1 exists but it is not a nested table
			body = `{"args":{"Schema":"app1pkg.cdoc2"},"elements": [
				{"path": "field1","fields": ["SomeField"]}
				]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("unknown nested table field1"))
		})
		t.Run("in nested", func(t *testing.T) {
			// Root.Nested.Third.Fld1 field exists but is not a nested table
			body = `{"args":{"Schema":"app1pkg.Root"},"elements": [
				{"path": "Nested/Third/Fld1","fields": ["SomeField"]}
			]}`
			vit.PostWS(ws, "q.sys.Collection", body, it.Expect400("unknown nested table Fld1"))
		})
	})
}

func TestFieldsAuthorization_OpForbidden(t *testing.T) {
	t.Skip("temporarily skipped. To be rolled back in https://github.com/voedger/voedger/issues/3199")
	logger.SetLogLevel(logger.LogLevelVerbose)
	defer logger.SetLogLevel(logger.LogLevelInfo)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("activate", func(t *testing.T) {
		body := `{"cuds": [{"fields": {"sys.ID": 1,"sys.QName": "app1pkg.DocActivateDenied"}}]}`
		id := vit.PostWS(ws, "c.sys.CUD", body).NewID()

		body = fmt.Sprintf(`{"cuds": [{"sys.ID":%d,"fields": {"sys.IsActive":true}}]}`, id)
		vit.PostWS(ws, "c.sys.CUD", body, it.Expect400("cuds[0] OperationKind_Activate", "operation forbidden"))
	})

	t.Run("deactivate", func(t *testing.T) {
		body := `{"cuds": [{"fields": {"sys.ID": 1,"sys.QName": "app1pkg.DocDeactivateDenied"}}]}`
		id := vit.PostWS(ws, "c.sys.CUD", body).NewID()

		body = fmt.Sprintf(`{"cuds": [{"sys.ID":%d,"fields": {"sys.IsActive":false}}]}`, id)
		vit.PostWS(ws, "c.sys.CUD", body, it.Expect400("cuds[0] OperationKind_Deactivate", "operation forbidden"))
	})

	t.Run("field insert", func(t *testing.T) {
		// allowed
		body := `{"cuds": [{"fields": {"sys.ID": 1,"sys.QName": "app1pkg.DocFieldInsertDenied","FldAllowed":42}}]}`
		vit.PostWS(ws, "c.sys.CUD", body)

		// denied
		body = `{"cuds": [{"fields": {"sys.ID": 1,"sys.QName": "app1pkg.DocFieldInsertDenied","FldDenied":42}}]}`
		vit.PostWS(ws, "c.sys.CUD", body, it.Expect400("cuds[0] OperationKind_Insert", "operation forbidden"))
	})

	t.Run("field update", func(t *testing.T) {
		body := `{"cuds": [{"fields": {"sys.ID": 1,"sys.QName": "app1pkg.DocFieldUpdateDenied", "FldAllowed":42,"FldDenied":43}}]}`
		id := vit.PostWS(ws, "c.sys.CUD", body).NewID()

		// allowed
		body = fmt.Sprintf(`{"cuds": [{"sys.ID":%d,"fields": {"FldAllowed":45}}]}`, id)
		vit.PostWS(ws, "c.sys.CUD", body)

		// denied
		body = fmt.Sprintf(`{"cuds": [{"sys.ID":%d,"fields": {"FldDenied":46}}]}`, id)
		vit.PostWS(ws, "c.sys.CUD", body, it.Expect400("cuds[0] OperationKind_Update", "operation forbidden"))
	})

	// note: select authorization is tested in [TestDeniedResourcesAuthorization]
}

func TestErrors(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("no QName on insert -> 400 bad request", func(t *testing.T) {
		body := `{"cuds": [{"fields": {"sys.ID": 1,"FldAllowed":42}}]}`
		vit.PostWS(ws, "c.sys.CUD", body, it.Expect400("failed to parse sys.QName"))
	})
}

func TestUnnamingInQueryResult(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.category","name":"Awesome food"}}]}`
	catID := vit.PostWS(ws, "c.sys.CUD", body).NewID()

	body = fmt.Sprintf(`{"args": {"CategoryID":%d},"elements": [{"path":"","fields": ["CategoryID"],"refs":[["CategoryID", "name"],["CategoryID","int_fld1"]]}]}`, catID)
	vit.PostWS(ws, "q.app1pkg.QryReturnsCategory", body).Println()
}

func TestExpiredTokenSkippedOnGrantExecToAnonymousFunc(t *testing.T) {
	// avoiding validating an expired token accidentally kept in cookies
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	token := ws.Owner.Token

	// expire the token
	vit.TimeAdd(24 * time.Hour)

	t.Run("command", func(t *testing.T) {
		vit.PostWS(ws, "c.app1pkg.CmdAllowedToAnonymousOnly", "{}", httpu.WithAuthorizeBy(token))
	})

	t.Run("query", func(t *testing.T) {
		t.Run("APIv1", func(t *testing.T) {
			vit.PostWS(ws, "q.app1pkg.QryAllowedToAnonymousOnly", "{}", httpu.WithAuthorizeBy(token))
		})
		t.Run("APIv2", func(t *testing.T) {
			vit.GET(fmt.Sprintf(`api/v2/apps/test1/app1/workspaces/%d/queries/app1pkg.QryAllowedToAnonymousOnly`, ws.WSID), httpu.WithAuthorizeBy(token))
		})
	})
}
