/*
 * Copyright (c) 2022-present Sigma-Soft, Ltd.
 * @author: Dmitry Molchanovsky
 * @author: Maxim Geraskin (refactoring)
 */

package bbolt

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"

	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/utils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istorage"
)

type appStorageFactory struct {
	bboltParams ParamsType
	iTime       coreutils.ITime
	ctx         context.Context
	cancel      context.CancelFunc
	wg          *sync.WaitGroup
}

func (p *appStorageFactory) AppStorage(appName istorage.SafeAppName) (s istorage.IAppStorage, err error) {
	dbName := filepath.Join(p.bboltParams.DBDir, appName.String()+".db")
	exists, err := coreutils.Exists(dbName)
	if err != nil {
		// notest
		return nil, err
	}
	if !exists {
		return nil, istorage.ErrStorageDoesNotExist
	}
	db, err := bolt.Open(dbName, coreutils.FileMode_rw_rw_rw_, bolt.DefaultOptions)
	if err != nil {
		// notest
		return nil, err
	}

	if err := initDB(db); err != nil {
		return nil, err
	}

	impl := &appStorageType{db: db, iTime: p.iTime}
	// start background cleaner
	p.wg.Add(1)
	go impl.backgroundCleaner(p.ctx, p.wg)

	return impl, nil
}

func (p *appStorageFactory) Init(appName istorage.SafeAppName) error {
	dbName := filepath.Join(p.bboltParams.DBDir, appName.String()+".db")
	exists, err := coreutils.Exists(dbName)
	if err != nil {
		// notest
		return err
	}
	if exists {
		return istorage.ErrStorageAlreadyExists
	}
	if err = os.MkdirAll(p.bboltParams.DBDir, coreutils.FileMode_rwxrwxrwx); err != nil {
		// notest
		return err
	}
	db, err := bolt.Open(dbName, coreutils.FileMode_rw_rw_rw_, bolt.DefaultOptions)
	if err != nil {
		// notest
		return err
	}

	if err := initDB(db); err != nil {
		return err
	}

	return db.Close()
}

func (p *appStorageFactory) Time() coreutils.ITime {
	return p.iTime
}

func (p *appStorageFactory) StopGoroutines() {
	p.cancel()
	p.wg.Wait()
}

// if the key is empty or equal to nil, then convert it to nullKey
func safeKey(value []byte) []byte {
	if len(value) == 0 {
		return nullKey
	}
	return value
}

// if the key is nullKey, then convert it to nil
func unSafeKey(value []byte) []byte {
	if len(value) == 0 || (len(value) == 1 && value[0] == 0) {
		return nil
	}
	return value
}

// implemetation for istorage.IAppStorage.
type appStorageType struct {
	db    *bolt.DB
	iTime coreutils.ITime
}

//nolint:revive
func (s *appStorageType) InsertIfNotExists(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error) {
	found := false

	err = s.db.View(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			return nil
		}

		v := bucket.Get(safeKey(cCols))
		if v == nil {
			return nil
		}

		d := coreutils.ReadWithExpiration(v)
		if d.IsExpired(s.iTime.Now()) {
			return nil
		}

		found = true

		return nil
	})
	if err != nil {
		return false, err
	}

	if found {
		return false, nil
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		return s.putValue(tx, pKey, cCols, value, ttlSeconds)
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

//nolint:revive
func (s *appStorageType) CompareAndSwap(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error) {
	found := false

	found, err = s.findValue(pKey, cCols, oldValue)
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		return s.putValue(tx, pKey, cCols, newValue, ttlSeconds)
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

//nolint:revive
func (s *appStorageType) CompareAndDelete(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error) {
	found, err := s.findValue(pKey, cCols, expectedValue)
	if err != nil {
		return false, err
	}

	if !found {
		return false, nil
	}

	err = s.db.Update(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}

		if err := bucket.Delete(cCols); err != nil {
			return err
		}

		if bucket.Stats().KeyN == 0 {
			if err := dataBucket.DeleteBucket(pKey); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

//nolint:revive
func (s *appStorageType) TTLGet(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			return nil
		}

		v := bucket.Get(cCols)
		if v == nil {
			return nil
		}

		d := coreutils.ReadWithExpiration(v)
		if d.IsExpired(s.iTime.Now()) {
			return nil
		}

		*data = (*data)[:0]
		*data = d.Data
		ok = true

		return nil
	})
	if err != nil {
		return false, err
	}

	return
}

//nolint:revive
func (s *appStorageType) TTLRead(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	return s.read(ctx, pKey, startCCols, finishCCols, cb, true)
}

// istorage.IAppStorage.Put(pKey []byte, cCols []byte, value []byte) (err error)
func (s *appStorageType) Put(pKey []byte, cCols []byte, value []byte) (err error) {
	return s.db.Update(func(tx *bolt.Tx) error {
		dataBucket, err := tx.CreateBucketIfNotExists([]byte(dataBucketName))
		if err != nil {
			return err
		}
		d := coreutils.DataWithExpiration{Data: value}

		bucket, err := dataBucket.CreateBucketIfNotExists(pKey)
		if err != nil {
			return err
		}

		return bucket.Put(safeKey(cCols), d.ToBytes())
	})
}

// istorage.IAppStorage.PutBatch(items []BatchItem) (err error)
func (s *appStorageType) PutBatch(items []istorage.BatchItem) (err error) {
	return s.db.Update(func(tx *bolt.Tx) error {
		dataBucket, err := tx.CreateBucketIfNotExists([]byte(dataBucketName))
		if err != nil {
			return err
		}

		for i := 0; i < len(items); i++ {
			bucket, err := dataBucket.CreateBucketIfNotExists(items[i].PKey)
			if err != nil {
				return err
			}

			d := coreutils.DataWithExpiration{Data: items[i].Value}
			if err := bucket.Put(items[i].CCols, d.ToBytes()); err != nil {
				return err
			}
		}

		return nil
	})
}

// istorage.IAppStorage.Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error)
func (s *appStorageType) Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	*data = (*data)[0:0]

	err = s.db.View(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			return nil
		}

		v := bucket.Get(safeKey(cCols))
		if v == nil {
			return nil
		}

		*data = append(*data, v[utils.Uint64Size:]...)
		ok = true

		return nil
	})

	return ok, err
}

// istorage.IAppStorage.Read(ctx context.Context, pKey []byte, startCCols []byte, finishCCols []byte, cb ReadCallback) (err error)
func (s *appStorageType) Read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	return s.read(ctx, pKey, startCCols, finishCCols, cb, false)
}

// istorage.IAppStorage.GetBatch(pKey []byte, items []GetBatchItem) (err error)
func (s *appStorageType) GetBatch(pKey []byte, items []istorage.GetBatchItem) (err error) {
	return s.db.View(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			for i := 0; i < len(items); i++ {
				items[i].Ok = false
			}
			return nil
		}

		for i := 0; i < len(items); i++ {
			v := bucket.Get(safeKey(items[i].CCols))
			if v == nil {
				items[i].Ok = false
				*items[i].Data = append((*items[i].Data)[0:0], v...)
				continue
			}

			items[i].Ok = len(v[utils.Uint64Size:]) > 0
			*items[i].Data = append((*items[i].Data)[0:0], v[utils.Uint64Size:]...)
		}

		return nil
	})
}

func (s *appStorageType) read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback, checkTtl bool) (err error) {
	if (len(startCCols) > 0) && (len(finishCCols) > 0) && (bytes.Compare(startCCols, finishCCols) >= 0) {
		return nil // absurd range
	}

	return s.db.View(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		startCCols = unSafeKey(startCCols)
		finishCCols = unSafeKey(finishCCols)

		var (
			k []byte
			v []byte
		)

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			return nil
		}

		cr := bucket.Cursor()
		if startCCols == nil {
			k, v = cr.First()
		} else {
			k, v = cr.Seek(safeKey(startCCols))
		}

		var d coreutils.DataWithExpiration
		for (k != nil) && (finishCCols == nil || string(k) <= string(finishCCols)) {

			if ctx.Err() != nil {
				return nil
			}

			d = d.Update(v)
			if checkTtl && d.IsExpired(s.iTime.Now()) {
				k, v = cr.Next()
				continue
			}

			if cb != nil {
				if err != nil {
					return err
				}

				if err := cb(unSafeKey(k), d.Data); err != nil {
					return err
				}
			}
			k, v = cr.Next()
		}

		return nil
	})
}

func (s *appStorageType) findValue(pKey, cCols, value []byte) (found bool, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		dataBucket := tx.Bucket([]byte(dataBucketName))
		if dataBucket == nil {
			return ErrDataBucketNotFound
		}

		bucket := dataBucket.Bucket(pKey)
		if bucket == nil {
			return nil
		}

		v := bucket.Get(cCols)
		if v == nil {
			return nil
		}

		d := coreutils.ReadWithExpiration(v)
		if d.IsExpired(s.iTime.Now()) {
			return nil
		}

		if bytes.Equal(d.Data, value) {
			found = true
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return found, nil
}

func (s *appStorageType) putValue(tx *bolt.Tx, pKey, cCols, value []byte, ttlSeconds int) error {
	dataBucket, err := tx.CreateBucketIfNotExists([]byte(dataBucketName))
	if err != nil {
		return err
	}

	bucket, err := dataBucket.CreateBucketIfNotExists(pKey)
	if err != nil {
		return err
	}

	d := coreutils.DataWithExpiration{Data: value}
	if ttlSeconds > 0 {
		d.ExpireAt = s.iTime.Now().Add(time.Duration(ttlSeconds) * time.Second).UnixMilli()
	}

	if err := bucket.Put(safeKey(cCols), d.ToBytes()); err != nil {
		return err
	}

	if ttlSeconds > 0 {
		ttlBucket := tx.Bucket([]byte(ttlBucketName))
		if ttlBucket == nil {
			return ErrTTLBucketNotFound
		}

		if err := ttlBucket.Put(makeTtlKey(pKey, cCols, d.ExpireAt), nil); err != nil {
			return err
		}
	}

	return nil
}

func initDB(db *bolt.DB) error {
	// initialize the database
	return db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(dataBucketName)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(ttlBucketName)); err != nil {
			return err
		}

		return nil
	})
}

func (s *appStorageType) removeKey(tx *bolt.Tx, ttlKey []byte) error {
	pKey, cCols, err := extractPKAndCCols(ttlKey[utils.Uint64Size:])
	if err != nil {
		return err
	}

	dataBucket := tx.Bucket([]byte(dataBucketName))
	if dataBucket == nil {
		return ErrDataBucketNotFound
	}

	bucket := dataBucket.Bucket(pKey)
	if bucket != nil {
		if err := bucket.Delete(cCols); err != nil {
			return err
		}
		// if the bucket is empty, then delete it
		if bucket.Stats().KeyN == 0 {
			if err := dataBucket.DeleteBucket(pKey); err != nil {
				return err
			}
		}
	}

	ttlBucket := tx.Bucket([]byte(ttlBucketName))
	if ttlBucket == nil {
		return ErrTTLBucketNotFound
	}

	return ttlBucket.Delete(ttlKey)
}

func (s *appStorageType) backgroundCleaner(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for ctx.Err() == nil {
		timerCh := s.iTime.NewTimerChan(cleanupInterval)
		select {
		case <-ctx.Done():
			return
		case <-timerCh:
			err := s.db.Update(func(tx *bolt.Tx) error {
				ttlBucket := tx.Bucket([]byte(ttlBucketName))
				if ttlBucket == nil {
					return ErrTTLBucketNotFound
				}

				cr := ttlBucket.Cursor()
				k, _ := cr.First()
				for k != nil && ctx.Err() == nil {
					// extract expireAt from the key and check if it is expired
					expireAt := time.UnixMilli(int64(binary.BigEndian.Uint64(k[:utils.Uint64Size]))) //nolint: gosec
					if expireAt.After(s.iTime.Now()) {
						break
					}

					if err := s.removeKey(tx, k); err != nil {
						return err
					}

					k, _ = cr.Next()
				}

				return nil
			})
			if err != nil {
				logger.Error("bbolt storage: failed to cleanup expired records: " + err.Error())
			}
		}
	}
}

// extractPKAndCCols extracts both the pKey and the cCols from the ttl key.
// It assumes the key is in the format: len(pKey) + pKey + cCols
// where len(pKey) is an 8-byte unsigned integer (big-endian).
func extractPKAndCCols(ttlKey []byte) ([]byte, []byte, error) {
	if len(ttlKey) < utils.Uint64Size {
		return nil, nil, fmt.Errorf("invalid key: too short")
	}
	pKeyLength := binary.BigEndian.Uint64(ttlKey[:utils.Uint64Size])

	cColsOffset := utils.Uint64Size + pKeyLength
	if uint64(len(ttlKey)) < cColsOffset {
		return nil, nil, fmt.Errorf("invalid key: missing pKey data")
	}

	pKey := ttlKey[utils.Uint64Size : utils.Uint64Size+pKeyLength]
	cCols := ttlKey[cColsOffset:]

	return pKey, cCols, nil
}

// makeTtlKey creates a key for TTL bucket from the primary key, clustering columns and expireAt
func makeTtlKey(pKey, cCols []byte, expireAt int64) []byte {
	// key = 8 bytes for expireAt + 8 bytes for length of pKey + pKey + cCols
	totalLength := 2*utils.Uint64Size + len(pKey) + len(cCols)
	ttlKey := make([]byte, totalLength)
	binary.BigEndian.AppendUint64(ttlKey, uint64(expireAt))  // nolint G115
	binary.BigEndian.AppendUint64(ttlKey, uint64(len(pKey))) // nolint G115
	ttlKey = append(ttlKey, pKey...)
	ttlKey = append(ttlKey, cCols...)

	return ttlKey
}
