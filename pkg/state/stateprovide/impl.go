/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package stateprovide

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

func put(fieldName string, kind appdef.DataKind, rr istructs.IRowReader, rw istructs.IRowWriter) {
	switch kind {
	case appdef.DataKind_int32:
		rw.PutInt32(fieldName, rr.AsInt32(fieldName))
	case appdef.DataKind_int64:
		rw.PutInt64(fieldName, rr.AsInt64(fieldName))
	case appdef.DataKind_float32:
		rw.PutFloat32(fieldName, rr.AsFloat32(fieldName))
	case appdef.DataKind_float64:
		rw.PutFloat64(fieldName, rr.AsFloat64(fieldName))
	case appdef.DataKind_bytes:
		rw.PutBytes(fieldName, rr.AsBytes(fieldName))
	case appdef.DataKind_string:
		rw.PutString(fieldName, rr.AsString(fieldName))
	case appdef.DataKind_QName:
		rw.PutQName(fieldName, rr.AsQName(fieldName))
	case appdef.DataKind_bool:
		rw.PutBool(fieldName, rr.AsBool(fieldName))
	case appdef.DataKind_RecordID:
		rw.PutRecordID(fieldName, rr.AsRecordID(fieldName))
	default:
		panic(fmt.Errorf("illegal state: field - '%s', kind - '%d': %w", fieldName, kind, ErrNotSupported))
	}
}
