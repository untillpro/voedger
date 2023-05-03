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
	"github.com/voedger/voedger/pkg/sys/sqlquery"
	it "github.com/voedger/voedger/pkg/vit"
)

func TestBasicUsage_SqlQuery(t *testing.T) {
	require := require.New(t)
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	findPLogOffsetByWLogOffset := func(wLogOffset int64) int64 {
		type row struct {
			Workspace  istructs.WSID
			PlogOffset int64
			WLogOffset int64
		}
		body := `{"args":{"Query":"select Workspace, PlogOffset, WLogOffset from sys.plog"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)
		for _, element := range resp.Sections[0].Elements {
			r := new(row)
			require.NoError(json.Unmarshal([]byte(element[0][0][0].(string)), r))
			if r.Workspace == ws.WSID && r.WLogOffset == wLogOffset {
				return r.PlogOffset
			}
		}
		panic("PlogOffset not found")
	}

	tableNum := hit.NextNumber()

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"untill.category","name":"Awesome food"}}]}`
	hit.PostWS(ws, "c.sys.CUD", body)
	body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"untill.bill","tableno":%d,"id_untill_users":100000000000,"table_part":"a","proforma":0,"working_day":"20230227"}}]}`, tableNum)
	hit.PostWS(ws, "c.sys.CUD", body)
	body = `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"untill.payments","name":"EFT","guid":"0a53b7c6-2c47-491c-ac00-307b8d5ba6f2"}}]}`
	resp := hit.PostWS(ws, "c.sys.CUD", body)

	body = fmt.Sprintf(`{"args":{"Query":"select CUDs from sys.plog where Offset>=%d"},"elements":[{"fields":["Result"]}]}`, findPLogOffsetByWLogOffset(resp.CurrentWLogOffset))
	resp = hit.PostWS(ws, "q.sys.SqlQuery", body)

	require.Contains(resp.SectionRow()[0], "0a53b7c6-2c47-491c-ac00-307b8d5ba6f2")
}

func TestSqlQuery_plog(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	pLogSize := 0
	// it is wrong to consider last resp.CurrentWLogOffset as the pLog events amount because pLog contains events from different workspaces
	// currently log of partition 0 contains events from 2 workspaces: pseudo 140737488420870 and newely created 140737488486400
	// following util shows the initial content on pLog of partition 0:
	t.Run("print the pLog content", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		for _, intf := range resp.Sections[0].Elements {
			m := map[string]interface{}{}
			require.NoError(json.Unmarshal([]byte(intf[0][0][0].(string)), &m))
			log.Println(int(m["Workspace"].(float64)), m["PlogOffset"], m["WLogOffset"])
		}
		pLogSize = len(resp.Sections[0].Elements)
	})
	// note that we have wlogOffset 7 twice, so the last resp.CurrentWLogOffset is not the amount of events in pLog
	// currently events amount is 13, the last resp.CurrentWLogOffset is 12:
	/*
		140737488420870 1 7
		140737488486400 2 1
		140737488486400 3 2
		140737488486400 4 3
		140737488486400 5 4
		140737488486400 6 5
		140737488486400 7 6
		140737488486400 8 7
		140737488486400 9 8
		140737488486400 10 9
		140737488486400 11 10
		140737488486400 12 11
		140737488486400 13 12
	*/

	for i := 1; i <= 101; i++ {
		tableno := hit.NextNumber()
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":%d,"sys.QName":"untill.bill","tableno":%d,"id_untill_users":100000000000,"table_part":"a","proforma":0,"working_day":"20230227"}}]}`, i, tableno)
		hit.PostWS(ws, "c.sys.CUD", body)
		pLogSize++
	}

	t.Run("Should read events with default Offset and limit", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(sqlquery.DefaultOffset, istructs.Offset(m["PlogOffset"].(float64)))
		require.Len(resp.Sections[0].Elements, sqlquery.DefaultLimit)
	})

	lastPLogOffset := 0

	t.Run("Should read all events", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(sqlquery.DefaultOffset, istructs.Offset(m["PlogOffset"].(float64)))
		require.Len(resp.Sections[0].Elements, pLogSize)

		m = map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)), &m))
		lastPLogOffset = int(m["PlogOffset"].(float64))
	})
	t.Run("Should read one event by limit", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog limit 1"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read one event by Offset", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.plog where Offset > %d"},"elements":[{"fields":["Result"]}]}`, lastPLogOffset-1)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(lastPLogOffset, int(m["PlogOffset"].(float64)))
		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read two events by Offset", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.plog where Offset >= %d"},"elements":[{"fields":["Result"]}]}`, lastPLogOffset-1)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 2)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(lastPLogOffset-1, int(m["PlogOffset"].(float64)))
		m = map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(1)[0].(string)), &m))
		require.Equal(lastPLogOffset, int(m["PlogOffset"].(float64)))
	})
	t.Run("Should read event with specified Offset", func(t *testing.T) {
		require := require.New(t)
		specifiedOffset := lastPLogOffset - 52
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.plog where Offset = %d"},"elements":[{"fields":["Result"]}]}`, specifiedOffset)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
		require.Contains(resp.SectionRow()[0], fmt.Sprintf(`"PlogOffset":%d`, specifiedOffset))
	})
	t.Run("Should return error when field not found in def", func(t *testing.T) {
		body := `{"args":{"Query":"select abracadabra from sys.plog"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "field 'abracadabra' not found in def")
	})
}

func TestSqlQuery_wlog(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	var lastWLogOffset int64
	for i := 1; i <= 101; i++ {
		tableno := hit.NextNumber()
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":%d,"sys.QName":"untill.bill","tableno":%d,"id_untill_users":100000000000,"table_part":"a","proforma":0,"working_day":"20230227"}}]}`, i, tableno)
		resp := hit.PostWS(ws, "c.sys.CUD", body)
		lastWLogOffset = resp.CurrentWLogOffset
	}
	wLogEventsAmount := int(lastWLogOffset)

	t.Run("Should read events with default Offset and limit", func(t *testing.T) {
		require := require.New(t)

		body := `{"args":{"Query":"select * from sys.wlog"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 100)
	})
	t.Run("Should read all events", func(t *testing.T) {
		require := require.New(t)

		body := `{"args":{"Query":"select * from sys.wlog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, wLogEventsAmount)
	})
	t.Run("Should read one event by limit", func(t *testing.T) {
		require := require.New(t)

		body := `{"args":{"Query":"select * from sys.wlog limit 1"},"elements":[{"fields":["Result"]}]}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read one event by Offset", func(t *testing.T) {
		require := require.New(t)

		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.wlog where Offset > %d"},"elements":[{"fields":["Result"]}]}`, lastWLogOffset-1)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read two events by Offset", func(t *testing.T) {
		require := require.New(t)

		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.wlog where Offset >= %d"},"elements":[{"fields":["Result"]}]}`, lastWLogOffset-1)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 2)
	})
	t.Run("Should return error when field not found in def", func(t *testing.T) {
		body := `{"args":{"Query":"select abracadabra from sys.wlog"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "field 'abracadabra' not found in def")
	})
}

func TestSqlQuery_readLogParams(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	t.Run("Should return error when limit value not parsable", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog limit 7.1"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, `strconv.ParseInt: parsing "7.1": invalid syntax`)
	})
	t.Run("Should return error when limit value invalid", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog limit -3"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "limit must be greater than -2")
	})
	t.Run("Should return error when Offset value not parsable", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where Offset >= 2.1"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, `strconv.ParseInt: parsing "2.1": invalid syntax`)
	})
	t.Run("Should return error when Offset value invalid", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where Offset >= 0"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "offset must be greater than zero")
	})
	t.Run("Should return error when Offset operation not supported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where Offset < 2"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported operation: <")
	})
	t.Run("Should return error when column name not supported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where something >= 1"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported column name: something")
	})
	t.Run("Should return error when expression not supported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.wlog where Offset >= 1 and something >= 5"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported expression: *sqlparser.AndExpr")
	})
}

func TestSqlQuery_records(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"untill.payments","name":"EFT","guid":"guidEFT"}},
					   {"fields":{"sys.ID":2,"sys.QName":"untill.payments","name":"Cash","guid":"guidCash"}},
					   {"fields":{"sys.ID":3,"sys.QName":"untill.pos_emails","description":"invite"}}]}`
	res := hit.PostWS(ws, "c.sys.CUD", body)

	eftId := res.NewID()
	cashId := res.NewIDs["2"]
	emailId := res.NewIDs["3"]

	t.Run("Should read record with all fields by ID", func(t *testing.T) {
		require := require.New(t)
		body = fmt.Sprintf(`{"args":{"Query":"select * from untill.payments where id = %d"},"elements":[{"fields":["Result"]}]}`, eftId)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		resStr := resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)
		require.Contains(resStr, `"sys.QName":"untill.payments"`)
		require.Contains(resStr, fmt.Sprintf(`"sys.ID":%d`, eftId))
		require.Contains(resStr, `"guid":"guidEFT"`)
		require.Contains(resStr, `"name":"EFT"`)
		require.Contains(resStr, `"sys.IsActive":true`)
	})
	t.Run("Should read records with one field by IDs range", func(t *testing.T) {
		require := require.New(t)
		body = fmt.Sprintf(`{"args":{"Query":"select name, sys.IsActive from untill.payments where id in (%d,%d)"}, "elements":[{"fields":["Result"]}]}`, eftId, cashId)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Equal(resp.SectionRow()[0], `{"name":"EFT","sys.IsActive":true}`)
		require.Equal(resp.SectionRow(1)[0], `{"name":"Cash","sys.IsActive":true}`)
	})
	t.Run("Should return error when column name not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments where something = 1"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported column name: something")
	})
	t.Run("Should return error when ID not parsable", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments where id = 2.3"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, `strconv.ParseInt: parsing "2.3": invalid syntax`)
	})
	t.Run("Should return error when ID from IN clause not parsable", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments where id in (1.3)"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, `strconv.ParseInt: parsing "1.3": invalid syntax`)
	})
	t.Run("Should return error when ID operation not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments where id >= 2"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported operation: >=")
	})
	t.Run("Should return error when expression not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments where id = 2 and something = 2"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported expression: *sqlparser.AndExpr")
	})
	t.Run("Should return error when ID not present", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unable to find singleton ID for definition «untill.payments»: name not found")
	})
	t.Run("Should return error when requested record has mismatching QName", func(t *testing.T) {
		body = fmt.Sprintf(`{"args":{"Query":"select * from untill.payments where id = %d"}}`, emailId)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, fmt.Sprintf("record with ID '%d' has mismatching QName 'untill.pos_emails'", emailId))
	})
	t.Run("Should return error when record not found", func(t *testing.T) {
		body = `{"args":{"Query":"select * from untill.payments where id = 123456789"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "record with ID '123456789' not found")
	})
	t.Run("Should return error when field not found in def", func(t *testing.T) {
		body = fmt.Sprintf(`{"args":{"Query":"select abracadabra from untill.pos_emails where id = %d"}}`, emailId)
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "field 'abracadabra' not found in def")
	})
	t.Run("Should read singleton", func(t *testing.T) {
		require := require.New(t)
		body = `{"args":{"Query":"select sys.QName from air.Restaurant"},"elements":[{"fields":["Result"]}]}`
		restaurant := hit.PostWS(ws, "q.sys.SqlQuery", body).SectionRow(0)

		require.Equal(`{"sys.QName":"air.Restaurant"}`, restaurant[0])
	})
}

func TestSqlQuery_view_records(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"untill.payments","name":"EFT","guid":"guidEFT"}},
					   {"fields":{"sys.ID":2,"sys.QName":"untill.pos_emails","description":"invite"}}]}`
	resp := hit.PostWS(ws, "c.sys.CUD", body)
	paymentsID := resp.NewID()
	lastWLogOffset := resp.CurrentWLogOffset

	t.Run("Should read record with all fields", func(t *testing.T) {
		require := require.New(t)
		body = `{"args":{"Query":"select * from air.CollectionView where PartKey = 1 and DocQName = 'untill.payments'"}, "elements":[{"fields":["Result"]}]}`
		resp = hit.PostWS(ws, "q.sys.SqlQuery", body)

		respStr := resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)
		require.Contains(respStr, fmt.Sprintf(`"DocID":%d`, paymentsID))
		require.Contains(respStr, `"DocQName":"untill.payments"`)
		require.Contains(respStr, `"ElementID":0`)
		require.Contains(respStr, fmt.Sprintf(`"offs":%d`, lastWLogOffset))
		require.Contains(respStr, `"PartKey":1`)
		require.Contains(respStr, `"Record":{`)
		require.Contains(respStr, `"sys.QName":"air.CollectionView_Value"`)
	})
	t.Run("Should return error when operator not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from air.CollectionView where partKey > 1"}}`
		resp = hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported operator: >")
	})
	t.Run("Should return error when expression not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from air.CollectionView where partKey = 1 or docQname = 'untill.payments'"}}`
		resp = hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported expression: *sqlparser.OrExpr")
	})
	t.Run("Should return error when field does not exist in value def", func(t *testing.T) {
		body = `{"args":{"Query":"select abracadabra from air.CollectionView where PartKey = 1"}}`
		resp = hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "field 'abracadabra' does not exist in 'air.CollectionView' value def")
	})
	t.Run("Should return error when field does not exist in key def", func(t *testing.T) {
		body = `{"args":{"Query":"select * from air.CollectionView where partKey = 1"}}`
		resp = hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "field 'partKey' does not exist in 'air.CollectionView' key def")
	})
}

func TestSqlQuery(t *testing.T) {
	hit := it.NewHIT(t, &airsbp_it.SharedConfig_Air)
	defer hit.TearDown()

	ws := hit.WS(istructs.AppQName_untill_airs_bp, "test_restaurant")

	t.Run("Should return error when script invalid", func(t *testing.T) {
		body := `{"args":{"Query":" "}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireContainsError(t, "syntax error")
	})
	t.Run("Should return error when source of data unsupported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from git.hub"}}`
		resp := hit.PostWS(ws, "q.sys.SqlQuery", body, utils.Expect500())

		resp.RequireError(t, "unsupported source: git.hub")
	})
	t.Run("Should read sys.wlog from other workspace", func(t *testing.T) {
		wsOne := hit.PostWS(ws, "q.sys.SqlQuery", fmt.Sprintf(`{"args":{"Query":"select * from sys.wlog --wsid=%d"}}`, ws.Owner.ProfileWSID))
		wsTwo := hit.PostWS(ws, "q.sys.SqlQuery", `{"args":{"Query":"select * from sys.wlog"}}`)

		require.NotEqual(t, len(wsOne.Sections[0].Elements), len(wsTwo.Sections[0].Elements))
	})
}
