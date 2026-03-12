/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package iblobstoragestg

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/voedger/voedger/pkg/coreutils/utils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
)

type bStorageType struct {
	blobStorage BlobAppStoragePtr
	time        timeu.ITime
}

type storageWriter func(pKey, cCols, val []byte, duration iblobstorage.DurationType, oldValue []byte) error

// key does not cotain the bucket number
func (b *bStorageType) writeBLOB(ctx context.Context, blobKey []byte, descr iblobstorage.DescrType, reader io.Reader,
	limiter iblobstorage.WLimiterType, duration iblobstorage.DurationType, inserter, updater storageWriter) (uploadedSize uint64, err error) {
	var (
		bytesRead    uint64
		chunkNumber  uint64
		bucketNumber uint64 = 1
	)
	state := iblobstorage.BLOBState{
		Descr:     descr,
		StartedAt: istructs.UnixMilli(b.time.Now().UnixMilli()),
		Status:    iblobstorage.BLOBStatus_InProcess,
		Duration:  duration,
	}

	pKeyState, cColState := getStateKeys(blobKey)

	initialStateBytes, err := b.writeState(state, pKeyState, cColState, inserter, duration, nil)
	if err != nil {
		// notest
		return 0, err
	}

	chunkBuf := make([]byte, 0, chunkSize)

	pKeyWithBucket := newKeyWithBucketNumber(blobKey, bucketNumber)

	cCol := make([]byte, utils.Uint64Size)

	for ctx.Err() == nil && err == nil {
		var currentChunkSize int
		currentChunkSize, err = reader.Read(chunkBuf[:cap(chunkBuf)])
		if currentChunkSize > 0 {
			chunkBuf = chunkBuf[:currentChunkSize]
			bytesRead += uint64(len(chunkBuf))
			if err = limiter(uint64(len(chunkBuf))); err != nil {
				break
			}
			if bytesRead > chunkSize*bucketSize*bucketNumber {
				bucketNumber++
				pKeyWithBucket = mutateBucketNumber(pKeyWithBucket, bucketNumber)
			}
			cCol = mutateChunkNumber(cCol, chunkNumber)
			if err = inserter(pKeyWithBucket, cCol, chunkBuf, duration, nil); err != nil {
				// notest
				break
			}
			chunkNumber++
		}
	}

	if ctx.Err() != nil && err == nil {
		// err has priority over ctx.Err
		err = ctx.Err()
	}

	if errors.Is(err, io.EOF) {
		err = nil
	}

	state.FinishedAt = istructs.UnixMilli(b.time.Now().UnixMilli())
	state.Status = iblobstorage.BLOBStatus_Completed
	state.Size = bytesRead

	if err != nil {
		state.Error = err.Error()
		state.Status = iblobstorage.BLOBStatus_Unknown
	}

	if _, errState := b.writeState(state, pKeyState, cColState, updater, duration, initialStateBytes); errState != nil {
		// notest
		if err == nil {
			// err as priority over errStatus
			return 0, errState
		}
		logger.Error("failed to write blob state: " + errState.Error())
	}
	return state.Size, err
}

func mutateChunkNumber(key []byte, chunkNumber uint64) (mutadedKey []byte) {
	binary.LittleEndian.PutUint64(key, chunkNumber)
	return key
}

func mutateBucketNumber(key []byte, bucketNumber uint64) (mutatedKey []byte) {
	binary.LittleEndian.PutUint64(key[len(key)-utils.Uint64Size:], bucketNumber)
	return key
}

func getStateKeys(blobKey []byte) (pKeyState, cColSt []byte) {
	pKeyState = newKeyWithBucketNumber(blobKey, 0)
	return pKeyState, cColState
}

func (b *bStorageType) WriteBLOB(ctx context.Context, key iblobstorage.PersistentBLOBKeyType, descr iblobstorage.DescrType, reader io.Reader, limiter iblobstorage.WLimiterType) (uploadedSize uint64, err error) {
	inserterAndUpdater := func(pKey, cCols, val []byte, _ iblobstorage.DurationType, _ []byte) error {
		return (*(b.blobStorage)).Put(pKey, cCols, val)
	}
	return b.writeBLOB(ctx, key.Bytes(), descr, reader, limiter, 0, inserterAndUpdater, inserterAndUpdater)
}

func (b *bStorageType) WriteTempBLOB(ctx context.Context, key iblobstorage.TempBLOBKeyType, descr iblobstorage.DescrType, reader io.Reader, limiter iblobstorage.WLimiterType, duration iblobstorage.DurationType) (uploadedSize uint64, err error) {
	inserter := func(pKey, cCols, val []byte, duration iblobstorage.DurationType, _ []byte) error {
		ok, err := (*(b.blobStorage)).InsertIfNotExists(pKey, cCols, val, duration.Seconds())
		if err != nil {
			// notest
			return err
		}
		if !ok {
			// notest
			return fmt.Errorf("InsertIfNotExists false. Looks like a part of a blob is not expired yet.\npKey: 0x%x\ncCol: 0x%x", pKey, cCols)
		}
		return nil
	}
	updater := func(pKey, cCols, val []byte, duration iblobstorage.DurationType, oldValue []byte) error {
		ok, err := (*(b.blobStorage)).CompareAndSwap(pKey, cCols, oldValue, val, duration.Seconds())
		if err != nil {
			// notest
			return err
		}
		if !ok {
			// notest
			return fmt.Errorf("CompareAndSwap false.\npKey: 0x%x\ncCol: 0x%x", pKey, cCols)
		}
		return nil
	}
	return b.writeBLOB(ctx, key.Bytes(), descr, reader, limiter, duration, inserter, updater)
}

func (b *bStorageType) ReadBLOB(ctx context.Context, blobKey iblobstorage.IBLOBKey, stateCallback func(state iblobstorage.BLOBState) error, writer io.Writer, limiter iblobstorage.RLimiterType) (err error) {
	blobKeyBytes := blobKey.Bytes()

	// will not return on just !stateExists, need check if the blob is not corrupted
	stateExists := false
	state, err := b.QueryBLOBState(ctx, blobKey)
	if err == nil {
		stateExists = true
	} else if !errors.Is(err, iblobstorage.ErrBLOBNotFound) {
		// notest
		return err
	}

	if stateExists && stateCallback != nil {
		if err = stateCallback(state); err != nil {
			return err
		}
	}

	if len(state.Error) > 0 {
		return fmt.Errorf("%w: %s", iblobstorage.ErrBLOBCorrupted, state.Error)
	}

	bucketNumber := uint64(1)
	pKeyWithBucket := newKeyWithBucketNumber(blobKeyBytes, bucketNumber)

	var bytesRead uint64
	limited := false
	for ctx.Err() == nil {
		bucketExists := false
		err = (*(b.blobStorage)).Read(ctx, pKeyWithBucket, nil, nil,
			func(ccols []byte, viewRecord []byte) (err error) {
				bucketExists = true
				if !stateExists {
					return fmt.Errorf("%w: blob data exists whereas state is missing", iblobstorage.ErrBLOBCorrupted)
				}
				if limiter != nil {
					err = limiter(uint64(len(viewRecord)))
					if err != nil {
						if errors.Is(err, iblobstorage.ErrReadLimitReached) {
							limited = true
						}
						return err
					}
				}
				if _, err = writer.Write(viewRecord); err == nil {
					bytesRead += uint64(len(viewRecord))
				}
				return err
			})
		if errors.Is(err, iblobstorage.ErrReadLimitReached) {
			err = nil
			break
		}
		if err != nil || !bucketExists {
			break
		}
		bucketNumber++
		pKeyWithBucket = mutateBucketNumber(pKeyWithBucket, bucketNumber)
	}

	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	if !stateExists && bytesRead == 0 {
		return iblobstorage.ErrBLOBNotFound
	}

	if !limited && bytesRead != state.Size {
		return fmt.Errorf("%w: %d bytes stored in the blob whereas %d bytes are read", iblobstorage.ErrBLOBCorrupted, state.Size, bytesRead)
	}

	return nil
}

func (b *bStorageType) QueryBLOBState(ctx context.Context, key iblobstorage.IBLOBKey) (state iblobstorage.BLOBState, err error) {
	blobKeyBytes := key.Bytes()
	pKeyState, cColState := getStateKeys(blobKeyBytes)
	state, stateExists, err := b.readState(pKeyState, cColState, key.IsPersistent())
	if err != nil {
		// notest
		return state, err
	}
	if !stateExists {
		return state, iblobstorage.ErrBLOBNotFound
	}
	return state, nil
}

func (b *bStorageType) readState(pKey, cCol []byte, isPersistent bool) (state iblobstorage.BLOBState, ok bool, err error) {
	var stateBytes []byte
	if isPersistent {
		ok, err = (*(b.blobStorage)).Get(pKey, cCol, &stateBytes)
	} else {
		ok, err = (*(b.blobStorage)).TTLGet(pKey, cCol, &stateBytes)
	}
	if err != nil || !ok {
		return state, ok, err
	}
	err = json.Unmarshal(stateBytes, &state)
	return state, true, err
}

func (b *bStorageType) writeState(state iblobstorage.BLOBState, pKey, cCol []byte, storageWriter storageWriter, duration iblobstorage.DurationType,
	oldValue []byte) (stateBytes []byte, err error) {
	value, err := json.Marshal(state)
	if err != nil {
		// notest
		return nil, err
	}
	return value, storageWriter(pKey, cCol, value, duration, oldValue)
}

func newKeyWithBucketNumber(blobKey []byte, bucket uint64) []byte {
	res := make([]byte, len(blobKey)+8)
	copy(res, blobKey)
	binary.LittleEndian.PutUint64(res[len(res)-8:], bucket)
	return res
}

func (q *implSizeLimiter) limit(wantToWriteBytes uint64) error {
	q.uploadedSize += wantToWriteBytes
	if q.uploadedSize > uint64(q.maxSize) {
		return fmt.Errorf("%w (max %d allowed)", iblobstorage.ErrBLOBSizeQuotaExceeded, q.maxSize)
	}
	return nil
}
