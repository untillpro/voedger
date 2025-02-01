/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package istoragecache

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/mem"
	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
	imetrics "github.com/voedger/voedger/pkg/metrics"
)

const (
	testCacheSize = 1000
)

func TestBasicUsage(t *testing.T) {
	t.Run("Should put record to storage and cache after that use cache for read", func(t *testing.T) {
		require := require.New(t)
		times := 0
		ts := &testStorage{
			put: func(pKey []byte, cCols []byte, value []byte) (err error) {
				require.Equal([]byte("UK"), pKey)
				require.Equal([]byte("Article"), cCols)
				require.Equal([]byte("Cola"), value)
				return err
			},
			get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
				times++
				return ok, err
			},
		}
		tsp := &testStorageProvider{storage: ts}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.NoError(err)

		require.NoError(storage.Put([]byte("UK"), []byte("Article"), []byte("Cola")))

		_, err = storage.Get([]byte("UK"), []byte("Article"), &[]byte{})
		require.NoError(err)
		_, err = storage.Get([]byte("UK"), []byte("Article"), &[]byte{})
		require.NoError(err)

		require.Equal(0, times)
	})
	t.Run("Should put records to storage and cache after that use cache for read", func(t *testing.T) {
		require := require.New(t)
		times := 0
		ts := &testStorage{
			putBatch: func(items []istorage.BatchItem) (err error) {
				require.Len(items, 1)
				return err
			},
			get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
				times++
				return ok, err
			},
		}
		tsp := &testStorageProvider{storage: ts}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.NoError(err)

		require.NoError(storage.PutBatch([]istorage.BatchItem{{
			PKey:  []byte("UK"),
			CCols: []byte("Article"),
			Value: []byte("Cola"),
		}}))

		_, err = storage.Get([]byte("UK"), []byte("Article"), &[]byte{})
		require.NoError(err)
		_, err = storage.Get([]byte("UK"), []byte("Article"), &[]byte{})
		require.NoError(err)

		require.Equal(0, times)
	})
	t.Run("Should get record from storage after that put it to cache and use cache for read", func(t *testing.T) {
		require := require.New(t)
		times := 0
		ts := &testStorage{get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
			times++
			*data = append(*data, []byte("Cola")...)
			return true, nil
		}}
		data := make([]byte, 0, 100)
		tsp := &testStorageProvider{storage: ts}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.NoError(err)

		_, err = storage.Get([]byte("UK"), []byte("Article"), &data)
		require.NoError(err)
		require.Equal([]byte("Cola"), data)

		data = make([]byte, 0, 100)
		_, err = storage.Get([]byte("UK"), []byte("Article"), &data)
		require.NoError(err)
		require.Equal([]byte("Cola"), data)

		require.Equal(1, times)
	})
	t.Run("Should get batch from cache when at least one item from batch is not in the cache then complete batch re-read from underlying storage after that put batch to cache and use cache for read", func(t *testing.T) {
		require := require.New(t)
		ts := &testStorage{
			put: func(pKey []byte, cCols []byte, value []byte) (err error) { return err },
			getBatch: func(pKey []byte, items []istorage.GetBatchItem) (err error) {
				*items[0].Data = append((*items[0].Data)[0:0], []byte("Cola")...)
				items[0].Ok = true
				items[1].Ok = false
				*items[2].Data = append((*items[2].Data)[0:0], []byte("Napkin")...)
				items[2].Ok = true
				return err
			},
		}
		data := make([]byte, 0, 100)
		tsp := &testStorageProvider{storage: ts}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.NoError(err)
		require.NoError(storage.Put([]byte("NL"), []byte("Beverage"), []byte("Cola")))
		items := []istorage.GetBatchItem{
			{
				CCols: []byte("Beverage"),
				Data:  &[]byte{},
			},
			{
				CCols: []byte("Food"),
				Data:  &[]byte{},
			},
			{
				CCols: []byte("Misc"),
				Data:  &[]byte{},
			},
		}

		require.NoError(storage.GetBatch([]byte("NL"), items))

		require.Equal([]byte("Cola"), *items[0].Data)
		require.Empty(items[1].Data)
		require.Equal([]byte("Napkin"), *items[2].Data)

		_, err = storage.Get([]byte("NL"), items[0].CCols, &data)
		require.NoError(err)
		require.Equal([]byte("Cola"), data)
		_, err = storage.Get([]byte("NL"), items[2].CCols, &data)
		require.NoError(err)
		require.Equal([]byte("Napkin"), data)
	})

	t.Run("error on app storage get error", func(t *testing.T) {
		testErr := errors.New("test error")
		tsp := &testStorageProvider{appStorageGetError: testErr}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.ErrorIs(t, err, testErr)
		require.Nil(t, storage)
	})
}

func TestAppStorage_Put(t *testing.T) {
	require := require.New(t)
	testErr := errors.New("test error")
	ts := &testStorage{
		put: func(pKey []byte, cCols []byte, value []byte) (err error) {
			return testErr
		},
		get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
			return false, err
		},
	}
	tsp := &testStorageProvider{storage: ts}
	cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
	storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)

	err = storage.Put([]byte("UK"), []byte("Article"), []byte("Cola"))

	require.ErrorIs(err, testErr)

	ok, err := storage.Get([]byte("UK"), []byte("Article"), &[]byte{})

	require.False(ok)
	require.NoError(err)
}

func TestAppStorage_PutBatch(t *testing.T) {
	require := require.New(t)
	testErr := errors.New("test error")
	ts := &testStorage{
		putBatch: func(items []istorage.BatchItem) (err error) {
			return testErr
		},
		get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
			return false, err
		},
	}
	tsp := &testStorageProvider{storage: ts}
	cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
	storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)

	err = storage.PutBatch([]istorage.BatchItem{{
		PKey:  []byte("UK"),
		CCols: []byte("Article"),
		Value: []byte("Cola"),
	}})

	require.ErrorIs(err, testErr)

	ok, err := storage.Get([]byte("UK"), []byte("Article"), &[]byte{})

	require.False(ok)
	require.NoError(err)
}

func TestAppStorage_Get(t *testing.T) {
	require := require.New(t)
	testErr := errors.New("test error")
	ts := &testStorage{get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
		return false, testErr
	}}
	tsp := &testStorageProvider{storage: ts}
	cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
	storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)

	ok, err := storage.Get([]byte("UK"), []byte("Article"), &[]byte{})

	require.False(ok)
	require.ErrorIs(err, testErr)
}

func TestAppStorage_GetBatch(t *testing.T) {
	t.Run("Should handle error", func(t *testing.T) {
		require := require.New(t)
		testErr := errors.New("test error")
		ts := &testStorage{getBatch: func(pKey []byte, items []istorage.GetBatchItem) (err error) {
			return testErr
		}}
		tsp := &testStorageProvider{storage: ts}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.NoError(err)

		err = storage.GetBatch([]byte("UK"), []istorage.GetBatchItem{{
			CCols: []byte("Article"),
			Data:  &[]byte{},
		}})

		require.ErrorIs(err, testErr)
	})
	t.Run("Should remove item from cache when it was not found from underlying storage", func(t *testing.T) {
		require := require.New(t)
		ts := &testStorage{
			get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
				return false, err
			},
			put: func(pKey []byte, cCols []byte, value []byte) (err error) {
				return err
			},
			getBatch: func(pKey []byte, items []istorage.GetBatchItem) (err error) {
				items[0].Ok = false
				*items[1].Data = append((*items[1].Data)[0:0], []byte("Burger ver.1.1")...)
				items[1].Ok = true
				*items[2].Data = append((*items[2].Data)[0:0], []byte("Napkin ver.1.0")...)
				items[2].Ok = true
				return err
			}}
		tsp := &testStorageProvider{storage: ts}
		cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.MockTime)
		storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
		require.NoError(err)

		require.NoError(storage.Put([]byte("UK"), []byte("Beverage"), []byte("Cola ver.1.0")))
		require.NoError(storage.Put([]byte("UK"), []byte("Food"), []byte("Burger ver.1.0")))

		require.NoError(storage.GetBatch([]byte("UK"), []istorage.GetBatchItem{
			{
				CCols: []byte("Beverage"),
				Data:  &[]byte{},
			},
			{
				CCols: []byte("Food"),
				Data:  &[]byte{},
			},
			{
				CCols: []byte("Misc"),
				Data:  &[]byte{},
			},
		}))

		data := make([]byte, 0, 100)

		ok, err := storage.Get([]byte("UK"), []byte("Beverage"), &data)
		require.NoError(err)
		require.False(ok)
		_, err = storage.Get([]byte("UK"), []byte("Food"), &data)
		require.NoError(err)
		require.Equal([]byte("Burger ver.1.1"), data)
		_, err = storage.Get([]byte("UK"), []byte("Misc"), &data)
		require.NoError(err)
		require.Equal([]byte("Napkin ver.1.0"), data)
	})
}

func TestTechnologyCompatibilityKit(t *testing.T) {
	asf := mem.Provide(coreutils.MockTime)
	asp := istorageimpl.Provide(asf)
	cachingStorageProvider := Provide(testCacheSize, asp, imetrics.Provide(), "vvm", asf.Time())
	storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(t, err)
	istorage.TechnologyCompatibilityKit_Storage(t, storage, asf.Time())
}

func TestCacheNils(t *testing.T) {
	require := require.New(t)
	dbQueriedTimes := 0
	ts := &testStorage{
		get: func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
			dbQueriedTimes++
			return false, err
		},
		getBatch: func(pKey []byte, items []istorage.GetBatchItem) (err error) {
			items[0].Ok = true
			*items[0].Data = []byte{1}
			items[1].Ok = false
			dbQueriedTimes++
			return nil
		},
	}
	tsp := &testStorageProvider{storage: ts}
	cachingStorageProvider := Provide(testCacheSize, tsp, imetrics.Provide(), "vvm", coreutils.NewITime())
	storage, err := cachingStorageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)

	t.Run("Get()", func(t *testing.T) {
		dbQueriedTimes = 0
		// first Get() call -> no data for the key -> missing key state is cached
		data := make([]byte, 0, 100)
		require.Equal(0, dbQueriedTimes)
		ok, err := storage.Get([]byte("missing"), []byte("missing"), &data)
		require.NoError(err)
		require.False(ok)
		require.Equal(1, dbQueriedTimes)

		// second Get() call by missing key -> missing key state should be taken from the cache, db should not be queried
		ok, err = storage.Get([]byte("missing"), []byte("missing"), &data)
		require.NoError(err)
		require.False(ok)
		require.Equal(1, dbQueriedTimes)
	})

	t.Run("GetBatch()", func(t *testing.T) {
		dbQueriedTimes = 0
		batch := []istorage.GetBatchItem{
			{
				CCols: []byte("Beverage"),
				Data:  &[]byte{},
			},
			{
				CCols: []byte("missing"),
				Data:  &[]byte{},
			},
		}
		// 1st call -> no data in the cache, db should be queried, missing key state should be cached
		require.NoError(storage.GetBatch([]byte("UK"), batch))
		require.True(batch[0].Ok)
		require.False(batch[1].Ok)
		require.Equal(1, dbQueriedTimes)

		// 2st call -> no data in the cache, db should be queried, missing key state should be cached
		require.NoError(storage.GetBatch([]byte("UK"), batch))
		require.True(batch[0].Ok)
		require.False(batch[1].Ok)
		require.Equal(1, dbQueriedTimes)
	})
}

func TestMakeKes(t *testing.T) {
	require := require.New(t)
	require.Equal([]byte{1, 2, 3, 4, 5, 6}, makeKey([]byte{1, 2, 3}, []byte{4, 5, 6}))
}

type testStorageProvider struct {
	storage            *testStorage
	appStorageGetError error
}

func (sp *testStorageProvider) Prepare(_ any) error { return nil }

func (sp *testStorageProvider) Run(_ context.Context) {}

func (sp *testStorageProvider) Stop() {}

func (sp *testStorageProvider) AppStorage(appdef.AppQName) (istorage.IAppStorage, error) {
	if sp.appStorageGetError != nil {
		return nil, sp.appStorageGetError
	}
	return sp.storage, nil
}

func (sp *testStorageProvider) Init(appQName appdef.AppQName) error {
	return nil
}

type testStorage struct {
	insertIfNotExists func(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error)
	compareAndSwap    func(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error)
	compareAndDelete  func(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error)
	ttlGet            func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error)
	ttlRead           func(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error)
	put               func(pKey []byte, cCols []byte, value []byte) (err error)
	putBatch          func(items []istorage.BatchItem) (err error)
	get               func(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error)
	getBatch          func(pKey []byte, items []istorage.GetBatchItem) (err error)
}

func (s *testStorage) InsertIfNotExists(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error) {
	return s.insertIfNotExists(pKey, cCols, value, ttlSeconds)
}

func (s *testStorage) CompareAndSwap(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error) {
	return s.compareAndSwap(pKey, cCols, oldValue, newValue, ttlSeconds)
}

func (s *testStorage) CompareAndDelete(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error) {
	return s.compareAndDelete(pKey, cCols, expectedValue)
}

func (s *testStorage) TTLGet(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	return s.ttlGet(pKey, cCols, data)
}

func (s *testStorage) TTLRead(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	return s.ttlRead(ctx, pKey, startCCols, finishCCols, cb)
}

func (s *testStorage) Put(pKey []byte, cCols []byte, value []byte) (err error) {
	return s.put(pKey, cCols, value)
}

func (s *testStorage) PutBatch(items []istorage.BatchItem) (err error) {
	return s.putBatch(items)
}

func (s *testStorage) Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	return s.get(pKey, cCols, data)
}

func (s *testStorage) GetBatch(pKey []byte, items []istorage.GetBatchItem) (err error) {
	return s.getBatch(pKey, items)
}

func (s *testStorage) Read(context.Context, []byte, []byte, []byte, istorage.ReadCallback) (err error) {
	return err
}
