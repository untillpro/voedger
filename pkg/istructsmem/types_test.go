/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package istructsmem

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/testingu/require"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem/internal/qnames"
)

func Test_rowNullType(t *testing.T) {
	require := require.New(t)

	row := newTestRow()

	row.setQName(appdef.NullQName)
	require.Equal(appdef.NullQName, row.QName())
	require.Equal(appdef.NullType, row.typeDef())

	row.setType(appdef.NullType)
	require.Equal(appdef.NullQName, row.QName())
	require.Equal(appdef.NullType, row.typeDef())

	row.setType(nil)
	require.Equal(appdef.NullQName, row.QName())
	require.Equal(appdef.NullType, row.typeDef())
}

func Test_clarifyJSONValue(t *testing.T) {
	require := require.New(t)
	test := test()

	row := newTestRow()

	id, _ := test.AppCfg.qNames.ID(test.saleCmdName)
	expectedQNameIDBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(expectedQNameIDBytes, id)

	cases := []struct {
		val              interface{}
		kind             appdef.DataKind
		expectedTypeKind reflect.Kind
		expectedVal      interface{}
	}{
		{val: int32(7), kind: appdef.DataKind_int32, expectedTypeKind: reflect.Int32},
		{val: int64(7), kind: appdef.DataKind_int64, expectedTypeKind: reflect.Int64},
		{val: float32(7.7), kind: appdef.DataKind_float32, expectedTypeKind: reflect.Float32},
		{val: float64(7.7), kind: appdef.DataKind_float64, expectedTypeKind: reflect.Float64},
		{val: istructs.RecordID(7), kind: appdef.DataKind_RecordID, expectedTypeKind: reflect.Uint64},
		{val: json.Number("7"), kind: appdef.DataKind_int32, expectedTypeKind: reflect.Int32, expectedVal: int32(7)},
		{val: json.Number("7"), kind: appdef.DataKind_int64, expectedTypeKind: reflect.Int64, expectedVal: int64(7)},
		{val: json.Number("7.7"), kind: appdef.DataKind_float32, expectedTypeKind: reflect.Float32, expectedVal: float32(7.7)},
		{val: json.Number("7.7"), kind: appdef.DataKind_float64, expectedTypeKind: reflect.Float64, expectedVal: float64(7.7)},
		{val: json.Number("7"), kind: appdef.DataKind_RecordID, expectedTypeKind: reflect.Uint64, expectedVal: istructs.RecordID(7)},
		{val: true, kind: appdef.DataKind_bool, expectedTypeKind: reflect.Bool},
		{val: "test 🎄 tree", kind: appdef.DataKind_string, expectedTypeKind: reflect.String},
		{val: test.saleCmdName, kind: appdef.DataKind_QName, expectedTypeKind: reflect.Slice, expectedVal: expectedQNameIDBytes},
		{val: test.saleCmdName.String(), kind: appdef.DataKind_QName, expectedTypeKind: reflect.Slice, expectedVal: expectedQNameIDBytes},
		{val: []byte{1, 2, 3}, kind: appdef.DataKind_bytes, expectedTypeKind: reflect.Slice},
		{val: "AQIDBA==", kind: appdef.DataKind_bytes, expectedTypeKind: reflect.Slice, expectedVal: []byte{1, 2, 3, 4}},
	}

	for _, c := range cases {
		t.Run(fmt.Sprintf("%T", c.val), func(t *testing.T) {
			v, err := row.clarifyJSONValue(c.val, c.kind)
			require.NoError(err)
			if c.expectedVal != nil {
				require.Equal(c.expectedVal, v)
			} else {
				require.EqualValues(c.val, v)
			}
			require.Equal(c.expectedTypeKind, reflect.TypeOf(v).Kind(), c.expectedTypeKind.String(), reflect.TypeOf(v).Kind().String())
		})
	}

	errorCases := []struct {
		val           interface{}
		kind          appdef.DataKind
		expectedError error
	}{
		{val: float64(7), kind: appdef.DataKind_int32, expectedError: ErrWrongFieldType},
		{val: float64(7), kind: appdef.DataKind_int64, expectedError: ErrWrongFieldType},
		{val: float64(7), kind: appdef.DataKind_float32, expectedError: ErrWrongFieldType},
		{val: float32(7), kind: appdef.DataKind_float64, expectedError: ErrWrongFieldType},
		{val: float64(7), kind: appdef.DataKind_RecordID, expectedError: ErrWrongFieldType},
		{val: json.Number("1.1"), kind: appdef.DataKind_int32},
		{val: json.Number("1.1"), kind: appdef.DataKind_int64},
		{val: json.Number("1.1"), kind: appdef.DataKind_RecordID},
		{val: json.Number(strconv.Itoa(math.MaxInt32 + 1)), kind: appdef.DataKind_int32},
		{val: json.Number(strconv.Itoa(math.MinInt32 - 1)), kind: appdef.DataKind_int32},
		{val: json.Number(fmt.Sprint(math.MaxInt64 + (float64(1)))), kind: appdef.DataKind_int64},
		{val: json.Number(fmt.Sprint(math.MinInt64 - (float64(1)))), kind: appdef.DataKind_int64},
		{val: json.Number(fmt.Sprint(math.MaxFloat64)), kind: appdef.DataKind_float32},
		{val: json.Number(fmt.Sprint(-math.MaxFloat64)), kind: appdef.DataKind_float32},
		{val: json.Number("a"), kind: appdef.DataKind_float32},
		{val: json.Number("a"), kind: appdef.DataKind_float64},
		{val: json.Number("a"), kind: appdef.DataKind_int32},
		{val: json.Number("a"), kind: appdef.DataKind_int64},
		{val: json.Number("a"), kind: appdef.DataKind_RecordID},
		{val: json.Number(coreutils.TooBigNumberStr), kind: appdef.DataKind_float64},
		{val: json.Number("-" + coreutils.TooBigNumberStr), kind: appdef.DataKind_float64},
		{val: json.Number("-1"), kind: appdef.DataKind_RecordID},
		{val: int64(-1), kind: appdef.DataKind_RecordID},
		{val: float64(7), kind: appdef.DataKind_bool, expectedError: ErrWrongFieldType},
		{val: float64(7), kind: appdef.DataKind_string, expectedError: ErrWrongFieldType},
		{val: float64(7), kind: appdef.DataKind_QName, expectedError: ErrWrongFieldType},
		{val: float64(7), kind: appdef.DataKind_bytes, expectedError: ErrWrongFieldType},
		{val: "a", kind: appdef.DataKind_bytes},
		{val: "a", kind: appdef.DataKind_QName},
		{val: "a.a", kind: appdef.DataKind_QName, expectedError: qnames.ErrNameNotFound},
		{val: appdef.NewQName("a", "a"), kind: appdef.DataKind_QName, expectedError: qnames.ErrNameNotFound},
	}

	for i, ec := range errorCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			clarifiedVal, err := row.clarifyJSONValue(ec.val, ec.kind)
			if ec.expectedError != nil {
				require.ErrorIs(err, ec.expectedError)
			} else {
				require.Error(err)
			}
			require.Zero(clarifiedVal)
		})
	}

	t.Run("test Record", func(t *testing.T) {
		var testRec istructs.IRecord = newTestCRecord(100500700)

		checkRecord := func(data interface{}) {
			b, ok := data.([]byte)
			require.True(ok)
			require.NotNil(b)
			r := newTestCRecord(istructs.NullRecordID)
			err := r.loadFromBytes(b)
			require.NoError(err)
			testRecsIsEqual(t, testRec, r)
		}

		v, err := row.clarifyJSONValue(testRec, appdef.DataKind_Record)
		require.NoError(err)
		checkRecord(v)

		v, err = row.clarifyJSONValue("ups", appdef.DataKind_Record)
		require.ErrorIs(err, ErrWrongFieldType)
		require.Nil(v)
	})

	t.Run("test Event", func(t *testing.T) {
		var testEvent istructs.IDbEvent = newTestEvent(100501, 100500700)

		checkEvent := func(data interface{}) {
			b, ok := data.([]byte)
			require.True(ok)
			require.NotNil(b)
			e := newEmptyTestEvent()
			err := e.loadFromBytes(b)
			require.NoError(err)
			testTestEvent(t, testEvent, 100501, 100500700, false)
		}

		v, err := row.clarifyJSONValue(testEvent, appdef.DataKind_Event)
		require.NoError(err)
		checkEvent(v)

		v, err = row.clarifyJSONValue("ups", appdef.DataKind_Event)
		require.ErrorIs(err, ErrWrongFieldType)
		require.Nil(v)
	})
}

func Test_rowType_PutAs_SimpleTypes(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("Putxxx and Asxxx row methods for simple types", func(t *testing.T) {
		row1 := newTestRow()

		testTestRow(t, row1)

		row2 := newRow(nil)
		row2.copyFrom(row1)

		testRowsIsEqual(t, row1, row2)

		testTestRow(t, row2)
	})

	t.Run("Asxxx row methods must return default values if not calls Putxxx", func(t *testing.T) {
		row := newEmptyTestRow()

		require.Equal(int32(0), row.AsInt32("int32"))
		require.Equal(int64(0), row.AsInt64("int64"))
		require.Equal(float32(0), row.AsFloat32("float32"))
		require.Equal(float64(0), row.AsFloat64("float64"))
		require.Equal([]byte(nil), row.AsBytes("bytes"))
		require.Equal("", row.AsString("string"))

		require.EqualValues([]byte(nil), row.AsBytes("raw"))

		require.Equal(appdef.NullQName, row.AsQName("QName"))
		require.False(row.AsBool("bool"))
		require.Equal(istructs.NullRecordID, row.AsRecordID("RecordID"))

		val := newEmptyTestViewValue()
		require.Equal(istructs.IDbEvent(nil), val.AsEvent(test.testViewRecord.valueFields.event))
		rec := val.AsRecord(test.testViewRecord.valueFields.record)
		require.Equal(appdef.NullQName, rec.QName())
	})

	t.Run("PutNumber to numeric-type fields must be available (json)", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutNumber("int32", json.Number("1"))
		row.PutNumber("int64", json.Number("2"))
		row.PutNumber("float32", json.Number("3"))
		row.PutNumber("float64", json.Number("4"))
		row.PutNumber("RecordID", json.Number("5"))

		require.NoError(row.build())

		require.Equal(int32(1), row.AsInt32("int32"))
		require.Equal(int64(2), row.AsInt64("int64"))
		require.Equal(float32(3), row.AsFloat32("float32"))
		require.Equal(float64(4), row.AsFloat64("float64"))
		require.Equal(istructs.RecordID(5), row.AsRecordID("RecordID"))

		t.Run("Should be OK to Asxxx with type casts", func(t *testing.T) {
			require.EqualValues(1, row.AsFloat64("int32"))
			require.EqualValues(2, row.AsFloat64("int64"))
			require.EqualValues(3, row.AsFloat64("float32"))
			require.EqualValues(5, row.AsFloat64("RecordID"))

			require.EqualValues(5, row.AsInt64("RecordID"))

			require.EqualValues(2, row.AsRecordID("int64"))
		})
	})

	t.Run("PutChars to char-type fields (string, bytes, raw and QName) must be available (json)", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutChars("string", "test 🏐 哇")
		row.PutChars("QName", test.saleCmdName.String())

		// cspell:disable
		row.PutChars("bytes", "AQIDBA==")
		// cspell:enable

		rawValue := bytes.Repeat([]byte{1, 2, 3, 4}, 1024)
		row.PutChars("raw", base64.StdEncoding.EncodeToString(rawValue))

		require.NoError(row.build())

		require.Equal("test 🏐 哇", row.AsString("string"))
		require.Equal(test.saleCmdName, row.AsQName("QName"))
		require.Equal([]byte{1, 2, 3, 4}, row.AsBytes("bytes"))
		require.Equal(rawValue, row.AsBytes("raw"))
	})
}

func Test_rowType_PutFromJSON(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("basic", func(t *testing.T) {

		bld := test.AppStructs.ObjectBuilder(test.testRow)

		data := map[appdef.FieldName]any{
			"int32":    json.Number("1"),
			"int64":    json.Number("2"),
			"float32":  json.Number("3"),
			"float64":  json.Number("4"),
			"bytes":    "BQY=", // []byte{5,6}
			"string":   "str",
			"QName":    test.testCDoc.String(),
			"bool":     true,
			"RecordID": json.Number("7"),
		}

		bld.PutFromJSON(data)

		row, err := bld.Build()
		require.NoError(err)

		require.EqualValues(test.testRow, row.QName())
		require.EqualValues(1, row.AsInt32("int32"))
		require.EqualValues(2, row.AsInt64("int64"))
		require.EqualValues(3, row.AsFloat32("float32"))
		require.EqualValues(4, row.AsFloat64("float64"))
		require.Equal([]byte{5, 6}, row.AsBytes("bytes"))
		require.Equal("str", row.AsString("string"))
		require.Equal(test.testCDoc, row.AsQName("QName"))
		require.True(row.AsBool("bool"))
		require.EqualValues(7, row.AsRecordID("RecordID"))
	})

	t.Run("[]byte as bytes value instead of base64 string", func(t *testing.T) {
		bld := test.AppStructs.ObjectBuilder(test.testRow)
		data := map[appdef.FieldName]any{
			"bytes": []byte{1, 2, 3},
		}
		bld.PutFromJSON(data)
		row, err := bld.Build()
		require.NoError(err)
		require.Equal([]byte{1, 2, 3}, row.AsBytes("bytes"))
	})

	t.Run("wrong type -> error", func(t *testing.T) {
		bld := test.AppStructs.ObjectBuilder(test.testRow)

		data := map[appdef.FieldName]any{
			"int32": uint8(42),
		}
		bld.PutFromJSON(data)
		_, err := bld.Build()
		require.ErrorIs(err, ErrWrongType)
	})

	t.Run("json.Number errors", func(t *testing.T) {
		cases := map[string][]struct {
			val         interface{}
			expectedErr error
		}{
			"int32": {
				{val: json.Number("1.1"), expectedErr: strconv.ErrSyntax},
				{val: json.Number("d"), expectedErr: strconv.ErrSyntax},
				{val: json.Number(strconv.Itoa(math.MaxInt32 + 1)), expectedErr: coreutils.ErrNumberOverflow},
				{val: json.Number(strconv.Itoa(math.MinInt32 - 1)), expectedErr: coreutils.ErrNumberOverflow},
			},
			"int64": {
				{val: json.Number("1.1"), expectedErr: strconv.ErrSyntax},
				{val: json.Number("d"), expectedErr: strconv.ErrSyntax},
				{val: json.Number(coreutils.TooBigNumberStr), expectedErr: strconv.ErrRange},
				{val: json.Number("-" + coreutils.TooBigNumberStr), expectedErr: strconv.ErrRange},
			},
			"float32": {
				{val: json.Number("d"), expectedErr: strconv.ErrSyntax},
				{val: json.Number(fmt.Sprint(math.MaxFloat64)), expectedErr: coreutils.ErrNumberOverflow},
				{val: json.Number(fmt.Sprint(-math.MaxFloat64)), expectedErr: coreutils.ErrNumberOverflow},
			},
			"float64": {
				{val: json.Number("d"), expectedErr: strconv.ErrSyntax},
				{val: json.Number(coreutils.TooBigNumberStr), expectedErr: strconv.ErrRange},
				{val: json.Number("-" + coreutils.TooBigNumberStr), expectedErr: strconv.ErrRange},
			},
		}

		for fieldName, vals := range cases {
			for _, val := range vals {
				bld := test.AppStructs.ObjectBuilder(test.testRow)
				data := map[string]interface{}{}
				data[fieldName] = val.val
				bld.PutFromJSON(data)
				_, err := bld.Build()
				if val.expectedErr != nil {
					require.ErrorIs(err, val.expectedErr)
				} else {
					require.Error(err)
				}
			}
		}
	})
}

func Test_rowType_PutAs_ComplexTypes(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("PutRecord and PutEvent / AsRecord and AsEvent row methods (via IValue)", func(t *testing.T) {
		v1 := newTestViewValue()
		testTestViewValue(t, v1)

		v2 := newTestViewValue()
		v2.copyFrom(&v1.rowType)
		testTestViewValue(t, v2)

		testRowsIsEqual(t, &v1.rowType, &v2.rowType)
	})

	t.Run("must success NullRecord value for PutRecord / AsRecord methods", func(t *testing.T) {
		row := newEmptyTestViewValue()
		row.PutString(test.testViewRecord.valueFields.buyer, "buyer")
		row.PutRecord(test.testViewRecord.valueFields.record, NewNullRecord(istructs.NullRecordID))
		require.NoError(row.build())

		rec := row.AsRecord(test.testViewRecord.valueFields.record)
		require.NotNil(rec)
		require.Equal(appdef.NullQName, rec.QName())
		require.Equal(istructs.NullRecordID, rec.AsRecordID(appdef.SystemField_ID))
	})
}

func Test_rowType_PutErrors(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("Putxxx to unknown field names must be error", func(t *testing.T) {

		testPut := func(put func(row istructs.IRowWriter)) {
			row := newRow(test.AppCfg)
			row.setQName(test.testRow)
			put(row)
			require.ErrorIs(row.build(), ErrNameNotFound)
		}

		testPut(func(row istructs.IRowWriter) { row.PutInt32("unknown_field", 1) })
		testPut(func(row istructs.IRowWriter) { row.PutInt32("unknown_field", 1) })
		testPut(func(row istructs.IRowWriter) { row.PutInt64("unknown_field", 2) })
		testPut(func(row istructs.IRowWriter) { row.PutFloat32("unknown_field", 3) })
		testPut(func(row istructs.IRowWriter) { row.PutFloat64("unknown_field", 4) })
		testPut(func(row istructs.IRowWriter) { row.PutBytes("unknown_field", []byte{1, 2, 3}) })
		testPut(func(row istructs.IRowWriter) { row.PutString("unknown_field", "abc") })
		testPut(func(row istructs.IRowWriter) { row.PutQName("unknown_field", istructs.QNameForError) })
		testPut(func(row istructs.IRowWriter) { row.PutBool("unknown_field", true) })
		testPut(func(row istructs.IRowWriter) { row.PutRecordID("unknown_field", istructs.NullRecordID) })

		testPut(func(row istructs.IRowWriter) { row.PutNumber("unknown_field", json.Number("88")) })
		testPut(func(row istructs.IRowWriter) { row.PutChars("unknown_field", "c.h.a.r.s.") })
	})

	t.Run("Putxxx with wrong types must be error", func(t *testing.T) {

		testPut := func(put func(row istructs.IRowWriter)) {
			row := newRow(test.AppCfg)
			row.setQName(test.testRow)
			put(row)
			require.ErrorIs(row.build(), ErrWrongFieldType)
		}

		testPut(func(row istructs.IRowWriter) { row.PutInt32("int64", 1) })
		testPut(func(row istructs.IRowWriter) { row.PutInt64("float32", 2) })
		testPut(func(row istructs.IRowWriter) { row.PutFloat32("int32", 3) })
		testPut(func(row istructs.IRowWriter) { row.PutFloat64("string", 4) })
		testPut(func(row istructs.IRowWriter) { row.PutRecordID("raw", 4) })
		testPut(func(row istructs.IRowWriter) { row.PutBytes("float64", []byte{1, 2, 3}) })
		testPut(func(row istructs.IRowWriter) { row.PutString("bytes", "abc") })
		testPut(func(row istructs.IRowWriter) { row.PutQName("RecordID", istructs.QNameForError) })
		testPut(func(row istructs.IRowWriter) { row.PutBool("QName", true) })
		testPut(func(row istructs.IRowWriter) { row.PutRecordID("bool", istructs.NullRecordID) })
	})

	t.Run("PutNumber to non-numeric type field must be error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutNumber("bytes", json.Number("29"))
		row.PutNumber("raw", json.Number("3.141592653589793238"))

		require.ErrorIs(row.build(), ErrWrongFieldType)
	})

	t.Run("PutQName with unknown QName value must be error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutQName("QName", appdef.NewQName("unknown", "unknown"))

		require.ErrorIs(row.build(), qnames.ErrNameNotFound)
	})

	t.Run("PutChars error handling", func(t *testing.T) {
		t.Run("PutChars to non-char type fields must be error", func(t *testing.T) {
			row := makeRow(test.AppCfg)
			row.setQName(test.testRow)

			row.PutChars("int32", "29")

			require.ErrorIs(row.build(), ErrWrongFieldType)
		})

		t.Run("PutChars to QName-type fields non convertible value must be error", func(t *testing.T) {
			row := makeRow(test.AppCfg)
			row.setQName(test.testRow)

			row.PutChars("QName", "welcome.2.error")

			require.ErrorIs(row.build(), appdef.ErrConvertError)
		})

		t.Run("PutChars to bytes-type fields non convertible base64 value must be error", func(t *testing.T) {
			row := makeRow(test.AppCfg)
			row.setQName(test.testRow)

			row.PutChars("bytes", "welcome.2.error")

			require.ErrorContains(row.build(), "illegal base64 data")
		})
	})

	t.Run("Multiply Putxxx errors must be concatenated in build error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutFloat32("unknown_field", 555.5)
		row.PutInt32("int64", 1)

		err := row.build()
		require.ErrorIs(err, ErrNameNotFound)
		require.ErrorIs(err, ErrWrongFieldType)
	})

	t.Run("Must be error to put into abstract table", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.abstractCDoc)

		row.PutInt32("int32", 1)

		err := row.build()
		require.ErrorIs(err, ErrAbstractType)
	})
}

func Test_rowType_AsPanics(t *testing.T) {
	t.Run("Asxxx unknown fields must panic", func(t *testing.T) {
		require := require.New(t)

		unknown := "unknownField"
		row := newTestRow()

		require.Panics(func() { row.AsInt32(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsInt64(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsFloat32(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsFloat64(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsBytes(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsString(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsQName(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsBool(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsRecordID(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsRecord(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
		require.Panics(func() { row.AsEvent(unknown) }, require.Is(ErrNameNotFound), require.Has(unknown))
	})

	t.Run("Asxxx from fields with invalid type cast must panic", func(t *testing.T) {
		require := require.New(t)
		row := newTestRow()

		require.Panics(func() { row.AsInt32("raw") }, require.Is(ErrNameNotFound), require.Has("raw"))
		require.Panics(func() { row.AsInt64("string") }, require.Is(ErrNameNotFound), require.Has("string"))
		require.Panics(func() { row.AsFloat32("bytes") }, require.Is(ErrNameNotFound), require.Has("bytes"))
		require.Panics(func() { row.AsFloat64("bool") }, require.Is(ErrNameNotFound), require.Has("bool"))
		require.Panics(func() { row.AsBytes("QName") }, require.Is(ErrNameNotFound), require.Has("QName"))
		require.Panics(func() { row.AsString("RecordID") }, require.Is(ErrNameNotFound), require.Has("RecordID"))
		require.Panics(func() { row.AsQName("int32") }, require.Is(ErrNameNotFound), require.Has("int32"))
		require.Panics(func() { row.AsBool("int64") }, require.Is(ErrNameNotFound), require.Has("int64"))
		require.Panics(func() { row.AsRecordID("float32") }, require.Is(ErrNameNotFound), require.Has("float32"))
		require.Panics(func() { row.AsRecord("float64") }, require.Is(ErrNameNotFound), require.Has("float64"))
		require.Panics(func() { row.AsEvent("bool") }, require.Is(ErrNameNotFound), require.Has("bool"))
	})
}

func Test_rowType_RecordIDs(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("should be ok to enum all IDs with RecordIDs(true)", func(t *testing.T) {

		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutRecordID("RecordID", 1)
		row.PutRecordID("RecordID_2", 2)

		require.NoError(row.build())

		cnt := 0
		for name, value := range row.RecordIDs(true) {
			switch name {
			case "RecordID":
				require.Equal(istructs.RecordID(1), value)
			case "RecordID_2":
				require.Equal(istructs.RecordID(2), value)
			default:
				require.Fail("unexpected field name", "field name: «%s»", name)
			}
			cnt++
		}

		require.Equal(2, cnt)
	})

	t.Run("should be ok to enum not null IDs with RecordIDs(false)", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutRecordID("RecordID", 1)
		row.PutRecordID("RecordID_2", istructs.NullRecordID)

		require.NoError(row.build())

		cnt := 0
		for name, value := range row.RecordIDs(false) {
			switch name {
			case "RecordID":
				require.Equal(istructs.RecordID(1), value)
			default:
				require.Fail("unexpected field name", "field name: «%s»", name)
			}
			cnt++
		}

		require.Equal(1, cnt)
	})
}

func Test_rowType_maskValues(t *testing.T) {
	require := require.New(t)

	t.Run("maskValues must hide all rows data", func(t *testing.T) {
		row := newTestRow()

		row.maskValues()

		require.Equal(int32(0), row.AsInt32("int32"))
		require.Equal(int64(0), row.AsInt64("int64"))
		require.Equal(float32(0), row.AsFloat32("float32"))
		require.Equal(float64(0), row.AsFloat64("float64"))
		require.Nil(row.AsBytes("bytes"))
		require.Equal("*", row.AsString("string"))
		require.Nil(row.AsBytes("raw"))
		require.Equal(appdef.NullQName, row.AsQName("QName"))
		require.False(row.AsBool("bool"))
		require.Equal(istructs.NullRecordID, row.AsRecordID("RecordID"))
	})
}

func Test_rowType_FieldNames(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("new [or null] row must have hot fields", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		for fieldName := range row.FieldNames {
			require.Fail("unexpected field", "name: «%s»", fieldName)
		}
	})

	t.Run("new test row must have only QName field", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		cnt := 0
		for fieldName := range row.FieldNames {
			require.Equal(appdef.SystemField_QName, fieldName)
			cnt++
		}
		require.Equal(1, cnt)
	})

	t.Run("filled test row must iterate all fields without duplicates", func(t *testing.T) {
		row := newTestRow()

		cnt := 0
		names := make(map[appdef.FieldName]bool)
		for fieldName := range row.FieldNames {
			require.False(names[fieldName])
			names[fieldName] = true
			cnt++
		}
		require.Equal(11, cnt) // sys.QName + ten user fields for simple types
	})

	t.Run("should be ok iterate with filled system fields", func(t *testing.T) {
		rec := newTestCRecord(7)
		rec.PutRecordID(appdef.SystemField_ParentID, 5)
		rec.PutString(appdef.SystemField_Container, "rec")

		sys := make(map[appdef.FieldName]interface{})
		for fieldName := range rec.FieldNames {
			if appdef.IsSysField(fieldName) {
				switch rec.fieldDef(fieldName).DataKind() {
				case appdef.DataKind_QName:
					sys[fieldName] = rec.AsQName(fieldName)
				case appdef.DataKind_RecordID:
					sys[fieldName] = rec.AsRecordID(fieldName)
				case appdef.DataKind_string:
					sys[fieldName] = rec.AsString(fieldName)
				case appdef.DataKind_bool:
					sys[fieldName] = rec.AsBool(fieldName)
				default:
					require.Fail("unexpected system field", "field name: «%s»", fieldName)
				}
			}
		}
		require.Len(sys, 5)
		require.EqualValues(test.testCRec, sys[appdef.SystemField_QName])
		require.EqualValues(7, sys[appdef.SystemField_ID])
		require.EqualValues(5, sys[appdef.SystemField_ParentID])
		require.EqualValues("rec", sys[appdef.SystemField_Container])
		require.True(sys[appdef.SystemField_IsActive].(bool))
	})
}

func Test_rowType_BuildErrors(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("Putxxx unknown field name must have build error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutInt32("unknown", 1)
		require.ErrorIs(row.build(), ErrNameNotFound)
	})

	t.Run("Putxxx invalid field value type must have build error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutString("int32", "a")
		require.ErrorIs(row.build(), ErrWrongFieldType)
	})

	t.Run("PutString to []byte type must collect convert error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutString("bytes", "some string")

		require.ErrorIs(row.build(), ErrWrongFieldType)
	})

	t.Run("PutQName invalid QName must have build error", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutString("QName", "zZz")

		require.ErrorIs(row.build(), ErrWrongFieldType)
	})
}

func Test_rowType_Nils(t *testing.T) {
	require := require.New(t)
	test := test()

	t.Run("must be empty nils if no nil assignment", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		row.PutInt32("int32", 8)
		require.NoError(row.build())
		require.Empty(row.nils)
	})

	t.Run("check nils", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)

		t.Run("check first nil", func(t *testing.T) {
			row.PutInt32("int32", 8)
			row.PutChars("bytes", "")
			require.NoError(row.build())
			require.Len(row.nils, 1)
			require.Contains(row.nils, "bytes")
		})

		t.Run("check second nil", func(t *testing.T) {
			row.PutChars("string", "")
			require.NoError(row.build())
			require.Len(row.nils, 2)
			require.Contains(row.nils, "bytes")
			require.Contains(row.nils, "string")
		})

		t.Run("check third nil", func(t *testing.T) {
			row.PutChars("raw", "")
			require.NoError(row.build())
			require.Len(row.nils, 3)
			require.Contains(row.nils, "bytes")
			require.Contains(row.nils, "string")
			require.Contains(row.nils, "raw")
		})

		t.Run("check repeat nil", func(t *testing.T) {
			row.PutChars("bytes", "")
			require.NoError(row.build())
			require.Len(row.nils, 3)
			require.Contains(row.nils, "bytes")
			require.Contains(row.nils, "string")
			require.Contains(row.nils, "raw")
		})

		t.Run("check nils are kept", func(t *testing.T) {
			row.PutInt32("int32", 888)
			require.NoError(row.build())
			require.Len(row.nils, 3)
			require.Contains(row.nils, "bytes")
			require.Contains(row.nils, "string")
			require.Contains(row.nils, "raw")
		})

		t.Run("check nil can be reassigned", func(t *testing.T) {
			row.PutBytes("bytes", []byte{0})
			require.NoError(row.build())
			require.Len(row.nils, 2)
			require.Contains(row.nils, "string")
			require.Contains(row.nils, "raw")

			row.PutBytes("raw", []byte("📷"))
			require.NoError(row.build())
			require.Len(row.nils, 1)
			require.Contains(row.nils, "string")
		})
	})

	t.Run("check nil assignment", func(t *testing.T) {
		row := makeRow(test.AppCfg)
		row.setQName(test.testRow)
		row.PutInt32("int32", 0)
		row.PutInt64("int64", 0)
		row.PutFloat32("float32", 0)
		row.PutFloat64("float64", 0)
		row.PutBytes("bytes", []byte{})
		row.PutString("string", "")
		row.PutBytes("raw", []byte{})
		row.PutQName("QName", appdef.NullQName)
		row.PutBool("bool", false)
		row.PutRecordID("RecordID", istructs.NullRecordID)

		require.NoError(row.build())

		require.True(row.HasValue("int32"))
		require.True(row.HasValue("int64"))
		require.True(row.HasValue("float32"))
		require.True(row.HasValue("float64"))
		require.False(row.HasValue("bytes"))
		require.False(row.HasValue("string"))
		require.False(row.HasValue("raw"))
		require.True(row.HasValue("QName"))
		require.True(row.HasValue("bool"))
		require.True(row.HasValue("RecordID"))

		cnt := 0
		row.dyB.IterateFields(nil, func(name string, newData interface{}) bool {
			switch name {
			case "int32", "int64", "float32", "float64":
				require.Zero(newData)
			case "QName":
				var nullQNameBytes = []byte{0x0, 0x0}
				require.Equal(nullQNameBytes, newData)
			case "bool":
				require.False(newData.(bool))
			case "RecordID":
				require.EqualValues(istructs.NullRecordID, newData)
			default:
				require.Fail("unexpected field", "field name: «%s»", name)
			}
			cnt++
			return true
		})

		require.Equal(7, cnt)

		require.Len(row.nils, 3)
		require.Contains(row.nils, "bytes")
		require.Contains(row.nils, "string")
		require.Contains(row.nils, "raw")
	})
}

func Test_rowType_String(t *testing.T) {
	require := require.New(t)

	test := test()

	t.Run("must be null row", func(t *testing.T) {
		r := newRow(test.AppCfg)
		require.Equal("null row", r.String())
	})

	t.Run("must be complete form for record", func(t *testing.T) {
		r := newRecord(test.AppCfg)
		r.setQName(test.testCRec)
		r.setContainer("child")
		s := r.String()
		require.Contains(s, "CRecord")
		require.Contains(s, "«child: test.Record»")
	})

	t.Run("must be short form for document", func(t *testing.T) {
		r := newRecord(test.AppCfg)
		r.setQName(test.testCDoc)
		s := r.String()
		require.Contains(s, "CDoc")
		require.Contains(s, "«test.CDoc»")
	})
}

func TestWrong(t *testing.T) {
	require := require.New(t)
	test := test()

	bld := test.AppStructs.ObjectBuilder(test.testRow)

	data := map[appdef.FieldName]any{
		"RecordID": istructs.RecordID(1),
	}

	bld.PutFromJSON(data)

	row, err := bld.Build()
	require.NoError(err)
	require.EqualValues(istructs.RecordID(1), row.AsRecordID("RecordID"))
}
