/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sqlquery

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blastrain/vitess-sqlparser/sqlparser"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func readRecords(wsid istructs.WSID, qName appdef.QName, expr sqlparser.Expr, appStructs istructs.IAppStructs, f *filter,
	callback istructs.ExecQueryCallback, recordID istructs.RecordID) error {
	rr := make([]istructs.RecordGetBatchItem, 0)

	switch r := expr.(type) {
	case *sqlparser.ComparisonExpr:
		if r.Left.(*sqlparser.ColName).Name.Lowered() != "id" {
			return fmt.Errorf("unsupported column name: %s", r.Left.(*sqlparser.ColName).Name.String())
		}
		switch r.Operator {
		case sqlparser.EqualStr:
			id, err := parseInt64(r.Right.(*sqlparser.SQLVal).Val)
			if err != nil {
				return err
			}
			rr = append(rr, istructs.RecordGetBatchItem{ID: istructs.RecordID(id)})
		case sqlparser.InStr:
			if r.Left.(*sqlparser.ColName).Name.Lowered() != "id" {
				return fmt.Errorf("unsupported column name: %s", r.Left.(*sqlparser.ColName).Name.String())
			}
			for _, v := range r.Right.(sqlparser.ValTuple) {
				id, err := parseInt64(v.(*sqlparser.SQLVal).Val)
				if err != nil {
					return err
				}
				rr = append(rr, istructs.RecordGetBatchItem{ID: istructs.RecordID(id)})
			}
		default:
			return fmt.Errorf("unsupported operation: %s", r.Operator)
		}
	case nil:
		singletonRec, e := appStructs.Records().GetSingleton(wsid, qName)
		if e != nil {
			if errors.Is(e, istructsmem.ErrNameNotFound) {
				return fmt.Errorf("'%s' is not a singleton. Please specify at least one record ID", qName)
			}
			return e
		}
		rr = append(rr, istructs.RecordGetBatchItem{ID: singletonRec.ID()})
	default:
		return fmt.Errorf("unsupported expression: %T", r)
	}

	if recordID == 0 && len(rr) == 0 {
		return errors.New("at least one record ID must be provided")
	}

	if recordID > 0 && len(rr) > 0 {
		return errors.New("both ID and 'where id ...' clause can not be provided in one query")
	}

	if recordID > 0 && len(rr) == 0 {
		rr = append(rr, istructs.RecordGetBatchItem{ID: recordID})
	}

	err := appStructs.Records().GetBatch(wsid, true, rr)
	if err != nil {
		return err
	}

	t := appStructs.AppDef().Type(qName)

	if !f.acceptAll {
		for field := range f.fields {
			if t.(appdef.IFields).Field(field) == nil {
				return fmt.Errorf("field '%s' not found in def", field)
			}
		}
	}

	for _, r := range rr {
		if r.Record.QName() == appdef.NullQName {
			return fmt.Errorf("record with ID '%d' not found", r.Record.ID())
		}
		if r.Record.QName() != qName {
			return fmt.Errorf("record with ID '%d' has mismatching QName '%s'", r.Record.ID(), r.Record.QName())
		}

		data := coreutils.FieldsToMap(r.Record, appStructs.AppDef(), getFilter(f.filter))
		bb, e := json.Marshal(data)
		if e != nil {
			return e
		}

		e = callback(&result{value: string(bb)})
		if e != nil {
			return e
		}
	}

	return nil
}
