/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package storages

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys"
)

func createAppDef() appdef.IAppDef {
	appDef := appdef.New()
	appDef.AddObject(testRecordQName1).
		AddField("number", appdef.DataKind_int64, false)
	appDef.AddObject(testRecordQName2).
		AddField("age", appdef.DataKind_int64, false)
	wsDesc := appDef.AddCDoc(testWSDescriptorQName)
	wsDesc.AddField(field_WSKind, appdef.DataKind_bytes, false)
	ws := appDef.AddWorkspace(testWSQName)
	ws.AddType(testRecordQName1)
	ws.AddType(testRecordQName2)
	ws.SetDescriptor(testWSDescriptorQName)

	app, err := appDef.Build()
	if err != nil {
		panic(err)
	}
	return app
}

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

		appDef := appdef.New()
		appDef.AddObject(testRecordQName1).
			AddField("number", appdef.DataKind_int64, false)
		appDef.AddObject(testRecordQName2).
			AddField("age", appdef.DataKind_int64, false)

		app, err := appDef.Build()
		require.NoError(err)

		appStructs := &mockAppStructs{}
		appStructs.
			On("AppDef").Return(app).
			On("AppQName").Return(testAppQName).
			On("Records").Return(records).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})

		appStructsFunc := func() istructs.IAppStructs {
			return appStructs
		}
		storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
		k1 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k1.PutRecordID(sys.Storage_Record_Field_ID, 1)
		k2 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k2.PutRecordID(sys.Storage_Record_Field_ID, 2)
		k2.PutInt64(sys.Storage_Record_Field_WSID, 2)
		k3 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k3.PutRecordID(sys.Storage_Record_Field_ID, 3)
		k3.PutInt64(sys.Storage_Record_Field_WSID, 2)

		rr := make([]result, 0)
		batchItems := []state.GetBatchItem{
			{Key: k1},
			{Key: k2},
			{Key: k3},
		}
		err = storage.(state.IWithGetBatch).GetBatch(batchItems)
		require.NoError(err)
		require.Equal(int64(10), rr[0].value.AsInt64("number"))
		require.Equal(int64(20), rr[1].value.AsInt64("age"))
		require.Equal(int64(21), rr[2].value.AsInt64("age"))
	})
	t.Run("Should handle singleton records", func(t *testing.T) {
		require := require.New(t)
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
		nullRecord.On("QName").Return(appdef.NullQName)

		mockWorkspaceRecord := &mockRecord{}
		mockWorkspaceRecord.On("AsQName", "WSKind").Return(testWSDescriptorQName)
		mockWorkspaceRecord.On("QName").Return(qNameCDocWorkspaceDescriptor)

		records := &mockRecords{}
		records.
			On("GetSingleton", istructs.WSID(1), testRecordQName1).Return(singleton1, nil).
			On("GetSingleton", istructs.WSID(2), testRecordQName2).Return(nullRecord, nil).
			On("GetSingleton", istructs.WSID(3), testRecordQName2).Return(singleton2, nil).
			On("GetSingleton", mock.Anything, qNameCDocWorkspaceDescriptor).Return(mockWorkspaceRecord, nil)

		appStructs := &mockAppStructs{}
		appStructs.
			On("AppDef").Return(createAppDef()).
			On("AppQName").Return(testAppQName).
			On("Records").Return(records).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})

		appStructsFunc := func() istructs.IAppStructs {
			return appStructs
		}
		storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
		k1 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k1.PutQName(sys.Storage_Record_Field_Singleton, testRecordQName1)
		k2 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k2.PutQName(sys.Storage_Record_Field_Singleton, testRecordQName2)
		k2.PutInt64(sys.Storage_Record_Field_WSID, 2)
		k3 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k3.PutQName(sys.Storage_Record_Field_Singleton, testRecordQName2)
		k3.PutInt64(sys.Storage_Record_Field_WSID, 3)
		k4 := storage.NewKeyBuilder(appdef.NullQName, nil)
		k4.PutBool(sys.Storage_Record_Field_IsSingleton, true)
		rr := make([]result, 0)
		batchItems := []state.GetBatchItem{
			{Key: k1},
			{Key: k2},
			{Key: k3},
			{Key: k4},
		}
		err := storage.(state.IWithGetBatch).GetBatch(batchItems)
		require.NoError(err)
		require.Equal(int64(10), rr[0].value.AsInt64("number"))
		require.Equal(int64(18), rr[2].value.AsInt64("age"))
	})
	t.Run("Should return error when 'id' not found", func(t *testing.T) {
		require := require.New(t)
		appStructs := &mockAppStructs{}
		appStructsFunc := func() istructs.IAppStructs {
			return appStructs
		}
		storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
		k := storage.NewKeyBuilder(appdef.NullQName, nil)
		_, err := storage.(state.IWithGet).Get(k)
		require.ErrorIs(err, ErrNotFound)
	})
	t.Run("Should return error on get", func(t *testing.T) {
		require := require.New(t)
		records := &mockRecords{}
		records.On("Get", istructs.WSID(1), true, mock.Anything).Return(nil, errTest)
		appStructs := &mockAppStructs{}
		appStructs.
			On("AppDef").Return(createAppDef()).
			On("AppQName").Return(testAppQName).
			On("Records").Return(records).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})
		appStructsFunc := func() istructs.IAppStructs {
			return appStructs
		}
		storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
		k := storage.NewKeyBuilder(appdef.NullQName, nil)
		k.PutRecordID(sys.Storage_Record_Field_ID, istructs.RecordID(1))
		_, err := storage.(state.IWithGet).Get(k)
		require.ErrorIs(err, errTest)
	})
	t.Run("Should return error on get singleton", func(t *testing.T) {
		require := require.New(t)

		mockWorkspaceRecord := &mockRecord{}
		mockWorkspaceRecord.On("AsQName", "WSKind").Return(testWSDescriptorQName)
		mockWorkspaceRecord.On("QName").Return(qNameCDocWorkspaceDescriptor)

		records := &mockRecords{}
		records.On("GetSingleton", istructs.WSID(1), testRecordQName1).Return(&mockRecord{}, errTest)
		records.On("GetSingleton", mock.Anything, qNameCDocWorkspaceDescriptor).Return(mockWorkspaceRecord, nil)
		appStructs := &mockAppStructs{}
		appStructs.
			On("AppDef").Return(createAppDef()).
			On("AppQName").Return(testAppQName).
			On("Records").Return(records).
			On("ViewRecords").Return(&nilViewRecords{}).
			On("Events").Return(&nilEvents{})
		appStructsFunc := func() istructs.IAppStructs {
			return appStructs
		}
		storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
		k := storage.NewKeyBuilder(appdef.NullQName, nil)
		k.PutQName(sys.Storage_Record_Field_Singleton, testRecordQName1)
		_, err := storage.(state.IWithGet).Get(k)
		require.ErrorIs(err, errTest)
	})
}
func TestRecordsStorage_Insert(t *testing.T) {
	require := require.New(t)
	fieldName := "name"
	value := "Voedger"
	rw := &mockRowWriter{}
	rw.
		On("PutString", fieldName, value)
	cud := &mockCUD{}
	cud.On("Create").Return(rw)
	appStructs := &mockAppStructs{}
	appStructsFunc := func() istructs.IAppStructs {
		return appStructs
	}
	storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
	kb := storage.NewKeyBuilder(testRecordQName1, nil)
	vb, err := storage.(state.IWithInsert).ProvideValueBuilder(kb, nil)
	require.NoError(err)
	vb.PutString(fieldName, value)
	rw.AssertExpectations(t)
}
func TestRecordsStorage_Update(t *testing.T) {
	require := require.New(t)
	fieldName := "name"
	value := "Voedger"
	rw := &mockRowWriter{}
	rw.On("PutString", fieldName, value)
	r := &mockRecord{}
	sv := &recordsValue{record: r}
	cud := &mockCUD{}
	cud.On("Update", mock.Anything).Return(rw)

	appStructs := &mockAppStructs{}
	appStructsFunc := func() istructs.IAppStructs {
		return appStructs
	}
	storage := NewRecordsStorage(appStructsFunc, state.SimpleWSIDFunc(istructs.WSID(1)), nil)
	kb := storage.NewKeyBuilder(testRecordQName1, nil)
	vb, err := storage.(state.IWithUpdate).ProvideValueBuilderForUpdate(kb, sv, nil)
	require.NoError(err)
	vb.PutString(fieldName, value)
	rw.AssertExpectations(t)
}

func TestRecordsStorage_ValidateInWorkspaces_Reads(t *testing.T) {
	require := require.New(t)

	mockedStructs, mockedViews := mockedStructs(t)
	mockedViews.
		On("KeyBuilder", mock.Anything).Return(&nilKeyBuilder{}).
		On("NewValueBuilder", mock.Anything).Return(&nilValueBuilder{}).
		On("Get", istructs.WSID(1), mock.Anything).Return(&nilValue{}, nil).
		On("PutBatch", mock.Anything, mock.Anything).Return(nil)

	s := ProvideAsyncActualizerStateFactory()(context.Background(), appStructsFunc(mockedStructs), nil, SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil, nil, nil, 10, 10)

	wrongSingleton := appdef.NewQName("test", "RecordX")
	wrongKb, err := s.KeyBuilder(sys.Storage_Record, appdef.NullQName)
	wrongKb.PutQName(sys.Storage_Record_Field_Singleton, wrongSingleton)
	require.NoError(err)
	expectedError := typeIsNotDefinedInWorkspaceWithDescriptor(wrongSingleton, testWSDescriptorQName)

	t.Run("CanExist should validate for unavailable records", func(t *testing.T) {
		value, ok, err := s.CanExist(wrongKb)
		require.EqualError(err, expectedError.Error())
		require.Nil(value)
		require.False(ok)
	})

	t.Run("CanExistAll should validate for unavailable records", func(t *testing.T) {
		err = s.CanExistAll([]istructs.IStateKeyBuilder{wrongKb}, nil)
		require.EqualError(err, expectedError.Error())
	})

	t.Run("MustExist should validate for unavailable records", func(t *testing.T) {
		value, err := s.MustExist(wrongKb)
		require.EqualError(err, expectedError.Error())
		require.Nil(value)
	})

	t.Run("MustNotExist should validate for unavailable records", func(t *testing.T) {
		err := s.MustNotExist(wrongKb)
		require.EqualError(err, expectedError.Error())
	})

	t.Run("MustExistAll should validate for unavailable records", func(t *testing.T) {
		err = s.MustExistAll([]istructs.IStateKeyBuilder{wrongKb}, nil)
		require.EqualError(err, expectedError.Error())
	})

	t.Run("MustNotExistAll should validate for unavailable records", func(t *testing.T) {
		err = s.MustNotExistAll([]istructs.IStateKeyBuilder{wrongKb})
		require.EqualError(err, expectedError.Error())
	})
}

func TestRecordsStorage_ValidateInWorkspaces_Writes(t *testing.T) {
	require := require.New(t)

	mockedStructs, mockedViews := mockedStructs(t)
	mockedViews.
		On("KeyBuilder", mock.Anything).Return(&nilKeyBuilder{}).
		On("NewValueBuilder", mock.Anything).Return(&nilValueBuilder{}).
		On("Get", istructs.WSID(1), mock.Anything).Return(&nilValue{}, nil).
		On("PutBatch", mock.Anything, mock.Anything).Return(nil)

	s := ProvideCommandProcessorStateFactory()(context.Background(), appStructsFunc(mockedStructs), nil,
		SimpleWSIDFunc(istructs.WSID(1)), nil, nil, nil, nil, 10, nil, nil, nil, nil, nil)

	wrongSingleton := appdef.NewQName("test", "RecordX")
	wrongKb, err := s.KeyBuilder(sys.Storage_Record, wrongSingleton)
	require.NoError(err)
	expectedError := typeIsNotDefinedInWorkspaceWithDescriptor(wrongSingleton, testWSDescriptorQName)

	t.Run("NewValue should validate for unavailable records", func(t *testing.T) {
		builder, err := s.NewValue(wrongKb)
		require.EqualError(err, expectedError.Error())
		require.Nil(builder)
	})

}
