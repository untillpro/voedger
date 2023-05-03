/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */
package heeus_it

import (
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	airsbp_it "github.com/untillpro/airs-bp3/packages/air/it"
	"github.com/untillpro/airs-bp3/utils"
	"github.com/voedger/voedger/pkg/istructs"
	it "github.com/voedger/voedger/pkg/vit"
)

func TestBasicUsage_CUD(t *testing.T) {
	require := require.New(t)
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	t.Run("create", func(t *testing.T) {
		body := `
			{
				"cuds": [
					{
						"fields": {
							"sys.ID": 1,
							"sys.QName": "untill.articles",
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
		hit.PostWS(ws, "c.sys.CUD", body).Println()
	})

	var id float64
	t.Run("read using collection", func(t *testing.T) {
		body := `
		{
			"args":{
				"Schema":"untill.articles"
			},
			"elements":[
				{
					"fields": ["name", "control_active", "sys.ID"]
				}
			],
			"orderBy":[{"field":"name"}]
		}`
		resp := hit.PostWS(ws, "q.sys.Collection", body)
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
		hit.PostWS(ws, "c.sys.CUD", body)

		body = `
		{
			"args":{
				"Schema":"untill.articles"
			},
			"elements":[
				{
					"fields": ["name", "control_active", "sys.ID"]
				}
			]
		}`
		resp := hit.PostWS(ws, "q.sys.Collection", body)
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
		resp = hit.PostWS(ws, "q.sys.CDoc", body)
		jsonBytes := []byte(resp.SectionRow()[0].(string))
		cdoc := map[string]interface{}{}
		require.Nil(json.Unmarshal(jsonBytes, &cdoc))
		log.Println(string(jsonBytes))
		log.Println(cdoc)
	})

	t.Run("404 on update unexisting", func(t *testing.T) {
		body := `
			{
				"cuds": [
					{
						"sys.ID": 100000000001,
						"fields": {}
					}
				]
			}`
		hit.PostWS(ws, "c.sys.CUD", body, utils.Expect404())
	})
}

func TestBasicUsage_Init(t *testing.T) {
	require := require.New(t)
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	body := `
		{
			"cuds": [
				{
					"fields": {
						"sys.ID": 1000000002,
						"sys.QName": "untill.articles",
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
	hit.PostWSSys(ws, "c.sys.Init", body)

	body = `
		{
			"args":{
				"Schema":"untill.articles"
			},
			"elements":[
				{
					"fields": ["name", "control_active", "sys.ID"]
				}
			],
			"orderBy":[{"field":"name"}]
		}`
	resp := hit.PostWS(ws, "q.sys.Collection", body)
	actualName := resp.SectionRow()[0].(string)
	actualControlActive := resp.SectionRow()[1].(float64)
	id := resp.SectionRow()[2].(float64)
	require.Equal("cola", actualName)
	require.Equal(float64(51), actualControlActive)
	require.Equal(float64(1000000002), id)
}

func TestBasicUsage_Singletons(t *testing.T) {
	require := require.New(t)
	hit := it.NewHIT(t, &it.SharedConfig_Simple)
	defer hit.TearDown()

	body := `
		{
			"cuds": [
				{
					"fields": {
						"sys.ID": 1,
						"sys.QName": "test.Config",
						"Fld1": "42"
					}
				}
			]
		}`
	prn := hit.GetPrincipal(istructs.AppQName_test1_app1, "login")
	resp := hit.PostProfile(prn, "c.sys.CUD", body)
	require.Empty(resp.NewIDs) // ничего не прошло через ID generator

	// повторное создание -> ошибка
	hit.PostProfile(prn, "c.sys.CUD", body, utils.Expect409()).Println()

	// запросим ID через collection
	body = `{
		"args":{ "Schema":"test.Config" },
		"elements":[{ "fields": ["sys.ID"] }]
	}`
	resp = hit.PostProfile(prn, "q.sys.Collection", body)
	singletonID := int64(resp.SectionRow()[0].(float64))
	log.Println(singletonID)
	require.True(istructs.RecordID(singletonID) >= istructs.FirstSingletonID && istructs.RecordID(singletonID) <= istructs.MaxSingletonID)
}

func TestUnlinkReference(t *testing.T) {
	require := require.New(t)
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	body := `
		{
			"cuds": [
				{
					"fields": {
						"sys.ID": 1,
						"sys.QName": "untill.options"
					}
				},
				{
					"fields": {
						"sys.ID": 2,
						"sys.QName": "untill.department",
						"pc_fix_button": 1,
						"rm_fix_button": 1
					}
				},
				{
					"fields": {
						"sys.ID": 3,
						"sys.QName": "untill.department_options",
						"id_options": 1,
						"id_department": 2,
						"sys.ParentID": 2,
						"sys.Container": "department_options",
						"option_type": 1
					}
				}
			]
		}`
	resp := hit.PostWS(ws, "c.sys.CUD", body)

	// unlink department_option from options
	idDep := resp.NewIDs["2"]
	idDepOpts := resp.NewIDs["3"]
	body = fmt.Sprintf(`{"cuds": [{"sys.ID": %d, "fields": {"id_options": %d}}]}`, idDepOpts, istructs.NullRecordID)
	hit.PostWS(ws, "c.sys.CUD", body)

	// read the root department
	body = fmt.Sprintf(`{"args":{"ID": %d},"elements":[{"fields": ["Result"]}]}`, idDep)
	resp = hit.PostWS(ws, "q.sys.CDoc", body)
	m := map[string]interface{}{}
	require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
	require.Zero(m["department_options"].([]interface{})[0].(map[string]interface{})["id_options"].(float64))
}

func TestRefIntegrity(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()
	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	t.Run("CUDs", func(t *testing.T) {
		body := `{"cuds":[{"fields":{"sys.ID":2,"sys.QName":"untill.department","pc_fix_button": 1,"rm_fix_button": 1, "id_food_group": 123456}}]}`
		hit.PostWS(ws, "c.sys.CUD", body, utils.Expect400())
	})

	t.Run("cmd args", func(t *testing.T) {
		// InviteID arg is recordID that references an unexisting record
		body := `{"args":{"InviteID":1234567}}`
		hit.PostWS(ws, "c.sys.CancelSentInvite", body, utils.Expect400())
	})
}

// https://github.com/voedger/voedger/issues/54
func TestEraseString(t *testing.T) {
	hit := it.NewHIT(t, &it.SharedConfig_Simple)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `{"cuds":[{"sys.ID": 5000000000400,"fields":{"name":""}}]}`
	hit.PostWS(ws, "c.sys.CUD", body)

	body = `{"args":{"Schema":"untill.air_table_plan"},"elements":[{"fields": ["name","sys.ID"]}],"filters":[{"expr":"eq","args":{"field":"sys.ID","value":5000000000400}}]}`
	resp := hit.PostWS(ws, "q.sys.Collection", body)

	require.Equal(t, "", resp.SectionRow()[0].(string))
}
