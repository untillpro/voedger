/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package queryprocessor

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

type EqualsFilter struct {
	field   string
	value   interface{}
	epsilon float64
}

func (f EqualsFilter) IsMatch(fd coreutils.FieldsDef, outputRow IOutputRow) (bool, error) {
	switch fd[f.field] {
	case appdef.DataKind_int32:
		return outputRow.Value(f.field).(int32) == int32(f.value.(float64)), nil
	case appdef.DataKind_int64:
		return outputRow.Value(f.field).(int64) == int64(f.value.(float64)), nil
	case appdef.DataKind_float32:
		return nearlyEqual(f.value.(float64), float64(outputRow.Value(f.field).(float32)), f.epsilon), nil
	case appdef.DataKind_float64:
		return nearlyEqual(f.value.(float64), outputRow.Value(f.field).(float64), f.epsilon), nil
	case appdef.DataKind_string:
		return outputRow.Value(f.field).(string) == f.value.(string), nil
	case appdef.DataKind_bool:
		return outputRow.Value(f.field).(bool) == f.value.(bool), nil
	case appdef.DataKind_RecordID:
		return outputRow.Value(f.field).(istructs.RecordID) == istructs.RecordID(int64(f.value.(float64))), nil
	case appdef.DataKind_null:
		return false, nil
	default:
		return false, fmt.Errorf("'%s' filter: field %s: %w", filterKind_Eq, f.field, ErrWrongType)
	}
}
