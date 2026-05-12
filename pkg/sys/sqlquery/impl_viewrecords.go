/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sqlquery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blastrain/vitess-sqlparser/sqlparser"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istructs"
)

func readViewRecords(ctx context.Context, wsid istructs.WSID, viewRecordQName appdef.QName, expr sqlparser.Expr, appStructs istructs.IAppStructs, f *filter, callback istructs.ExecQueryCallback) error {
	view := appdef.View(appStructs.AppDef().Type, viewRecordQName)

	if !f.acceptAll {
		allowedFields := make(map[string]bool, view.Key().FieldCount()+view.Value().FieldCount())
		for _, f := range view.Key().Fields() {
			allowedFields[f.Name()] = true
		}
		for _, f := range view.Value().Fields() {
			allowedFields[f.Name()] = true
		}
		for field := range f.fields {
			if !allowedFields[field] {
				return fmt.Errorf("field '%s' does not exist in '%s' value def", field, viewRecordQName)
			}
		}
	}

	kk := make([]keyPart, 0)
	var keyParts func(expr sqlparser.Expr) error
	keyParts = func(expr sqlparser.Expr) error {
		switch r := expr.(type) {
		case *sqlparser.ComparisonExpr:
			if r.Operator != sqlparser.EqualStr {
				return fmt.Errorf("unsupported operator: %s", r.Operator)
			}

			cn := r.Left.(*sqlparser.ColName)

			var name string
			if !cn.Qualifier.IsEmpty() {
				name = fmt.Sprintf("%s.%s", cn.Qualifier.Name, cn.Name)
			} else {
				name = cn.Name.String()
			}

			kk = append(kk, keyPart{
				name:  recoverFieldName(view, name),
				value: r.Right.(*sqlparser.SQLVal).Val,
			})
		case *sqlparser.AndExpr:
			e := keyParts(r.Left)
			if e != nil {
				return e
			}
			e = keyParts(r.Right)
			if e != nil {
				return e
			}
		case nil:
		default:
			return fmt.Errorf("unsupported expression: %T", r)
		}
		return nil
	}
	err := keyParts(expr)
	if err != nil {
		return err
	}

	kb := appStructs.ViewRecords().KeyBuilder(viewRecordQName)

	for i, k := range kk {
		correctedName := recoverFieldName(view.Key(), k.name)
		kk[i].name = correctedName
		f := view.Key().Field(correctedName)
		if f == nil {
			return fmt.Errorf("field '%s' does not exist in '%s' key def", k.name, viewRecordQName)
		}

		switch f.DataKind() {
		case appdef.DataKind_int8, appdef.DataKind_int16, appdef.DataKind_int32, appdef.DataKind_int64,
			appdef.DataKind_float32, appdef.DataKind_float64, appdef.DataKind_RecordID:
			kb.PutNumber(k.name, json.Number(k.value))
		case appdef.DataKind_bytes, appdef.DataKind_string, appdef.DataKind_QName:
			kb.PutChars(k.name, string(k.value))
		default:
			return errUnsupportedDataKind
		}
	}

	return appStructs.ViewRecords().Read(ctx, wsid, kb, func(key istructs.IKey, value istructs.IValue) (err error) {
		data := coreutils.FieldsToMap(key, appStructs.AppDef(), getFilter(f.filter))
		for k, v := range coreutils.FieldsToMap(value, appStructs.AppDef(), getFilter(f.filter)) {
			data[k] = v
		}
		bb, err := json.Marshal(data)
		if err != nil {
			return err
		}

		return callback(&result{value: string(bb)})
	})
}
