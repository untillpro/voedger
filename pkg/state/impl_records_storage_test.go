/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package state

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/untillpro/voedger/pkg/istructs"
)

func TestRecordsStorage_GetBatch(t *testing.T) {
	type result struct {
		key    istructs.IKeyBuilder
		value  istructs.IStateValue
		exists bool
	}
	t.Run("Should handle general records", func(t *testing.T) {
		require := require.New(t)
		records := &mockRecords{}
		records.
			On("GetBatch", istructs.WSID(1), true, mock.AnythingOfType("[]istructs.RecordGetBatchItem")).
			Return(nil).
			Run(func(args mock.Arguments) {
				items := args.Get(2).([]istructs.RecordGetBatchItem)
				record := &mockRecord{}
				record.On("QName").Return(testRecordQName1)
				record.On("AsInt64", "number").Return(int64(10))
				items[0].Record = record
			}).
			On("GetBatch", istructs.WSID(2), true, mock.AnythingOfType("[]istructs.RecordGetBatchItem")).
			Return(nil).
			Run(func(args mock.Arguments) {
				items := args.Get(2).([]istructs.RecordGetBatchItem)
				record1 := &mockRecord{}
				record1.
					On("QName").Return(testRecordQName2).
					On("AsInt64", "age").Return(int64(20))
				items[0].Record = record1
				record2 := &mockRecord{}
				record2.
					On("QName").Return(testRecordQName2).
					On("AsInt64", "age").Return(int64(21))
				items[1].Record = record2
			})
		schema1 := &mockSchema{}
		schema1.On("Fields", mock.AnythingOfType("")).Run(func(args mock.Arguments) {
			cb := args.Get(0).(func(fieldName string, kind istructs.DataKindType))
			cb("number", istructs.DataKind_int64)
		})
		schema2 := &mockSchema{}
		schema2.On("Fields", mock.AnythingOfType("")).Run(func(args mock.Arguments) {
			cb := args.Get(0).(func(fieldName string, kind istructs.DataKindType))
			cb("age", istructs.DataKind_int64)
		})
		schemas := &mockSchemas{}
		schemas.
			On("Schema", testRecordQName1).Return(schema1).
			On("Schema", testRecordQName2).Return(schema2)
		appStructs := &mockAppStructs{}
		appStructs.
			On("Records").Return(records).
			On("Schemas").Return(schemas).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})
		s := ProvideQueryProcessorStateFactory()(context.Background(), appStructs, nil, SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil)
		k1, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k1.PutRecordID(Field_ID, 1)
		k2, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k2.PutRecordID(Field_ID, 2)
		k2.PutInt64(Field_WSID, 2)
		k3, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k3.PutRecordID(Field_ID, 3)
		k3.PutInt64(Field_WSID, 2)

		rr := make([]result, 0)
		_ = s.CanExistAll([]istructs.IStateKeyBuilder{k1, k2, k3}, func(key istructs.IKeyBuilder, value istructs.IStateValue, ok bool) (err error) {
			rr = append(rr, result{
				key:    key.(*recordsKeyBuilder),
				value:  value,
				exists: ok,
			})
			return
		})

		require.Len(rr, 3)
		require.Equal(istructs.RecordID(1), rr[0].key.(*recordsKeyBuilder).id)
		require.Equal(istructs.WSID(1), rr[0].key.(*recordsKeyBuilder).wsid)
		require.True(rr[0].exists)
		require.Equal(int64(10), rr[0].value.AsInt64("number"))
		require.Equal(istructs.RecordID(2), rr[1].key.(*recordsKeyBuilder).id)
		require.Equal(istructs.WSID(2), rr[1].key.(*recordsKeyBuilder).wsid)
		require.True(rr[1].exists)
		require.Equal(int64(20), rr[1].value.AsInt64("age"))
		require.Equal(istructs.RecordID(3), rr[2].key.(*recordsKeyBuilder).id)
		require.Equal(istructs.WSID(2), rr[2].key.(*recordsKeyBuilder).wsid)
		require.True(rr[2].exists)
		require.Equal(int64(21), rr[2].value.AsInt64("age"))
	})
	t.Run("Should handle singleton records", func(t *testing.T) {
		require := require.New(t)
		toJSON := func(sv istructs.IStateValue) string {
			json, _ := sv.ToJSON()
			return json
		}
		singleton1 := &mockRecord{}
		singleton1.
			On("QName").Return(testRecordQName1).
			On("AsQName", mock.Anything).Return(testRecordQName1).
			On("AsInt64", "number").Return(int64(10)).
			On("FieldNames", mock.Anything).Run(func(a mock.Arguments) {
			x := a.Get(0).(func(name string))
			x("number")
		})
		singleton2 := &mockRecord{}
		singleton2.
			On("QName").Return(testRecordQName2).
			On("AsQName", mock.Anything).Return(testRecordQName2).
			On("AsInt64", "age").Return(int64(18)).
			On("FieldNames", mock.Anything).Run(func(a mock.Arguments) {
			x := a.Get(0).(func(name string))
			x("age")
		})
		nullRecord := &mockRecord{}
		nullRecord.On("QName").Return(istructs.NullQName)
		records := &mockRecords{}
		records.
			On("GetSingleton", istructs.WSID(1), testRecordQName1).Return(singleton1, nil).
			On("GetSingleton", istructs.WSID(2), testRecordQName2).Return(nullRecord, nil).
			On("GetSingleton", istructs.WSID(3), testRecordQName2).Return(singleton2, nil)
		schema1 := &mockSchema{}
		schema1.On("Fields", mock.AnythingOfType("")).Run(func(args mock.Arguments) {
			cb := args.Get(0).(func(fieldName string, kind istructs.DataKindType))
			cb("number", istructs.DataKind_int64)
		}).
			On("FieldKind", mock.Anything).Return(istructs.DataKind_int64)
		schema2 := &mockSchema{}
		schema2.On("Fields", mock.AnythingOfType("")).Run(func(args mock.Arguments) {
			cb := args.Get(0).(func(fieldName string, kind istructs.DataKindType))
			cb("age", istructs.DataKind_int64)
		}).
			On("FieldKind", mock.Anything).Return(istructs.DataKind_int64)
		schemas := &mockSchemas{}
		schemas.
			On("Schema", testRecordQName1).Return(schema1).
			On("Schema", testRecordQName2).Return(schema2)
		appsTructs := &mockAppStructs{}
		appsTructs.
			On("Records").Return(records).
			On("Schemas").Return(schemas).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})
		s := ProvideQueryProcessorStateFactory()(context.Background(), appsTructs, nil, SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil)
		k1, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k1.PutQName(Field_Singleton, testRecordQName1)
		k2, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k2.PutQName(Field_Singleton, testRecordQName2)
		k2.PutInt64(Field_WSID, 2)
		k3, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k3.PutQName(Field_Singleton, testRecordQName2)
		k3.PutInt64(Field_WSID, 3)

		rr := make([]result, 0)
		_ = s.CanExistAll([]istructs.IStateKeyBuilder{k1, k2, k3}, func(key istructs.IKeyBuilder, value istructs.IStateValue, ok bool) (err error) {
			rr = append(rr, result{
				key:    key.(*recordsKeyBuilder),
				value:  value,
				exists: ok,
			})
			return
		})

		require.Len(rr, 3)
		require.Equal(int64(10), rr[0].value.AsInt64("number"))
		require.True(rr[0].exists)
		require.JSONEq(`{"number":10}`, toJSON(rr[0].value))
		require.Equal(istructs.WSID(2), rr[1].key.(*recordsKeyBuilder).wsid)
		require.Nil(rr[1].value)
		require.False(rr[1].exists)
		require.Equal(istructs.WSID(3), rr[2].key.(*recordsKeyBuilder).wsid)
		require.True(rr[2].exists)
		require.Equal(int64(18), rr[2].value.AsInt64("age"))
		require.JSONEq(`{"age":18}`, toJSON(rr[2].value))
	})
	t.Run("Should return error when 'id' not found", func(t *testing.T) {
		require := require.New(t)
		s := ProvideQueryProcessorStateFactory()(context.Background(), &nilAppStructs{}, nil, SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil)
		k, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)

		_, ok, err := s.CanExist(k)

		require.False(ok)
		require.ErrorIs(err, ErrNotFound)
	})
	t.Run("Should return error on get batch", func(t *testing.T) {
		require := require.New(t)
		records := &mockRecords{}
		records.On("GetBatch", istructs.WSID(1), true, mock.AnythingOfType("[]istructs.RecordGetBatchItem")).Return(errTest)
		appsTructs := &mockAppStructs{}
		appsTructs.
			On("Records").Return(records).
			On("Schemas").Return(&nilSchemas{}).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})
		s := ProvideQueryProcessorStateFactory()(context.Background(), appsTructs, nil, SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil)
		k, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k.PutRecordID(Field_ID, istructs.RecordID(1))

		_, ok, err := s.CanExist(k)

		require.False(ok)
		require.ErrorIs(err, errTest)
	})
	t.Run("Should return error on get singleton", func(t *testing.T) {
		require := require.New(t)
		records := &mockRecords{}
		records.On("GetSingleton", istructs.WSID(1), testRecordQName1).Return(&mockRecord{}, errTest)
		appStructs := &mockAppStructs{}
		appStructs.
			On("Records").Return(records).
			On("Schemas").Return(&nilSchemas{}).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})
		s := ProvideQueryProcessorStateFactory()(context.Background(), appStructs, nil, SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil)
		k, _ := s.KeyBuilder(RecordsStorage, istructs.NullQName)
		k.PutQName(Field_Singleton, testRecordQName1)

		_, ok, err := s.CanExist(k)

		require.False(ok)
		require.ErrorIs(err, errTest)
	})
}
func TestRecordsStorage_Insert(t *testing.T) {
	fieldName := "name"
	value := "Heuus"
	rw := &mockRowWriter{}
	rw.
		On("PutString", fieldName, value)
	cud := &mockCUD{}
	cud.On("Create").Return(rw)
	s := ProvideCommandProcessorStateFactory()(context.Background(), nil, nil, SimpleWSIDFunc(istructs.NullWSID), nil, func() istructs.ICUD { return cud }, nil, nil, 1)
	kb, _ := s.KeyBuilder(RecordsStorage, testRecordQName1)

	vb, _ := s.NewValue(kb)
	vb.PutString(fieldName, value)

	_ = s.ValidateIntents()
	_ = s.ApplyIntents()
	rw.AssertExpectations(t)
}
func TestRecordsStorage_Update(t *testing.T) {
	fieldName := "name"
	value := "Heuus"
	rw := &mockRowWriter{}
	rw.On("PutString", fieldName, value)
	r := &mockRecord{}
	sv := &recordsStorageValue{record: r}
	cud := &mockCUD{}
	cud.On("Update", r).Return(rw)
	s := ProvideCommandProcessorStateFactory()(context.Background(), nil, nil, SimpleWSIDFunc(istructs.NullWSID), nil, func() istructs.ICUD { return cud }, nil, nil, 1)
	kb, _ := s.KeyBuilder(RecordsStorage, testRecordQName1)

	vb, _ := s.UpdateValue(kb, sv)
	vb.PutString(fieldName, value)

	_ = s.ValidateIntents()
	_ = s.ApplyIntents()
	rw.AssertExpectations(t)
}
