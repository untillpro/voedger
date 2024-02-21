/*
  - Copyright (c) 2023-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/

package iextenginewazero

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

var errTestIOError = errors.New("test i/o error")

var storageEvent = appdef.NewQName("sys", "EventStorage")
var storageSendmail = appdef.NewQName("sys", "SendMailStorage")
var storageRecords = appdef.NewQName("sys", "RecordsStorage")
var storageTest = appdef.NewQName("sys", "Test")
var storageTest2 = appdef.NewQName("sys", "Test2")
var storageTest3 = appdef.NewQName("sys", "Test3")
var storageIoError = appdef.NewQName("sys", "IoErrorStorage")

var projectorMode bool

type mockIo struct {
	istructs.IState
	istructs.IIntents
	intents []intent
}

func testModuleURL(path string) (u *url.URL) {

	path, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}

	u, err = url.Parse("file:///" + filepath.ToSlash(path))
	if err != nil {
		panic(err)
	}

	return

}

func (s *mockIo) KeyBuilder(storage, entity appdef.QName) (builder istructs.IStateKeyBuilder, err error) {
	return &mockKeyBuilder{
		entity:  entity,
		storage: storage,
	}, nil
}

func mockedValue(name string, value interface{}) istructs.IStateValue {
	mv := mockValue{
		TestObject: coreutils.TestObject{Data: map[string]interface{}{}},
	}
	mv.Data[name] = value
	return &mv
}

func (s *mockIo) CanExist(key istructs.IStateKeyBuilder) (value istructs.IStateValue, ok bool, err error) {
	k := key.(*mockKeyBuilder)
	mv := mockValue{
		TestObject: coreutils.TestObject{Data: map[string]interface{}{}},
	}
	if k.storage == storageIoError {
		return nil, false, errTestIOError
	}
	if k.storage == storageEvent {
		if projectorMode {
			mv.Data["offs"] = int32(12345)
			mv.Data["qname"] = "air.UpdateSubscription"
			mv.Data["arg"] = newJsonValue(`
				{
					"subscription": {
						"status": "active"
					},
					"customer": {
						"email": "customer@test.com"
					}
				}
			`)
			return &mv, true, nil

		}
		mv.Data["qname"] = "sys.InvitationAccepted"
		mv.Data["arg"] = mockedValue("UserEmail", "email@user.com")
		mv.Data["offs"] = int32(12345)
		return &mv, true, nil
	}
	if k.storage == storageTest {
		mv.Data["500c"] = "01234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"
		mv.Data["bytes"] = make([]byte, WasmPreallocatedBufferSize*2)
		return &mv, true, nil
	}
	if k.storage == storageTest3 {
		mv.index = make([]interface{}, 4)
		mv.index[0] = int32(123)
		mv.index[1] = "test string"
		mv.index[2] = make([]byte, 1024)
		return &mv, true, nil
	}
	if k.storage == storageTest2 {
		const vvv = "012345678901234567890"
		mv.Data["а10"] = vvv
		mv.Data["а11"] = vvv
		mv.Data["а12"] = vvv
		mv.Data["а13"] = vvv
		mv.Data["а14"] = vvv
		mv.Data["а15"] = vvv
		mv.Data["а16"] = vvv
		mv.Data["а17"] = vvv
		mv.Data["а18"] = vvv
		mv.Data["а19"] = vvv
		mv.Data["а20"] = vvv
		mv.Data["а21"] = vvv
		mv.Data["а22"] = vvv
		mv.Data["а23"] = vvv
		mv.Data["а24"] = vvv
		mv.Data["а25"] = vvv
		mv.Data["а26"] = vvv
		mv.Data["а27"] = vvv
		mv.Data["а28"] = vvv
		mv.Data["а29"] = vvv
		mv.Data["а30"] = vvv
		mv.Data["а31"] = vvv
		mv.Data["а32"] = vvv
		mv.Data["а33"] = vvv
		mv.Data["а34"] = vvv
		mv.Data["а35"] = vvv
		mv.Data["а36"] = vvv
		mv.Data["а37"] = vvv
		mv.Data["а38"] = vvv
		mv.Data["а39"] = vvv
		return &mv, true, nil
	}
	if k.storage == storageSendmail {
		return &mv, true, nil
	}
	if k.storage == storageRecords {
		return &mv, false, nil
	}
	return nil, false, errors.New("unsupported storage: " + k.storage.Pkg() + "." + k.storage.Entity())
}

func (s *mockIo) CanExistAll(keys []istructs.IStateKeyBuilder, callback istructs.StateValueCallback) (err error) {
	return nil
}

func (s *mockIo) MustExist(key istructs.IStateKeyBuilder) (value istructs.IStateValue, err error) {
	k := key.(*mockKeyBuilder)
	if k.storage == storageIoError {
		return nil, errTestIOError
	}
	if k.storage == storageRecords {
		return nil, state.ErrNotExists
	}
	v, ok, err := s.CanExist(key)
	if err != nil {
		return v, err
	}
	if !ok {
		panic("not exists")
	}

	return v, nil
}

func (s *mockIo) MustExistAll(keys []istructs.IStateKeyBuilder, callback istructs.StateValueCallback) (err error) {
	return nil
}

func (s *mockIo) MustNotExist(key istructs.IStateKeyBuilder) (err error) {
	return nil
}

func (s *mockIo) MustNotExistAll(keys []istructs.IStateKeyBuilder) (err error) {
	return nil
}

func (s *mockIo) Read(key istructs.IStateKeyBuilder, callback istructs.ValueCallback) (err error) {
	k := key.(*mockKeyBuilder)
	if k.storage == storageIoError {
		return errTestIOError
	}
	if k.storage == storageTest {
		for i := 1; i <= 3; i++ {
			mk := coreutils.TestObject{Data: map[string]interface{}{}}
			mk.Data["i32"] = int32(i)
			mk.Data["i64"] = 10 + int64(i)
			mk.Data["f32"] = float32(i) + 0.1
			mk.Data["f64"] = float64(i) + 0.01
			mk.Data["str"] = fmt.Sprintf("key%d", i)
			mk.Data["bytes"] = []byte{byte(i), 2, 3}
			mk.Data["qname"] = appdef.NewQName("keypkg", fmt.Sprintf("e%d", i))
			mk.Data["bool"] = true

			mv := mockValue{
				TestObject: coreutils.TestObject{Data: map[string]interface{}{}},
			}
			mv.Data["i32"] = 100 + int32(i)
			mv.Data["i64"] = 1000 + int64(i)
			mv.Data["f32"] = float32(i) + 0.001
			mv.Data["f64"] = float64(i) + 0.0001
			mv.Data["str"] = fmt.Sprintf("value%d", i)
			mv.Data["bytes"] = []byte{3, 2, 1}
			mv.Data["qname"] = appdef.NewQName("valuepkg", fmt.Sprintf("ee%d", i))
			mv.Data["bool"] = false
			if err := callback(&mk, &mv); err != nil {
				return err
			}
		}

	}
	return nil
}

type mockKeyBuilder struct {
	entity  appdef.QName
	storage appdef.QName
}

func (kb *mockKeyBuilder) Storage() appdef.QName                            { return kb.storage }
func (kb *mockKeyBuilder) Entity() appdef.QName                             { return kb.entity }
func (kb *mockKeyBuilder) PartitionKey() istructs.IRowWriter                { return nil }
func (kb *mockKeyBuilder) ClusteringColumns() istructs.IRowWriter           { return nil }
func (kb *mockKeyBuilder) Equals(src istructs.IKeyBuilder) bool             { return false }
func (kb *mockKeyBuilder) PutInt32(name string, value int32)                {}
func (kb *mockKeyBuilder) PutInt64(name string, value int64)                {}
func (kb *mockKeyBuilder) PutFloat32(name string, value float32)            {}
func (kb *mockKeyBuilder) PutFloat64(name string, value float64)            {}
func (kb *mockKeyBuilder) PutBytes(name string, value []byte)               {}
func (kb *mockKeyBuilder) PutString(name, value string)                     {}
func (kb *mockKeyBuilder) PutQName(name string, value appdef.QName)         {}
func (kb *mockKeyBuilder) PutBool(name string, value bool)                  {}
func (kb *mockKeyBuilder) PutRecordID(name string, value istructs.RecordID) {}

// Tries to make conversion from value to a name type
func (kb *mockKeyBuilder) PutNumber(name string, value float64) {}

// Tries to make conversion from value to a name type
func (kb *mockKeyBuilder) PutChars(name string, value string) {}

func (kb *mockKeyBuilder) PutFromJSON(map[string]any) {}

func newJsonValue(jsonString string) istructs.IStateValue {
	v := mockValue{TestObject: coreutils.TestObject{Data: map[string]interface{}{}}}
	err := json.Unmarshal([]byte(jsonString), &v.Data)
	if err != nil {
		panic(err)
	}
	return &v
}

type mockValue struct {
	coreutils.TestObject
	index []interface{}
}

func (v *mockValue) ToJSON(opts ...interface{}) (string, error)     { return "", nil }
func (v *mockValue) AsRecord(name string) (record istructs.IRecord) { return nil }
func (v *mockValue) AsEvent(name string) (event istructs.IDbEvent)  { return nil }

func (v *mockValue) GetAsInt32(index int) int32        { return v.index[index].(int32) }
func (v *mockValue) GetAsInt64(index int) int64        { return 0 }
func (v *mockValue) GetAsFloat32(index int) float32    { return 0 }
func (v *mockValue) GetAsFloat64(index int) float64    { return 0 }
func (v *mockValue) GetAsBytes(index int) []byte       { return v.index[index].([]byte) }
func (v *mockValue) GetAsString(index int) string      { return v.index[index].(string) }
func (v *mockValue) GetAsQName(index int) appdef.QName { return appdef.NullQName }
func (v *mockValue) GetAsBool(index int) bool          { return false }

func (v *mockValue) Length() int                              { return 0 }
func (v *mockValue) AsRecordID(name string) istructs.RecordID { return 0 }
func (v *mockValue) GetAsValue(index int) istructs.IStateValue {
	iv, ok := v.index[index].(istructs.IStateValue)
	if ok {
		return iv
	}
	mv, ok := v.index[index].([]interface{})
	if ok {
		return &mockValue{
			index: mv,
		}
	}
	panic(fmt.Sprintf("unsupported value stored under index: %d", index))
}
func (v *mockValue) AsValue(name string) istructs.IStateValue {
	iv, ok := v.Data[name].(istructs.IStateValue)
	if ok {
		return iv
	}
	mv, ok := v.Data[name].(map[string]interface{})
	if ok {
		return &mockValue{
			TestObject: coreutils.TestObject{Data: mv},
		}
	}
	panic("unsupported value stored under key: " + name)
}
func (v *mockValue) RecordIDs(includeNulls bool, cb func(name string, value istructs.RecordID)) {}
func (v *mockValue) FieldNames(cb func(fieldName string)) {
	v.TestObject.FieldNames(cb)
}

type intent struct {
	key   istructs.IStateKeyBuilder
	value istructs.IStateValueBuilder
}

func (s *mockIo) NewValue(key istructs.IStateKeyBuilder) (builder istructs.IStateValueBuilder, err error) {
	k := key.(*mockKeyBuilder)
	if k.storage == storageIoError {
		return nil, errTestIOError
	}
	vb := mockValueBuilder{
		items: make(map[string]interface{}),
	}
	s.intents = append(s.intents, intent{
		key:   key,
		value: &vb,
	})
	return &vb, nil
}

func (s *mockIo) UpdateValue(key istructs.IStateKeyBuilder, existingValue istructs.IStateValue) (builder istructs.IStateValueBuilder, err error) {
	k := key.(*mockKeyBuilder)
	if k.storage == storageIoError {
		return nil, errTestIOError
	}
	vb := mockValueBuilder{
		items: make(map[string]interface{}),
	}
	mv := existingValue.(*mockValue)
	for k, v := range mv.Data {
		vb.items[k] = v
	}
	s.intents = append(s.intents, intent{
		key:   key,
		value: &vb,
	})
	return &vb, nil
}

type mockValueBuilder struct {
	items map[string]interface{}
}

func (kb *mockValueBuilder) BuildValue() istructs.IStateValue                 { return nil }
func (kb *mockValueBuilder) PutRecord(name string, record istructs.IRecord)   {}
func (kb *mockValueBuilder) PutEvent(name string, event istructs.IDbEvent)    {}
func (kb *mockValueBuilder) Build() istructs.IValue                           { return nil }
func (kb *mockValueBuilder) PutInt32(name string, value int32)                { kb.items[name] = value }
func (kb *mockValueBuilder) PutInt64(name string, value int64)                {}
func (kb *mockValueBuilder) PutFloat32(name string, value float32)            {}
func (kb *mockValueBuilder) PutFloat64(name string, value float64)            {}
func (kb *mockValueBuilder) PutBytes(name string, value []byte)               { kb.items[name] = value }
func (kb *mockValueBuilder) PutString(name, value string)                     { kb.items[name] = value }
func (kb *mockValueBuilder) PutQName(name string, value appdef.QName)         {}
func (kb *mockValueBuilder) PutBool(name string, value bool)                  {}
func (kb *mockValueBuilder) PutRecordID(name string, value istructs.RecordID) {}
func (kb *mockValueBuilder) PutFromJSON(map[string]any)                       {}

// Tries to make conversion from value to a name type
func (kb *mockValueBuilder) PutNumber(name string, value float64) {}

// Tries to make conversion from value to a name type
func (kb *mockValueBuilder) PutChars(name string, value string) {}
