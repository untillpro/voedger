/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sys_it

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/iauthnzimpl"
	"github.com/voedger/voedger/pkg/istructs"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/registry"
	"github.com/voedger/voedger/pkg/sys/sqlquery"
	it "github.com/voedger/voedger/pkg/vit"
	sys_test_template "github.com/voedger/voedger/pkg/vit/testdata"
	"github.com/voedger/voedger/pkg/vvm"
)

func TestBasicUsage_SqlQuery(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	idUntillUsers := vit.GetAny("app1pkg.untill_users", ws)

	findPLogOffsetByWLogOffset := func(wLogOffset istructs.Offset) istructs.Offset {
		type row struct {
			Workspace  istructs.WSID
			PlogOffset istructs.Offset
			WLogOffset istructs.Offset
		}
		body := `{"args":{"Query":"select Workspace, PlogOffset, WLogOffset from sys.plog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		for _, element := range resp.Sections[0].Elements {
			r := new(row)
			require.NoError(json.Unmarshal([]byte(element[0][0][0].(string)), r))
			if r.Workspace == ws.WSID && r.WLogOffset == wLogOffset {
				return r.PlogOffset
			}
		}
		panic("PlogOffset not found")
	}

	tableNum := vit.NextNumber()

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.category","name":"Awesome food"}}]}`
	vit.PostWS(ws, "c.sys.CUD", body)
	body = fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.bill","tableno":%d,"id_untill_users":%d,"table_part":"a","proforma":0,"working_day":"20230227"}}]}`, tableNum, idUntillUsers)
	vit.PostWS(ws, "c.sys.CUD", body)
	body = `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.payments","name":"EFT","guid":"0a53b7c6-2c47-491c-ac00-307b8d5ba6f2"}}]}`
	resp := vit.PostWS(ws, "c.sys.CUD", body)

	body = fmt.Sprintf(`{"args":{"Query":"select CUDs from sys.plog where Offset>=%d"},"elements":[{"fields":["Result"]}]}`, findPLogOffsetByWLogOffset(resp.CurrentWLogOffset))
	resp = vit.PostWS(ws, "q.sys.SqlQuery", body)

	require.Contains(resp.SectionRow()[0], "0a53b7c6-2c47-491c-ac00-307b8d5ba6f2")
}

func TestSqlQuery_plog(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	idUntillUsers := vit.GetAny("app1pkg.untill_users", ws)

	pLogSize := 0
	// it is wrong to consider last resp.CurrentWLogOffset as the pLog events amount because pLog contains events from different workspaces
	// currently log of partition 0 contains events from 2 workspaces: pseudo 140737488420870 and newely created 140737488486400
	// following util shows the initial content on pLog of partition 0:
	t.Run("print the pLog content", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

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
		tableno := vit.NextNumber()
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":%d,"sys.QName":"app1pkg.bill","tableno":%d,"id_untill_users":%d,"table_part":"a","proforma":0,"working_day":"20230227"}}]}`, i, tableno, idUntillUsers)
		vit.PostWS(ws, "c.sys.CUD", body)
		pLogSize++
	}

	t.Run("Should read events with default Offset and limit", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(istructs.FirstOffset, istructs.Offset(m["PlogOffset"].(float64)))
		require.Len(resp.Sections[0].Elements, sqlquery.DefaultLimit)
	})

	lastPLogOffset := 0

	t.Run("Should read all events", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(istructs.FirstOffset, istructs.Offset(m["PlogOffset"].(float64)))
		require.GreaterOrEqual(len(resp.Sections[0].Elements), pLogSize)

		m = map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)), &m))
		lastPLogOffset = int(m["PlogOffset"].(float64))

	})
	t.Run("Should read one event by limit", func(t *testing.T) {
		require := require.New(t)
		body := `{"args":{"Query":"select * from sys.plog limit 1"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read one event by Offset", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.plog where Offset > %d"},"elements":[{"fields":["Result"]}]}`, lastPLogOffset-1)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow()[0].(string)), &m))
		require.Equal(lastPLogOffset, int(m["PlogOffset"].(float64)))
		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read two events by Offset", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.plog where Offset >= %d"},"elements":[{"fields":["Result"]}]}`, lastPLogOffset-1)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

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
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
		require.Contains(resp.SectionRow()[0], fmt.Sprintf(`"PlogOffset":%d`, specifiedOffset))
	})

	t.Run("select operation is allowed only", func(t *testing.T) {
		body := `{"args":{"Query":"update sys.plog set a = 1"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("'select' operation is expected"))
	})
}

func TestSqlQuery_wlog(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	idUntillUsers := vit.GetAny("app1pkg.untill_users", ws)

	var lastWLogOffset istructs.Offset
	for i := 1; i <= 101; i++ {
		tableno := vit.NextNumber()
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":%d,"sys.QName":"app1pkg.bill","tableno":%d,"id_untill_users":%d,"table_part":"a","proforma":0,"working_day":"20230227"}}]}`, i, tableno, idUntillUsers)
		resp := vit.PostWS(ws, "c.sys.CUD", body)
		lastWLogOffset = resp.CurrentWLogOffset
	}
	wLogEventsAmount := int(lastWLogOffset)

	t.Run("Should read events with default Offset and limit", func(t *testing.T) {
		require := require.New(t)

		body := `{"args":{"Query":"select * from sys.wlog"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 100)
	})
	t.Run("Should read all events", func(t *testing.T) {
		require := require.New(t)

		body := `{"args":{"Query":"select * from sys.wlog limit -1"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, wLogEventsAmount)
	})
	t.Run("Should read one event by limit", func(t *testing.T) {
		require := require.New(t)

		body := `{"args":{"Query":"select * from sys.wlog limit 1"},"elements":[{"fields":["Result"]}]}`
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read one event by Offset", func(t *testing.T) {
		require := require.New(t)

		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.wlog where Offset > %d"},"elements":[{"fields":["Result"]}]}`, lastWLogOffset-1)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 1)
	})
	t.Run("Should read two events by Offset", func(t *testing.T) {
		require := require.New(t)

		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.wlog where Offset >= %d"},"elements":[{"fields":["Result"]}]}`, lastWLogOffset-1)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.Len(resp.Sections[0].Elements, 2)
	})
	t.Run("Should return error when field not found in def", func(t *testing.T) {
		body := `{"args":{"Query":"select abracadabra from sys.wlog"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("field 'abracadabra' not found in def"))
	})
}

func TestSqlQuery_readLogParams(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("Should return error when limit value not parsable", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog limit 7.1"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500(`strconv.ParseInt: parsing "7.1": invalid syntax`))
	})
	t.Run("Should return error when limit value invalid", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog limit -3"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500("limit must be greater than -2"))
	})
	t.Run("Should return error when Offset value not parsable", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where Offset >= 2.1"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500(`strconv.ParseUint: parsing "2.1": invalid syntax`))
	})
	t.Run("Should return error when Offset value invalid", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where Offset >= 0"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500("offset must be greater than zero"))
	})
	t.Run("Should return error when Offset operation not supported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where Offset < 2"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500("unsupported operation: <"))
	})
	t.Run("Should return error when column name not supported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.plog where something >= 1"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500("unsupported column name: something"))
	})
	t.Run("Should return error when expression not supported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from sys.wlog where Offset >= 1 and something >= 5"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect500("unsupported expression: *sqlparser.AndExpr"))
	})
}

func TestSqlQuery_records(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.payments","name":"EFT","guid":"guidEFT"}},
					   {"fields":{"sys.ID":2,"sys.QName":"app1pkg.payments","name":"Cash","guid":"guidCash"}},
					   {"fields":{"sys.ID":3,"sys.QName":"app1pkg.pos_emails","description":"invite"}}]}`
	res := vit.PostWS(ws, "c.sys.CUD", body)

	eftID := res.NewID()
	cashID := res.NewIDs["2"]
	emailID := res.NewIDs["3"]

	t.Run("Should read record with all fields by ID", func(t *testing.T) {
		require := require.New(t)
		body = fmt.Sprintf(`{"args":{"Query":"select * from app1pkg.payments where id = %d"},"elements":[{"fields":["Result"]}]}`, eftID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		resStr := resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)
		require.Contains(resStr, `"sys.QName":"app1pkg.payments"`)
		require.Contains(resStr, fmt.Sprintf(`"sys.ID":%d`, eftID))
		require.Contains(resStr, `"guid":"guidEFT"`)
		require.Contains(resStr, `"name":"EFT"`)
		require.Contains(resStr, `"sys.IsActive":true`)
	})
	t.Run("Should read records with one field by IDs range", func(t *testing.T) {
		require := require.New(t)
		body = fmt.Sprintf(`{"args":{"Query":"select name, sys.IsActive from app1pkg.payments where id in (%d,%d)"}, "elements":[{"fields":["Result"]}]}`, eftID, cashID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)

		require.JSONEq(`{"name":"EFT","sys.IsActive":true}`, resp.SectionRow()[0].(string))
		require.JSONEq(`{"name":"Cash","sys.IsActive":true}`, resp.SectionRow(1)[0].(string))
	})

	t.Run("errors", func(t *testing.T) {
		cases := map[string]string{
			`{"args":{"Query":"select * from app1pkg.payments where something = 1"}}`:                                      "unsupported column name: something",
			`{"args":{"Query":"select * from app1pkg.payments where id = 2.3"}}`:                                           `parsing "2.3": invalid syntax`,
			`{"args":{"Query":"select * from app1pkg.payments where id in (1.3)"}}`:                                        `parsing "1.3": invalid syntax`,
			`{"args":{"Query":"select * from app1pkg.payments where id >= 2"}}`:                                            "unsupported operation: >=",
			`{"args":{"Query":"select * from app1pkg.payments where id = 2 and something = 2"}}`:                           "unsupported expression: *sqlparser.AndExpr",
			`{"args":{"Query":"select * from app1pkg.payments"}}`:                                                          "'app1pkg.payments' is not a singleton. At least one record ID must be provided",
			fmt.Sprintf(`{"args":{"Query":"select * from app1pkg.payments where id = %d"}}`, emailID):                      fmt.Sprintf("record with ID '%d' has mismatching QName 'app1pkg.pos_emails'", emailID),
			fmt.Sprintf(`{"args":{"Query":"select * from app1pkg.payments where id = %d"}}`, istructs.NonExistingRecordID): fmt.Sprintf("record with ID '%d' not found", istructs.NonExistingRecordID),
			fmt.Sprintf(`{"args":{"Query":"select abracadabra from app1pkg.pos_emails where id = %d"}}`, emailID):          "not found: field «abracadabra» in CDoc «app1pkg.pos_emails»",
			`{"args":{"Query":"select * from app1pkg.payments.2 where id = 2"}}`:                                           "record ID and 'where id ...' clause can not be used in one query",
			`{"args":{"Query":"select sys.QName from app1pkg.test_ws.1"}}`:                                                 "conditions are not allowed to query a singleton",
			`{"args":{"Query":"select sys.QName from app1pkg.test_ws where id = 1"}}`:                                      "conditions are not allowed to query a singleton",
			fmt.Sprintf(`{"args":{"Query":"select name, name from app1pkg.payments where id = %d"}}`, eftID):               `field "name" is selected more than once`,
		}

		for query, expectedError := range cases {
			t.Run(expectedError, func(t *testing.T) {
				vit.PostWS(ws, "q.sys.SqlQuery", query, it.Expect400(expectedError))
			})
		}
	})

	t.Run("Should read singleton", func(t *testing.T) {
		require := require.New(t)
		body = `{"args":{"Query":"select sys.QName from app1pkg.test_ws"},"elements":[{"fields":["Result"]}]}`
		restaurant := vit.PostWS(ws, "q.sys.SqlQuery", body).SectionRow(0)

		require.JSONEq(`{"sys.QName":"app1pkg.test_ws"}`, restaurant[0].(string))
	})
}

func TestSqlQuery_view_records(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.payments","name":"EFT","guid":"guidEFT"}},
					   {"fields":{"sys.ID":2,"sys.QName":"app1pkg.pos_emails","description":"invite"}}]}`
	resp := vit.PostWS(ws, "c.sys.CUD", body)
	paymentsID := resp.NewID()
	lastWLogOffset := resp.CurrentWLogOffset

	t.Run("Should read record with all fields", func(t *testing.T) {
		require := require.New(t)
		body = `{"args":{"Query":"select * from sys.CollectionView where PartKey = 1 and DocQName = 'app1pkg.payments'"}, "elements":[{"fields":["Result"]}]}`
		resp = vit.PostWS(ws, "q.sys.SqlQuery", body)

		respStr := resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)
		require.Contains(respStr, fmt.Sprintf(`"DocID":%d`, paymentsID))
		require.Contains(respStr, `"DocQName":"app1pkg.payments"`)
		require.Contains(respStr, `"ElementID":0`)
		require.Contains(respStr, fmt.Sprintf(`"offs":%d`, lastWLogOffset))
		require.Contains(respStr, `"PartKey":1`)
		require.Contains(respStr, `"Record":{`)
		require.Contains(respStr, `"sys.QName":"sys.CollectionView"`)
	})
	t.Run("Should return error when operator not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from sys.CollectionView where partKey > 1"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("unsupported operator: >"))
	})
	t.Run("Should return error when expression not supported", func(t *testing.T) {
		body = `{"args":{"Query":"select * from sys.CollectionView where partKey = 1 or docQname = 'app1pkg.payments'"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("unsupported expression: *sqlparser.OrExpr"))
	})
	t.Run("Should return error when field does not exist in value def", func(t *testing.T) {
		body = `{"args":{"Query":"select abracadabra from sys.CollectionView where PartKey = 1"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("field 'abracadabra' does not exist in 'sys.CollectionView' value def"))
	})
	t.Run("Should recover lowercased table and field names", func(t *testing.T) {
		require := require.New(t)
		body = `{"args":{"Query":"select docqname from sys.collectionview where PartKey = 1 and DocQName = 'app1pkg.payments'"}, "elements":[{"fields":["Result"]}]}`
		resp = vit.PostWS(ws, "q.sys.SqlQuery", body)
		require.NotEmpty(resp.Sections[0].Elements)
	})
}

func TestSqlQuery(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("Should return error when script invalid", func(t *testing.T) {
		body := `{"args":{"Query":" "}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("invalid query format"))
	})
	t.Run("Should return error when source of data unsupported", func(t *testing.T) {
		body := `{"args":{"Query":"select * from git.hub"}}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("do not know how to read from the requested git.hub"))
	})
	t.Run("Should read sys.wlog from other workspace", func(t *testing.T) {
		wsOne := vit.PostWS(ws, "q.sys.SqlQuery", fmt.Sprintf(`{"args":{"Query":"select * from %d.sys.wlog"}}`, ws.Owner.ProfileWSID))
		wsTwo := vit.PostWS(ws, "q.sys.SqlQuery", `{"args":{"Query":"select * from sys.wlog"}}`)

		require.NotEqual(t, len(wsOne.Sections[0].Elements), len(wsTwo.Sections[0].Elements))
	})

	t.Run("403 forbidden on read from non-inited workspace", func(t *testing.T) {
		vit.PostWS(ws, "q.sys.SqlQuery", fmt.Sprintf(`{"args":{"Query":"select * from %d.sys.wlog"}}`, istructs.NonExistingRecordID),
			it.Expect403(processors.ErrWSNotInited.Message))
	})
}

func TestReadFromWLogWithSysRawArg(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	lastOffset := vit.PostWS(ws, "c.app1pkg.TestCmdRawArg", "hello world").CurrentWLogOffset

	body := fmt.Sprintf(`{"args":{"Query":"select * from sys.wlog where Offset > %d"},"elements":[{"fields":["Result"]}]}`, lastOffset-1)
	resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
	res := resp.SectionRow()[0].(string)
	m := map[string]interface{}{}
	require.NoError(json.Unmarshal([]byte(res), &m))
	rawArg := m["ArgumentObject"].(map[string]interface{})["Body"].(string)
	require.Equal("hello world", rawArg)
}

func TestReadFromAnDifferentLocations(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	oneAppWS := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	t.Run("wsid", func(t *testing.T) {
		// create a record in one workspace of one app
		categoryName := vit.NextName()
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.category","name":"%s"}}]}`, categoryName)
		categoryID := vit.PostWS(oneAppWS, "c.sys.CUD", body).NewID()

		// create a workspace in another app
		anotherAppWSOwner := vit.GetPrincipal(istructs.AppQName_test1_app2, "login")
		qNameApp2_TestWSKind := appdef.NewQName("app2pkg", "test_ws")
		anotherAppWSName := vit.NextName()
		anotherAppWS := vit.CreateWorkspace(it.WSParams{
			Name:         anotherAppWSName,
			Kind:         qNameApp2_TestWSKind,
			ClusterID:    istructs.CurrentClusterID(),
			InitDataJSON: `{"IntFld":42}`,
		}, anotherAppWSOwner)

		// in the another app use sql to query the record from the first app
		body = fmt.Sprintf(`{"args":{"Query":"select * from test1.app1.%d.app1pkg.category.%d"},"elements":[{"fields":["Result"]}]}`, oneAppWS.WSID, categoryID)
		resp := vit.PostWS(anotherAppWS, "q.sys.SqlQuery", body)
		resStr := resp.SectionRow(len(resp.Sections[0].Elements) - 1)[0].(string)
		require.Contains(resStr, fmt.Sprintf(`"name":"%s"`, categoryName))
	})

	t.Run("app workspace number", func(t *testing.T) {
		// determine the number of the app workspace that stores cdoc.Login "login"
		registryAppStructs, err := vit.IAppStructsProvider.BuiltIn(istructs.AppQName_sys_registry)
		require.NoError(err)
		prn := vit.GetPrincipal(istructs.AppQName_test1_app1, "login") // from VIT shared config
		pseudoWSID := coreutils.GetPseudoWSID(istructs.NullWSID, prn.Name, istructs.CurrentClusterID())
		appWSNumber := coreutils.AppWSNumber(pseudoWSID, registryAppStructs.NumAppWorkspaces())

		// for example read cdoc.registry.Login.LoginHash from the app workspace
		loginID := vit.GetCDocLoginID(prn.Login)
		// request to the different app -> use sys token to avoid 403
		sysPrincipal := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.registry.a%d.registry.Login where id = %d"},"elements":[{"fields":["Result"]}]}`, appWSNumber, loginID)
		resp := vit.PostWS(oneAppWS, "q.sys.SqlQuery", body, httpu.WithAuthorizeBy(sysPrincipal.Token))
		loginHash := registry.GetLoginHash(prn.Login.Name)
		require.Contains(resp.SectionRow()[0].(string), fmt.Sprintf(`"LoginHash":"%s"`, loginHash))
	})

	t.Run("login hash", func(t *testing.T) {
		// for example read cdoc.registry.Login.LoginHash from the app workspace determined by the login name
		prn := vit.GetPrincipal(istructs.AppQName_test1_app1, "login") // from VIT shared config
		loginID := vit.GetCDocLoginID(prn.Login)
		// request to the different app -> use sys token to avoid 403
		sysPrincipal := vit.GetSystemPrincipal(istructs.AppQName_test1_app1)
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.registry.\"login\".registry.Login where id = %d"},"elements":[{"fields":["Result"]}]}`, loginID)
		resp := vit.PostWS(oneAppWS, "q.sys.SqlQuery", body, httpu.WithAuthorizeBy(sysPrincipal.Token))
		loginHash := registry.GetLoginHash(prn.Login.Name)
		require.Contains(resp.SectionRow()[0].(string), fmt.Sprintf(`"LoginHash":"%s"`, loginHash))
	})

	t.Run("query forwarding", func(t *testing.T) {
		wsAnother := vit.WS(istructs.AppQName_test1_app1, "test_ws_another")
		ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
		body := fmt.Sprintf(`{"args":{"Query":"select * from %d.sys.wlog"},"elements":[{"fields":["Result"]}]}`, ws.WSID)
		resp := vit.PostWS(wsAnother, "q.sys.SqlQuery", body)
		require.GreaterOrEqual(resp.NumRows(), 2)
		resp.Println()
	})

	t.Run("query forwarding with empty result", func(t *testing.T) {
		wsAnother := vit.WS(istructs.AppQName_test1_app1, "test_ws_another")
		ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
		body := fmt.Sprintf(`{"args":{"Query":"select * from %d.sys.wlog where offset = %d"},"elements":[{"fields":["Result"]}]}`, ws.WSID, istructs.NonExistingRecordID)
		resp := vit.PostWS(wsAnother, "q.sys.SqlQuery", body)
		require.True(resp.IsEmpty())
	})
}

func TestAuthnz(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	// issues api token as token that has no ability to read our odoc and orecord
	apiRole := appdef.NewQName("app1pkg", "ApiRole")
	as, err := vit.IAppStructsProvider.BuiltIn(istructs.AppQName_test1_app1)
	require.NoError(t, err)
	apiToken, err := iauthnzimpl.IssueAPIToken(as.AppTokens(), time.Hour, []appdef.QName{apiRole},
		ws.WSID, payloads.PrincipalPayload{
			Login:       ws.Owner.Name,
			SubjectKind: istructs.SubjectKind_User,
			ProfileWSID: ws.Owner.ProfileWSID,
		})
	require.NoError(t, err)

	t.Run("foreign app", func(t *testing.T) {
		loginID := vit.GetCDocLoginID(ws.Owner.Login)
		registryAppStructs, err := vit.IAppStructsProvider.BuiltIn(istructs.AppQName_sys_registry)
		require := require.New(t)
		require.NoError(err)
		pseudoWSID := coreutils.GetPseudoWSID(istructs.NullWSID, ws.Owner.Name, istructs.CurrentClusterID())
		appWSNumber := coreutils.AppWSNumber(pseudoWSID, registryAppStructs.NumAppWorkspaces())
		body := fmt.Sprintf(`{"args":{"Query":"select * from sys.registry.a%d.registry.Login where id = %d"},"elements":[{"fields":["Result"]}]}`, appWSNumber, loginID)
		vit.PostWS(ws, "q.sys.SqlQuery", body, httpu.Expect403())
	})

	t.Run("doc", func(t *testing.T) {
		body := `{"args":{"Query":"select * from app1pkg.TestDeniedCDoc.123"},"elements":[{"fields":["Result"]}]}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, httpu.Expect403())
	})

	t.Run("field", func(t *testing.T) {
		// denied
		body := `{"args":{"Query":"select DeniedFld2 from app1pkg.TestCDocWithDeniedFields.123"},"elements":[{"fields":["Result"]}]}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, httpu.Expect403())

		// allowed, just expect 400 not found
		body = `{"args":{"Query":"select Fld1 from app1pkg.TestCDocWithDeniedFields.123"},"elements":[{"fields":["Result"]}]}`
		vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400("record with ID '123' not found"))
	})

	// create odoc and orecord
	body := `{"args":{"sys.ID":1,"odocIntFld":42,"orecord1":[{"sys.ID":2,"sys.ParentID":1,"orecord1IntFld":43}]},"unloggedArgs":{"sys.ID":3}}`
	resp := vit.Func(fmt.Sprintf("api/v2/apps/test1/app1/workspaces/%d/commands/app1pkg.CmdODocOne", ws.WSID), body,
		httpu.WithMethod(http.MethodPost),
		httpu.WithAuthorizeBy(ws.Owner.Token),
	)
	odocID := resp.NewIDs["1"]
	orecordID := resp.NewIDs["2"]

	t.Run("403 on deny to read odoc", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"Query":"select * from app1pkg.odoc1.%d"},"elements":[{"fields":["Result"]}]}`, odocID)
		vit.PostWS(ws, "q.sys.SqlQuery", body, httpu.WithAuthorizeBy(apiToken), httpu.Expect403())
	})

	t.Run("403 on deny to read orecord", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"Query":"select * from app1pkg.orecord1.%d"},"elements":[{"fields":["Result"]}]}`, orecordID)
		vit.PostWS(ws, "q.sys.SqlQuery", body, httpu.WithAuthorizeBy(apiToken), httpu.Expect403())
	})
}

func TestReadODocs(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	body := `{"args":
		{"sys.ID": 1,"odocIntFld": 42,"orecord1":[
			{"sys.ID":2,"sys.ParentID":1, "orecord1IntFld": 43, "orecord2": [
				{"sys.ID":3, "sys.ParentID":2, "orecord2IntFld": 44}
			]},
			{"sys.ID":4,"sys.ParentID":1, "orecord1IntFld": 45, "orecord2": [
				{"sys.ID":5, "sys.ParentID":4, "orecord2IntFld": 46}
			]}
		]},
		"unloggedArgs":{"sys.ID":6}
	}`
	resp := vit.Func(fmt.Sprintf("api/v2/apps/test1/app1/workspaces/%d/commands/app1pkg.CmdODocOne", ws.WSID), body,
		httpu.WithMethod(http.MethodPost),
		httpu.WithAuthorizeBy(ws.Owner.Token),
	)
	odoc1ID := resp.NewIDs["1"]
	odoc1ORec11ID := resp.NewIDs["2"]
	odoc1ORec12ID := resp.NewIDs["4"]

	body = `{"args":{"sys.ID": 1,"odocIntFld": 47},"unloggedArgs":{"sys.ID":2}}`
	resp = vit.Func(fmt.Sprintf("api/v2/apps/test1/app1/workspaces/%d/commands/app1pkg.CmdODocOne", ws.WSID), body,
		httpu.WithMethod(http.MethodPost),
		httpu.WithAuthorizeBy(ws.Owner.Token),
	)
	odoc2ID := resp.NewID()

	t.Run("odoc", func(t *testing.T) {
		res := vit.SQLQuery(ws, "select * from app1pkg.odoc1.%d", odoc1ID)
		require.EqualValues(odoc1ID, res["sys.ID"])
		require.EqualValues(42, res["odocIntFld"])
	})

	t.Run("orecord", func(t *testing.T) {
		res := vit.SQLQuery(ws, "select * from app1pkg.orecord1.%d", odoc1ORec11ID)
		require.EqualValues(odoc1ORec11ID, res["sys.ID"])
		require.EqualValues(43, res["orecord1IntFld"])
	})

	t.Run("odocs", func(t *testing.T) {
		res := vit.SQLQueryRows(ws, "select * from app1pkg.odoc1 where id in(%d, %d)", odoc1ID, odoc2ID)
		require.Len(res, 2)
		require.EqualValues(odoc1ID, res[0]["sys.ID"])
		require.EqualValues(42, res[0]["odocIntFld"])
		require.EqualValues(odoc2ID, res[1]["sys.ID"])
		require.EqualValues(47, res[1]["odocIntFld"])
	})

	t.Run("orecords", func(t *testing.T) {
		res := vit.SQLQueryRows(ws, "select * from app1pkg.orecord1 where id in(%d, %d)", odoc1ORec11ID, odoc1ORec12ID)
		require.Len(res, 2)
		require.EqualValues(odoc1ORec11ID, res[0]["sys.ID"])
		require.EqualValues(43, res[0]["orecord1IntFld"])
		require.EqualValues(odoc1ORec12ID, res[1]["sys.ID"])
		require.EqualValues(45, res[1]["orecord1IntFld"])
	})
}

// https://github.com/voedger/voedger/issues/3913
func TestQNameFieldConditions(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	// just expecting no errors on condition on field with qname type
	body := `{"args":{"Query":"select * from app1pkg.ViewWithQName where IntFld = 42 and QName = 'app1pkg.category'"},"elements":[{"fields":["Result"]}]}`
	vit.PostWS(ws, "q.sys.SqlQuery", body)
}

func TestBlobFunctions_BasicUsage(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	expBLOB := []byte{1, 2, 3, 4, 5}
	blobID := vit.UploadBLOB(istructs.AppQName_test1_app1, ws.WSID, "myblob", httpu.ContentType_ApplicationXBinary, expBLOB,
		it.QNameDocWithBLOB, it.Field_Blob, httpu.WithAuthorizeBy(ws.Owner.Token))
	body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.DocWithBLOB","Blob":%d}}]}`, blobID)
	docID := vit.PostWS(ws, "c.sys.CUD", body).NewID()

	t.Run("blobinfo on doc", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select blobinfo(Blob) from app1pkg.DocWithBLOB.%d"},"elements":[{"fields":["Result"]}]}`, docID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		resp.Println()
		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(0)[0].(string)), &m))
		info := m["blobinfo(Blob)"].(map[string]interface{})
		require.Equal("myblob", info["name"])
		require.Equal(httpu.ContentType_ApplicationXBinary, info["mimetype"])
		require.EqualValues(len(expBLOB), info["size"])
	})

	t.Run("blobtext with binary blob returns base64", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select blobtext(Blob) from app1pkg.DocWithBLOB.%d"},"elements":[{"fields":["Result"]}]}`, docID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		resp.Println()
		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(0)[0].(string)), &m))
		require.Equal(base64.StdEncoding.EncodeToString(expBLOB), m["blobtext(Blob)"])
	})

	t.Run("blobinfo and blobtext in one request", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select blobinfo(Blob), blobtext(Blob) from app1pkg.DocWithBLOB.%d"},"elements":[{"fields":["Result"]}]}`, docID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		resp.Println()
		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(0)[0].(string)), &m))
		info := m["blobinfo(Blob)"].(map[string]interface{})
		require.Equal("myblob", info["name"])
		require.Equal(httpu.ContentType_ApplicationXBinary, info["mimetype"])
		require.EqualValues(len(expBLOB), info["size"])
		require.Equal(base64.StdEncoding.EncodeToString(expBLOB), m["blobtext(Blob)"])
	})

	t.Run("blobtext with text blob returns plain text", func(t *testing.T) {
		require := require.New(t)
		textContent := []byte("hello")
		textBlobID := vit.UploadBLOB(istructs.AppQName_test1_app1, ws.WSID, "textblob", httpu.ContentType_TextPlain, textContent,
			it.QNameDocWithBLOB, it.Field_Blob, httpu.WithAuthorizeBy(ws.Owner.Token))
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.DocWithBLOB","Blob":%d}}]}`, textBlobID)
		textDocID := vit.PostWS(ws, "c.sys.CUD", body).NewID()
		body = fmt.Sprintf(`{"args":{"Query":"select blobtext(Blob) from app1pkg.DocWithBLOB.%d"},"elements":[{"fields":["Result"]}]}`, textDocID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		resp.Println()
		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(0)[0].(string)), &m))
		require.Equal(string(textContent), m["blobtext(Blob)"])
	})

	t.Run("blobtext with startFrom", func(t *testing.T) {
		require := require.New(t)
		body := fmt.Sprintf(`{"args":{"Query":"select blobtext(Blob, 2) from app1pkg.DocWithBLOB.%d"},"elements":[{"fields":["Result"]}]}`, docID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		resp.Println()
		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(0)[0].(string)), &m))
		require.Equal(base64.StdEncoding.EncodeToString(expBLOB[2:]), m["blobtext(Blob)"])
	})

	t.Run("blobtext applies startFrom and max bytes", func(t *testing.T) {
		require := require.New(t)
		vitCfg := it.NewOwnVITConfig(
			it.WithApp(istructs.AppQName_test1_app1, it.ProvideApp1,
				it.WithWorkspaceTemplate(it.QNameApp1_TestWSKind, "test_template", sys_test_template.TestTemplateFS),
				it.WithUserLogin("login", "pwd"),
				it.WithChildWorkspace(it.QNameApp1_TestWSKind, "test_ws", "test_template", "", "login", map[string]interface{}{"IntFld": 42}),
			),
			it.WithVVMConfig(func(cfg *vvm.VVMConfig) {
				cfg.BLOBMaxSize = 20000
			}),
		)
		vit := it.NewVIT(t, &vitCfg)
		defer vit.TearDown()
		ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
		textContent := bytes.Repeat([]byte("0123456789"), 1500)
		textBlobID := vit.UploadBLOB(istructs.AppQName_test1_app1, ws.WSID, "bigtext", httpu.ContentType_TextPlain, textContent,
			it.QNameDocWithBLOB, it.Field_Blob, httpu.WithAuthorizeBy(ws.Owner.Token))
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"app1pkg.DocWithBLOB","Blob":%d}}]}`, textBlobID)
		textDocID := vit.PostWS(ws, "c.sys.CUD", body).NewID()
		body = fmt.Sprintf(`{"args":{"Query":"select blobtext(Blob, 5000) from app1pkg.DocWithBLOB.%d"},"elements":[{"fields":["Result"]}]}`, textDocID)
		resp := vit.PostWS(ws, "q.sys.SqlQuery", body)
		resp.Println()
		m := map[string]interface{}{}
		require.NoError(json.Unmarshal([]byte(resp.SectionRow(0)[0].(string)), &m))
		require.Equal(string(textContent[5000:15000]), m["blobtext(Blob)"])
	})

}

func TestBlobFunctionsErrors(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")

	cases := []struct {
		name  string
		query string
		err   string
	}{
		{name: "non-singleton -> ID must be provided", query: "select blobinfo(Blob) from app1pkg.DocWithBLOB", err: "blob functions require a document ID or a singleton"},
		{name: "WHERE clause is not allowed because >1 records could be in result", query: "select blobinfo(Blob) from app1pkg.DocWithBLOB where Blob = 123", err: "WHERE clause is not allowed with blobinfo/blobtext functions"},
		{name: "blobinfo no args", query: "select blobinfo() from app1pkg.DocWithBLOB.123", err: "blobinfo requires at least one argument (field name)"},
		{name: "blobtext no args", query: "select blobtext() from app1pkg.DocWithBLOB.123", err: "blobtext requires at least one argument (field name)"},
		{name: "blob func arg must be a field name", query: "select blobinfo(123) from app1pkg.DocWithBLOB.123", err: "blobinfo: first argument must be a field name"},
		{name: "blobinfo accepts only one arg", query: "select blobinfo(Blob, 123) from app1pkg.DocWithBLOB.123", err: "blobinfo does not accept a second argument"},
		{name: "blobtext accepts at most two args", query: "select blobtext(Blob, 123, 456) from app1pkg.DocWithBLOB.123", err: "blobtext accepts at most 2 arguments"},
		{name: "blobtext second arg must be an integer", query: "select blobtext(Blob, 123.456) from app1pkg.DocWithBLOB.123", err: "blobtext: second argument (startFrom) must be an integer"},
		{name: "blobtext startFrom must be non-negative", query: "select blobtext(Blob, -1) from app1pkg.DocWithBLOB.123", err: "blobtext: invalid startFrom value"},
		{name: "blobtext startFrom must be correct positive", query: "select blobtext(Blob, 9999999999999999999999999999) from app1pkg.DocWithBLOB.123", err: "blobtext: invalid startFrom value"},
		{name: "unknown function", query: "select unknownfunc(Blob) from app1pkg.DocWithBLOB.123", err: "unsupported function: unknownfunc"},
		{name: "duplicate blobinfo on same field", query: "select blobinfo(Blob), blobinfo(Blob) from app1pkg.DocWithBLOB.123", err: `field "blobinfo(Blob)" is selected more than once`},
		{name: "duplicate blobtext on same field", query: "select blobtext(Blob), blobtext(Blob) from app1pkg.DocWithBLOB.123", err: `field "blobtext(Blob)" is selected more than once`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"args":{"Query":"%s"},"elements":[{"fields":["Result"]}]}`, c.query)
			vit.PostWS(ws, "q.sys.SqlQuery", body, it.Expect400(c.err)).Println()
		})
	}
}
