/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package teststore

import (
	"bytes"
	"context"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/mem"
	"github.com/voedger/voedger/pkg/istorage/provider"
)

// Test storage. Trained to return the specified error
type (
	scheduleStorageError struct {
		err         error
		pKey, cCols []byte
	}

	damagedStorageFunc    func(*[]byte)
	scheduleStorageDamage struct {
		dam damagedStorageFunc
		scheduleStorageError
	}

	TestMemStorage struct {
		name     appdef.AppQName
		storage  istorage.IAppStorage
		get, put scheduleStorageError
		damage   scheduleStorageDamage
	}

	testStorageProvider struct {
		testStorage map[appdef.AppQName]*TestMemStorage
	}
)

//nolint:revive
func (s *TestMemStorage) InsertIfNotExists(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error) {
	//TODO implement me
	panic("implement me")
}

//nolint:revive
func (s *TestMemStorage) CompareAndSwap(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error) {
	//TODO implement me
	panic("implement me")
}

//nolint:revive
func (s *TestMemStorage) CompareAndDelete(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error) {
	//TODO implement me
	panic("implement me")
}

//nolint:revive
func (s *TestMemStorage) TTLGet(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	//TODO implement me
	panic("implement me")
}

//nolint:revive
func (s *TestMemStorage) TTLRead(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	//TODO implement me
	panic("implement me")
}

func (tsp testStorageProvider) Prepare(_ any) error { return nil }

func (tsp testStorageProvider) Run(_ context.Context) {}

func (tsp testStorageProvider) Stop() {}

func (tsp testStorageProvider) AppStorage(appName appdef.AppQName) (structs istorage.IAppStorage, err error) {
	if s, ok := tsp.testStorage[appName]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("%v: %w", appName, istorage.ErrStorageDoesNotExist)
}

// Returns new storage provider for specified test storage
func NewStorageProvider(ts *TestMemStorage) istorage.IAppStorageProvider {
	p := &testStorageProvider{testStorage: make(map[appdef.AppQName]*TestMemStorage)}
	p.testStorage[ts.name] = ts
	return p
}

// Returns new test storage
func NewStorage(appName appdef.AppQName) *TestMemStorage {
	s := &TestMemStorage{name: appName, get: scheduleStorageError{}, put: scheduleStorageError{}}
	asf := mem.Provide(coreutils.MockTime)
	sp := provider.Provide(asf)
	var err error
	if s.storage, err = sp.AppStorage(appName); err != nil {
		panic(err)
	}

	return s
}

// Returns new test storage and new test storage provider
func New(appName appdef.AppQName) (storage *TestMemStorage, provider istorage.IAppStorageProvider) {
	s := NewStorage(appName)
	return s, NewStorageProvider(s)
}

// Returns is partition key and clustering columns matches the scheduled error
func (e *scheduleStorageError) match(pKey, cCols []byte) bool {
	return ((len(e.pKey) == 0) || bytes.Equal(e.pKey, pKey)) &&
		((len(e.cCols) == 0) || bytes.Equal(e.cCols, cCols))
}

// Clear all scheduled errors
func (s *TestMemStorage) Reset() {
	s.get = scheduleStorageError{}
	s.put = scheduleStorageError{}
	s.damage = scheduleStorageDamage{}
}

// Schedule Get() to return error
func (s *TestMemStorage) ScheduleGetError(err error, pKey, cCols []byte) {
	s.get.err = err
	s.get.pKey = make([]byte, len(pKey))
	copy(s.get.pKey, pKey)
	s.get.cCols = make([]byte, len(cCols))
	copy(s.get.cCols, cCols)
}

// Schedule Get() to return damaged data
func (s *TestMemStorage) ScheduleGetDamage(dam damagedStorageFunc, pKey, cCols []byte) {
	s.damage.dam = dam
	s.damage.pKey = make([]byte, len(pKey))
	copy(s.damage.pKey, pKey)
	s.damage.cCols = make([]byte, len(cCols))
	copy(s.damage.cCols, cCols)
}

// Schedule Put() to return error
func (s *TestMemStorage) SchedulePutError(err error, pKey, cCols []byte) {
	s.put.err = err
	s.put.pKey = make([]byte, len(pKey))
	copy(s.put.pKey, pKey)
	s.put.cCols = make([]byte, len(cCols))
	copy(s.put.cCols, cCols)
}

func (s *TestMemStorage) Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	if s.get.err != nil {
		if s.get.match(pKey, cCols) {
			err = s.get.err
			s.get.err = nil
			return false, err
		}
	}

	ok, err = s.storage.Get(pKey, cCols, data)

	if ok && (s.damage.dam != nil) {
		if s.damage.match(pKey, cCols) {
			s.damage.dam(data)
			s.damage.dam = nil
			return ok, err
		}
	}

	return ok, err
}

func (s *TestMemStorage) GetBatch(pKey []byte, items []istorage.GetBatchItem) (err error) {
	if s.get.err != nil {
		for _, item := range items {
			if s.get.match(pKey, item.CCols) {
				err = s.get.err
				s.get.err = nil
				return err
			}
		}
	}

	err = s.storage.GetBatch(pKey, items)

	if s.damage.dam != nil {
		for i := 0; i < len(items); i++ {
			if s.damage.match(pKey, items[i].CCols) {
				if items[i].Ok {
					s.damage.dam(items[i].Data)
					s.damage.dam = nil
				}
			}
		}
	}

	return err
}

func (s *TestMemStorage) Put(pKey []byte, cCols []byte, value []byte) (err error) {
	if s.put.err != nil {
		if s.put.match(pKey, cCols) {
			err = s.put.err
			s.put.err = nil
			return err
		}
	}
	return s.storage.Put(pKey, cCols, value)
}

func (s *TestMemStorage) PutBatch(items []istorage.BatchItem) (err error) {
	for _, p := range items {
		if err = s.Put(p.PKey, p.CCols, p.Value); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestMemStorage) Read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	cbWrap := func(cCols []byte, data []byte) (err error) {
		if s.get.err != nil {
			if s.get.match(pKey, cCols) {
				err = s.get.err
				s.get.err = nil
				return err
			}
		}

		if s.damage.dam != nil {
			if s.damage.match(pKey, cCols) {
				s.damage.dam(&data)
				s.damage.dam = nil
			}
		}

		return cb(cCols, data)
	}

	return s.storage.Read(ctx, pKey, startCCols, finishCCols, cbWrap)
}
