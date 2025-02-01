/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package istorage

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
)

// TechnologyCompatibilityKit test suit
func TechnologyCompatibilityKit(t *testing.T, storageFactory IAppStorageFactory) {
	testAppQName := appdef.NewAppQName("tcktest", uuid.NewString())
	storage := testAppStorageFactory(t, storageFactory, testAppQName)
	TechnologyCompatibilityKit_Storage(t, storage, storageFactory.Time())
	storageFactory.StopGoroutines()
}

// need to test e.g. istoragecache
func TechnologyCompatibilityKit_Storage(t *testing.T, storage IAppStorage, iTime coreutils.ITime) {
	t.Run("TestAppStorage_GetPutRead", func(t *testing.T) { testAppStorage_GetPutRead(t, storage) })
	t.Run("TestAppStorage_PutBatch", func(t *testing.T) { testAppStorage_PutBatch(t, storage) })
	t.Run("TestAppStorage_GetBatch", func(t *testing.T) { testAppStorage_GetBatch(t, storage) })
	t.Run("TestAppStorage_InsertIfNotExists", func(t *testing.T) { testAppStorage_InsertIfNotExists(t, storage, iTime) })
	t.Run("TestAppStorage_CompareAndSwap", func(t *testing.T) { testAppStorage_CompareAndSwap(t, storage, iTime) })
	t.Run("TestAppStorage_CompareAndDelete", func(t *testing.T) { testAppStorage_CompareAndDelete(t, storage, iTime) })
	t.Run("TestAppStorage_TTLGet", func(t *testing.T) { testAppStorage_TTLGet(t, storage, iTime) })
	t.Run("TestAppStorage_TTLRead", func(t *testing.T) { testAppStorage_TTLRead(t, storage, iTime) })
}

func testAppStorageFactory(t *testing.T, sf IAppStorageFactory, testAppQName appdef.AppQName) IAppStorage {
	require := require.New(t)

	san, err := NewSafeAppName(testAppQName, func(name string) (bool, error) { return true, nil })
	require.NoError(err)
	t.Run("ErrStorageNotFound", func(t *testing.T) {
		s, err := sf.AppStorage(san)
		require.ErrorIs(err, ErrStorageDoesNotExist)
		require.Nil(s)
	})

	t.Run("ErrStorageExistsAlready", func(t *testing.T) {
		err := sf.Init(san)
		require.NoError(err)
		err = sf.Init(san)
		require.ErrorIs(err, ErrStorageAlreadyExists)
	})

	storage, err := sf.AppStorage(san)
	require.NoError(err)
	return storage
}

// nolint
func testAppStorage_GetPutRead(t *testing.T, storage IAppStorage) {

	t.Run("Should read not existing", func(t *testing.T) {
		ctx := context.Background()
		err := storage.Read(ctx, []byte{1}, nil, nil, nil)
		require.NoError(t, err, err)
	})
	t.Run("Should get not existing", func(t *testing.T) {
		require := require.New(t)
		require.NoError(storage.Put([]byte("*"), []byte("Month"), []byte("Dale - 24h")))

		data := make([]byte, 0)

		// not exists partition
		ok, err := storage.Get([]byte("0"), []byte("Month"), &data)

		require.False(ok)
		require.NoError(err)

		// not exists clustering columns

		ok, err = storage.Get([]byte("*"), []byte("Year"), &data)

		require.False(ok)
		require.NoError(err)
	})
	t.Run("Read method should read partition", func(t *testing.T) {
		ctx := context.Background()
		require := require.New(t)
		viewRecords := make([]string, 0, 2)
		resultCcols := []string{}
		reader := func(ccols, viewRecord []byte) (err error) {
			viewRecords = append(viewRecords, string(viewRecord))
			resultCcols = append(resultCcols, string(ccols))
			return err
		}
		require.NoError(storage.Put([]byte("1"), []byte("Dale"), []byte("Dale - 24h")))
		require.NoError(storage.Put([]byte("1"), []byte("Chip"), []byte("Chip - 10h")))
		require.NoError(storage.Put([]byte("2"), []byte("John"), []byte("John - 24h")))

		err := storage.Read(ctx, []byte("1"), nil, nil, reader)
		require.NoError(err)

		require.Len(viewRecords, 2)
		require.Equal("Chip - 10h", viewRecords[0])
		require.Equal("Dale - 24h", viewRecords[1])
		require.Equal("Chip", resultCcols[0])
		require.Equal("Dale", resultCcols[1])
	})

	t.Run("Read method should read by clustering columns range", func(t *testing.T) {
		ctx := context.Background()
		require := require.New(t)
		require.NoError(storage.Put([]byte{0x0}, []byte{0x10, 0x11, 0x17}, []byte("100$")))
		require.NoError(storage.Put([]byte{0x0}, []byte{0x10, 0x12, 0x12}, []byte("200$")))
		require.NoError(storage.Put([]byte{0x0}, []byte{0x10, 0x11, 0x16}, []byte("300$")))
		require.NoError(storage.Put([]byte{0x0}, []byte{0x10, 0x10, 0x12}, []byte("400$")))
		require.NoError(storage.Put([]byte{0x0}, []byte{0x09, 0x07, 0x00}, []byte("500$")))

		t.Run("read closed range", func(t *testing.T) {
			viewRecords := make([]string, 0, 5)
			reader := func(ccols, viewRecord []byte) (err error) {
				viewRecords = append(viewRecords, string(viewRecord))
				return err
			}

			err := storage.Read(ctx, []byte{0x0}, []byte{0x10, 0x10, 0x00}, []byte{0x10, 0x11, 0xff}, reader)
			require.NoError(err)

			require.Len(viewRecords, 3)
			require.Equal("400$", viewRecords[0])
			require.Equal("300$", viewRecords[1])
			require.Equal("100$", viewRecords[2])
		})

		t.Run("read left-open range", func(t *testing.T) {
			viewRecords := make([]string, 0, 5)
			reader := func(ccols, viewRecord []byte) (err error) {
				viewRecords = append(viewRecords, string(viewRecord))
				return err
			}

			err := storage.Read(ctx, []byte{0x0}, nil, []byte{0x10, 0x11, 0xff}, reader)
			require.NoError(err)

			require.Len(viewRecords, 4)
			require.Equal("500$", viewRecords[0])
			require.Equal("400$", viewRecords[1])
			require.Equal("300$", viewRecords[2])
			require.Equal("100$", viewRecords[3])
		})

		t.Run("read right-open range", func(t *testing.T) {
			viewRecords := make([]string, 0, 5)
			reader := func(ccols, viewRecord []byte) (err error) {
				viewRecords = append(viewRecords, string(viewRecord))
				return err
			}

			err := storage.Read(ctx, []byte{0x0}, []byte{0x10, 0x11, 0x00}, nil, reader)
			require.NoError(err)

			require.Len(viewRecords, 3)
			require.Equal("300$", viewRecords[0])
			require.Equal("100$", viewRecords[1])
			require.Equal("200$", viewRecords[2])
		})

		t.Run("read open range", func(t *testing.T) {
			viewRecords := make([]string, 0, 5)
			reader := func(ccols, viewRecord []byte) (err error) {
				viewRecords = append(viewRecords, string(viewRecord))
				return err
			}

			err := storage.Read(ctx, []byte{0x0}, nil, nil, reader)
			require.NoError(err)

			require.Len(viewRecords, 5)
			require.Equal("500$", viewRecords[0])
			require.Equal("400$", viewRecords[1])
			require.Equal("300$", viewRecords[2])
			require.Equal("100$", viewRecords[3])
			require.Equal("200$", viewRecords[4])
		})

		t.Run("read absurd range", func(t *testing.T) {
			times := 0
			reader := func(ccols, viewRecord []byte) (err error) {
				times++
				return nil
			}
			_ = reader(nil, nil) // to cover code

			err := storage.Read(ctx, []byte{0x0}, []byte{0x11, 0x11, 0x00}, []byte{0x10, 0x11, 0x00}, reader)
			require.NoError(err)
			require.Equal(1, times)
		})

		t.Run("read not exists pKey", func(t *testing.T) {
			times := 0
			reader := func(ccols, viewRecord []byte) (err error) {
				times++
				return nil
			}
			_ = reader(nil, nil) // to cover code

			err := storage.Read(ctx, []byte{0x1}, nil, nil, reader)
			require.NoError(err)
			require.Equal(1, times)
		})
	})
	t.Run("Read method should handle callback error", func(t *testing.T) {
		ctx := context.Background()
		require := require.New(t)
		errCb := errors.New("callback error")
		var times int
		reader := func(ccols, viewRecord []byte) (err error) {
			times++
			return errCb
		}
		require.NoError(storage.Put([]byte{1}, []byte{1}, []byte("100$")))
		require.NoError(storage.Put([]byte{1}, []byte{2}, []byte("200$")))

		times = 0
		err := storage.Read(ctx, []byte{1}, nil, nil, reader)

		require.ErrorIs(err, errCb)
		require.Equal(1, times)
	})

	t.Run("Should get exists", func(t *testing.T) {
		require := require.New(t)
		data := make([]byte, 0, 100)
		require.NoError(storage.Put([]byte{123}, []byte{}, []byte("100$")))

		ok, err := storage.Get([]byte{123}, []byte{}, &data)

		require.True(ok)
		require.NoError(err)
		require.Equal([]byte("100$"), data)
	})

	t.Run("Should get be able to reuse data slice", func(t *testing.T) {
		require := require.New(t)
		data := make([]byte, 0, 100)
		require.NoError(storage.Put([]byte{1}, []byte{}, []byte("150$")))
		require.NoError(storage.Put([]byte{2}, []byte{}, []byte("20$")))
		require.NoError(storage.Put([]byte{3}, []byte{}, []byte("4000$")))

		ok, err := storage.Get([]byte{1}, []byte{}, &data)

		require.True(ok)
		require.NoError(err)
		require.Equal([]byte("150$"), data)

		ok, err = storage.Get([]byte{2}, []byte{}, &data)

		require.True(ok)
		require.NoError(err)
		require.Equal([]byte("20$"), data)

		ok, err = storage.Get([]byte{3}, []byte{}, &data)

		require.True(ok)
		require.NoError(err)
		require.Equal([]byte("4000$"), data)
	})

	t.Run("Read should handle ctx error", func(t *testing.T) {
		require := require.New(t)
		var times int
		var cancel context.CancelFunc
		reader := func(ccols, viewRecord []byte) (err error) {
			times++
			cancel()
			return err
		}
		require.NoError(storage.Put([]byte("1-1"), []byte("20"), []byte("150$")))
		require.NoError(storage.Put([]byte("1-1"), []byte("21"), []byte("20$")))
		require.NoError(storage.Put([]byte("1-1"), []byte("22"), []byte("4000$")))

		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		times = 0
		err := storage.Read(ctx, []byte("1-1"), nil, nil, reader)
		require.NoError(err)
		require.Equal(1, times)

		cancel() // make stupid linter happy
	})

	t.Run("Should be able to pass nil clustering columns to Get / Put", func(t *testing.T) {
		ctx := context.Background()
		require := require.New(t)

		viewRecords := make(map[string][]byte)
		reader := func(clustCols, viewRecord []byte) (err error) { // This err is used on IAppStorage.Read invocation
			viewRecords[string(viewRecord)] = append(clustCols[:0:0], clustCols...)
			return err
		}

		require.NoError(storage.Put([]byte{0xaa}, []byte("33"), []byte("Pepsi")))
		require.NoError(storage.Put([]byte{0xaa}, nil, []byte("Cola")))

		err := storage.Read(ctx, []byte{0xaa}, nil, nil, reader)
		require.NoError(err)
		require.Len(viewRecords, 2)
		require.Equal([]byte("33"), viewRecords["Pepsi"])
		k, ok := viewRecords["Cola"]
		require.True(ok)
		require.Equal(0, len(k))

		var data []byte
		ok, err = storage.Get([]byte{0xaa}, nil, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal([]byte("Cola"), data)

		t.Run("zero-length clust columns must be same as nil", func(t *testing.T) {
			require.NoError(storage.Put([]byte{0xaa}, []byte{}, []byte("Baikal")))

			viewRecords = make(map[string][]byte) // clear

			require.NoError(storage.Read(ctx, []byte{0xaa}, []byte{}, []byte{}, reader))

			require.Len(viewRecords, 2)
			require.Equal([]byte("33"), viewRecords["Pepsi"])
			k, ok := viewRecords["Baikal"]
			require.True(ok)
			require.Equal(0, len(k))

			// Read as []byte{}
			{
				var data []byte
				ok, err := storage.Get([]byte{0xaa}, []byte{}, &data)
				require.True(ok)
				require.NoError(err)
				require.Equal([]byte("Baikal"), data)
			}
			// Read as nil
			{
				var data []byte
				ok, err := storage.Get([]byte{0xaa}, nil, &data)
				require.True(ok)
				require.NoError(err)
				require.Equal([]byte("Baikal"), data)
			}
		})
	})
}

// nolint
func testAppStorage_PutBatch(t *testing.T, storage IAppStorage) {
	require := require.New(t)
	type record struct {
		ccols []byte
		value []byte
	}
	items := []BatchItem{
		{
			PKey:  []byte("US"),
			CCols: []byte("Beverages"),
			Value: []byte("Beer"),
		},
		{
			PKey:  []byte("UK"),
			CCols: []byte("Main dishes"),
			Value: []byte("Steak"),
		},
		{
			PKey:  []byte("UK"),
			CCols: []byte("Side dishes"),
			Value: []byte("Salad"),
		},
	}

	require.NoError(storage.PutBatch(items))

	rr := make([]record, 0, len(items))
	require.NoError(storage.Read(context.Background(), items[0].PKey, []byte{}, []byte{}, func(ccols []byte, viewRecord []byte) (err error) {
		rr = append(rr, record{
			ccols: ccols,
			value: viewRecord,
		})
		return err
	}))
	require.NoError(storage.Read(context.Background(), items[1].PKey, []byte{}, []byte{}, func(ccols []byte, viewRecord []byte) (err error) {
		rr = append(rr, record{
			ccols: ccols,
			value: viewRecord,
		})
		return err
	}))

	require.Len(rr, len(items))
	require.Equal(items[0].CCols, rr[0].ccols)
	require.Equal(items[0].Value, rr[0].value)
	require.Equal(items[1].CCols, rr[1].ccols)
	require.Equal(items[1].Value, rr[1].value)
	require.Equal(items[2].CCols, rr[2].ccols)
	require.Equal(items[2].Value, rr[2].value)
}

// nolint:revive
func testAppStorage_GetBatch(t *testing.T, storage IAppStorage) {
	t.Run("Should get batch of existing records", func(t *testing.T) {
		require := require.New(t)
		type record struct {
			store bool
			ccols []byte
			value []byte
		}
		rr := []record{
			{
				ccols: []byte("notStored1"),
			},
			{
				store: true,
				ccols: []byte("Beverages"),
				value: []byte("Tomato juice"),
			},
			{
				store: true,
				ccols: []byte("Side dishes"),
				value: []byte("Backed corn"),
			},
			{
				store: true,
				ccols: []byte("Alcohol"),
				value: []byte("Vodka"),
			},
			{
				ccols: []byte("notStored2"),
			},
			{
				store: true,
				ccols: []byte("Main dishes"),
				value: []byte("Steak"),
			},
			{
				ccols: []byte("notStored3"),
			},
		}
		items := make([]GetBatchItem, len(rr))

		pKey := []byte("NL")

		for i, r := range rr {
			if r.store {
				require.NoError(storage.Put(pKey, r.ccols, r.value))
			}
			data := make([]byte, 0, 100)
			items[i] = GetBatchItem{
				CCols: r.ccols,
				Ok:    !r.store,
				Data:  &data,
			}
		}

		require.NoError(storage.GetBatch(pKey, items))

		require.Equal(rr[0].ccols, items[0].CCols)
		require.False(items[0].Ok)
		require.Empty(*items[0].Data)
		require.Equal(rr[1].ccols, items[1].CCols)
		require.True(items[1].Ok)
		require.Equal(rr[1].value, *items[1].Data)
		require.Equal(rr[2].ccols, items[2].CCols)
		require.True(items[2].Ok)
		require.Equal(rr[2].value, *items[2].Data)
		require.Equal(rr[3].ccols, items[3].CCols)
		require.True(items[3].Ok)
		require.Equal(rr[3].value, *items[3].Data)
		require.Equal(rr[4].ccols, items[4].CCols)
		require.False(items[4].Ok)
		require.Empty(*items[4].Data)
		require.Equal(rr[5].ccols, items[5].CCols)
		require.True(items[5].Ok)
		require.Equal(rr[5].value, *items[5].Data)
		require.Equal(rr[6].ccols, items[6].CCols)
		require.False(items[6].Ok)
		require.Empty(*items[6].Data)
	})
	t.Run("Should reuse data slice of each batch item", func(t *testing.T) {
		require := require.New(t)
		nlPKey := []byte("NL")
		ukPKey := []byte("UK")
		batch := []BatchItem{
			{
				PKey:  nlPKey,
				CCols: []byte("Beverages"),
				Value: []byte("Tomato juice"),
			},
			{
				PKey:  nlPKey,
				CCols: []byte("Main dishes"),
				Value: []byte("Steak"),
			},
			{
				PKey:  ukPKey,
				CCols: []byte("Side dishes"),
				Value: []byte("Backed corn"),
			},
			{
				PKey:  ukPKey,
				CCols: []byte("Alcohol"),
				Value: []byte("Vodka"),
			},
		}
		require.NoError(storage.PutBatch(batch))

		items := make([]GetBatchItem, 2)

		// Read NL batch
		for i := range items {
			items[i].CCols = batch[i].CCols
			data := make([]byte, 0, 100)
			items[i].Data = &data
		}

		require.NoError(storage.GetBatch(nlPKey, items))

		require.Equal(batch[0].CCols, items[0].CCols)
		require.True(items[0].Ok)
		require.Equal(batch[0].Value, *items[0].Data)
		require.Equal(batch[1].CCols, items[1].CCols)
		require.True(items[1].Ok)
		require.Equal(batch[1].Value, *items[1].Data)

		// Read UK batch
		for i := range items {
			items[i].CCols = batch[i+2].CCols
		}

		require.NoError(storage.GetBatch(ukPKey, items))

		require.Equal(batch[2].CCols, items[0].CCols)
		require.True(items[0].Ok)
		require.Equal(batch[2].Value, *items[0].Data)
		require.Equal(batch[3].CCols, items[1].CCols)
		require.True(items[1].Ok)
		require.Equal(batch[3].Value, *items[1].Data)
	})
	t.Run("Should set data for each batch item with equal ccols", func(t *testing.T) {
		require := require.New(t)

		countryPKey := []byte("RU")
		hotDrinksCCols := []byte("Hot drinks")
		coldDrinksCCols := []byte("Cold drinks")
		lemonTea := []byte("Lemon Tee")
		kombucha := []byte("Kombucha")

		batch := []BatchItem{
			{
				PKey:  countryPKey,
				CCols: hotDrinksCCols,
				Value: lemonTea,
			},
			{
				PKey:  countryPKey,
				CCols: coldDrinksCCols,
				Value: kombucha,
			},
		}
		require.NoError(storage.PutBatch(batch))

		newData := func() *[]byte {
			bb := make([]byte, 0, 100)
			return &bb
		}

		items := []GetBatchItem{
			{
				CCols: hotDrinksCCols,
				Data:  newData(),
			},
			{
				CCols: coldDrinksCCols,
				Data:  newData(),
			},
			{
				CCols: coldDrinksCCols,
				Data:  newData(),
			},
		}

		require.NoError(storage.GetBatch(countryPKey, items))

		require.Equal(hotDrinksCCols, items[0].CCols)
		require.True(items[0].Ok)
		require.Equal(lemonTea, *items[0].Data)
		require.Equal(coldDrinksCCols, items[1].CCols)
		require.True(items[1].Ok)
		require.Equal(kombucha, *items[1].Data)
		require.Equal(coldDrinksCCols, items[2].CCols)
		require.True(items[2].Ok)
		require.Equal(kombucha, *items[2].Data)
	})
	t.Run("Should return Ok=false if pKey does not exist", func(t *testing.T) {
		require := require.New(t)

		items := make([]GetBatchItem, 2)
		items[0].Ok = true
		items[0].CCols = []byte{}
		items[0].Data = &[]byte{}
		items[1].Ok = true
		items[1].CCols = []byte{}
		items[1].Data = &[]byte{}

		nePKey := []byte("This partition does not exist")

		require.NoError(storage.GetBatch(nePKey, items))
		require.False(items[0].Ok)
		require.False(items[1].Ok)

	})

}

//nolint:revive,goconst
func testAppStorage_InsertIfNotExists(t *testing.T, storage IAppStorage, iTime coreutils.ITime) {
	t.Run("Should insert if not exists", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Vehicles")
		ccols := []byte("Cars")
		value := []byte("Toyota")

		ok, err := storage.InsertIfNotExists(pKey, ccols, value, 1)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(2 * time.Second)

		ok, err = storage.InsertIfNotExists(pKey, ccols, value, 1)
		require.NoError(err)
		require.True(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal(value, data)
	})

	t.Run("Should not insert if exists", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Drinks")
		ccols := []byte("Non-alcohol")
		value := []byte("Tea")

		ok, err := storage.InsertIfNotExists(pKey, ccols, value, 1)
		require.NoError(err)
		require.True(ok)

		ok, err = storage.InsertIfNotExists(pKey, ccols, value, 1)
		require.NoError(err)
		require.False(ok)

		differentValue := bytes.Clone(value)
		differentValue = append(differentValue, []byte{42}...)
		ok, err = storage.InsertIfNotExists(pKey, ccols, differentValue, 1)
		require.NoError(err)
		require.False(ok)

		data := make([]byte, 0)
		ok, err = storage.Get(pKey, ccols, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal(value, data)
	})

	t.Run("ttl is zero", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Vehicles2")
		ccols := []byte("Cars2")
		value := []byte("Toyota2")

		ok, err := storage.InsertIfNotExists(pKey, ccols, value, 0)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(2 * time.Second)

		ok, err = storage.InsertIfNotExists(pKey, ccols, value, 0)
		require.NoError(err)
		require.False(ok)
	})
}

//nolint:revive,goconst
func testAppStorage_CompareAndSwap(t *testing.T, storage IAppStorage, iTime coreutils.ITime) {
	t.Run("Should swap if exists", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Games")
		ccols := []byte("Action")
		oldValue := []byte("Doom")

		ok, err := storage.InsertIfNotExists(pKey, ccols, oldValue, 2)
		require.NoError(err)
		require.True(ok)

		newValue := []byte("Call of Duty")
		ok, err = storage.CompareAndSwap(pKey, ccols, oldValue, newValue, 2)
		require.NoError(err)
		require.True(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal(newValue, data)
	})

	t.Run("Should not swap because of expiration", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Food")
		ccols := []byte("Salads")
		oldValue := []byte("Caesar")

		ok, err := storage.InsertIfNotExists(pKey, ccols, oldValue, 2)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(3 * time.Second)

		newValue := []byte("Olivier")
		ok, err = storage.CompareAndSwap(pKey, ccols, oldValue, newValue, 2)
		require.NoError(err)
		require.False(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.False(ok)
	})

	t.Run("Should not swap because of inequality new value and old one", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Movies")
		ccols := []byte("The Hobbit")
		oldValue := []byte("An unexpected journey")

		ok, err := storage.InsertIfNotExists(pKey, ccols, oldValue, 5)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(3 * time.Second)

		newValue := []byte("The desolation of Smaug")
		anotherOneValue := []byte("The battle of the five armies")
		ok, err = storage.CompareAndSwap(pKey, ccols, newValue, anotherOneValue, 2)
		require.NoError(err)
		require.False(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal(oldValue, data)
	})

	t.Run("ttl is zero", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Movies2")
		ccols := []byte("The Hobbit2")
		oldValue := []byte("An unexpected journey2")

		ok, err := storage.InsertIfNotExists(pKey, ccols, oldValue, 0)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(3 * time.Second)

		newValue := []byte("The desolation of Smaug")
		ok, err = storage.CompareAndSwap(pKey, ccols, oldValue, newValue, 2)
		require.NoError(err)
		require.True(ok)

		data := make([]byte, 0)
		ok, err = storage.Get(pKey, ccols, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal(newValue, data)
	})
}

//nolint:revive,goconst
func testAppStorage_CompareAndDelete(t *testing.T, storage IAppStorage, iTime coreutils.ITime) {
	t.Run("Should delete if exists", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Comics")
		ccols := []byte("Marvel")
		value := []byte("The Avengers")

		ok, err := storage.InsertIfNotExists(pKey, ccols, value, 2)
		require.NoError(err)
		require.True(ok)

		ok, err = storage.CompareAndDelete(pKey, ccols, value)
		require.NoError(err)
		require.True(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.False(ok)
	})

	t.Run("Should not delete because of key is expired", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Characters")
		ccols := []byte("Dwarves")
		oldValue := []byte("Thorin")

		ok, err := storage.InsertIfNotExists(pKey, ccols, oldValue, 1)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(2 * time.Second)

		ok, err = storage.CompareAndDelete(pKey, ccols, oldValue)
		require.NoError(err)
		require.False(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.False(ok)
	})

	t.Run("Should not delete because of inequality values", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Weapons")
		ccols := []byte("Guns")
		oldValue := []byte("M1911")

		ok, err := storage.InsertIfNotExists(pKey, ccols, oldValue, 2)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(1 * time.Second)

		newValue := []byte("Glock")
		ok, err = storage.CompareAndDelete(pKey, ccols, newValue)
		require.NoError(err)
		require.False(ok)

		ok, err = storage.InsertIfNotExists(pKey, ccols, oldValue, 2)
		require.NoError(err)
		require.False(ok)
	})
}

//nolint:revive,goconst
func testAppStorage_TTLGet(t *testing.T, storage IAppStorage, iTime coreutils.ITime) {
	t.Run("Should get ttl record if exists", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Books")
		ccols := []byte("Novels")
		value := []byte("Gone with the wind")

		ok, err := storage.InsertIfNotExists(pKey, ccols, value, 2)
		require.NoError(err)
		require.True(ok)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.Equal(value, data)
		require.True(ok)
	})

	t.Run("Should not get ttl record if it is expired", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Hogwarts")
		ccols := []byte("Griffindor")
		value := []byte("Harry Potter")

		ok, err := storage.InsertIfNotExists(pKey, ccols, value, 1)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(2 * time.Second)

		data := make([]byte, 0)
		ok, err = storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.False(ok)
	})

	t.Run("Should get regular record if exists", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Laptops")
		ccols := []byte("Apple")
		value := []byte("MacBook Pro")

		err := storage.Put(pKey, ccols, value)
		require.NoError(err)

		data := make([]byte, 0)
		ok, err := storage.TTLGet(pKey, ccols, &data)
		require.NoError(err)
		require.True(ok)
		require.Equal(value, data)
	})
}

//nolint:revive,goconst
func testAppStorage_TTLRead(t *testing.T, storage IAppStorage, iTime coreutils.ITime) {
	t.Run("Should read ttl records", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Key1")

		value1 := []byte{byte(1)}
		ok, err := storage.InsertIfNotExists(pKey, []byte("Col1"), value1, 1)
		require.NoError(err)
		require.True(ok)

		value2 := []byte{byte(2)}
		ok, err = storage.InsertIfNotExists(pKey, []byte("Col2"), value2, 3)
		require.NoError(err)
		require.True(ok)

		value3 := []byte{byte(3)}
		ok, err = storage.InsertIfNotExists(pKey, []byte("Col3"), value3, 4)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(1100 * time.Millisecond)

		subjects := make([][]byte, 0)
		data := make([]byte, 0)
		err = storage.TTLRead(context.Background(), pKey, nil, nil, func(ccols []byte, viewRecord []byte) (err error) {
			subjects = append(subjects, viewRecord)
			data = append(data[:0], viewRecord...)

			return nil
		})
		require.NoError(err)
		require.Len(subjects, 2)
	})

	t.Run("Should read regular records as well", func(t *testing.T) {
		require := require.New(t)
		pKey := []byte("Key2")

		value1 := []byte{byte(1)}
		err := storage.Put(pKey, []byte("Col1"), value1)
		require.NoError(err)

		value2 := []byte{byte(2)}
		err = storage.Put(pKey, []byte("Col2"), value2)
		require.NoError(err)

		value3 := []byte{byte(3)}
		ok, err := storage.InsertIfNotExists(pKey, []byte("Col3"), value3, 3)
		require.NoError(err)
		require.True(ok)

		iTime.Sleep(1 * time.Second)

		subjects := make([][]byte, 0)
		data := make([]byte, 0)
		err = storage.TTLRead(context.Background(), pKey, nil, nil, func(ccols []byte, viewRecord []byte) (err error) {
			subjects = append(subjects, viewRecord)
			data = append(data[:0], viewRecord...)

			return nil
		})
		require.NoError(err)
		require.Len(subjects, 3)
	})
}

// storageImplPkgPath returns package path of storage implementation
// it is used to skip tests for unsupported storage types
func storageImplPkgPath(storage IAppStorage) string {
	t := reflect.TypeOf(storage)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.PkgPath()
}
