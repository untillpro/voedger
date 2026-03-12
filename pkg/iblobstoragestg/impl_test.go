/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package iblobstoragestg

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"io"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/goutils/testingu"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istorage/mem"
	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
)

var (
	//go:embed testdata/logo.png
	blob    []byte
	maxSize iblobstorage.BLOBMaxSizeType = 19266
)

func TestBasicUsage(t *testing.T) {
	t.Run("persistent", func(t *testing.T) {
		key := iblobstorage.PersistentBLOBKeyType{
			ClusterAppID: 2,
			WSID:         2,
			BlobID:       2,
		}
		testBasicUsage(t, func() iblobstorage.IBLOBKey {
			return &key
		}, func(blobber iblobstorage.IBLOBStorage, desc iblobstorage.DescrType, reader *bytes.Reader, _ iblobstorage.DurationType) (uploadedSize uint64, err error) {
			ctx := context.Background()
			return blobber.WriteBLOB(ctx, key, desc, reader, NewWLimiter_Size(maxSize))
		}, 0)
	})

	t.Run("temporary", func(t *testing.T) {
		ssuid := iblobstorage.NewSUUID()
		key := iblobstorage.TempBLOBKeyType{
			ClusterAppID: 2,
			WSID:         2,
			SUUID:        ssuid,
		}
		blobStorage := testBasicUsage(t, func() iblobstorage.IBLOBKey {
			return &key
		}, func(blobber iblobstorage.IBLOBStorage, desc iblobstorage.DescrType, reader *bytes.Reader, duration iblobstorage.DurationType) (uploadedSize uint64, err error) {
			ctx := context.Background()
			return blobber.WriteTempBLOB(ctx, key, desc, reader, NewWLimiter_Size(maxSize), duration)
		}, iblobstorage.DurationType_1Day)

		// make the temp blob almost expired
		testingu.MockTime.Add(time.Duration(iblobstorage.DurationType_1Day.Seconds()-1) * time.Second)

		// blob still exists for now
		_, err := blobStorage.QueryBLOBState(context.Background(), &key)
		require.NoError(t, err)

		// cross the temp blob expiration instant
		testingu.MockTime.Add(time.Second)

		// blob disappeared
		_, err = blobStorage.QueryBLOBState(context.Background(), &key)
		require.ErrorIs(t, err, iblobstorage.ErrBLOBNotFound)
	})
}

func testBasicUsage(t *testing.T, keyGetter func() iblobstorage.IBLOBKey,
	blobWriter func(blobber iblobstorage.IBLOBStorage, desc iblobstorage.DescrType, reader *bytes.Reader, duration iblobstorage.DurationType) (uploadedSize uint64, err error),
	duration iblobstorage.DurationType) iblobstorage.IBLOBStorage {
	desc := iblobstorage.DescrType{
		Name:        "logo.png",
		ContentType: "image/png",
	}

	key := keyGetter()
	require := require.New(t)

	asf := mem.Provide(testingu.MockTime)
	asp := istorageimpl.Provide(asf)
	storage, err := asp.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)
	time := testingu.MockTime
	blobber := Provide(&storage, time)
	ctx := context.TODO()
	reader := provideTestData()

	t.Run("Read blob that is absent. We MUST get error iblobstorage.ErrBLOBNotFound.", func(t *testing.T) {
		var (
			buf   bytes.Buffer
			state iblobstorage.BLOBState
		)
		writer := bufio.NewWriter(&buf)
		err := blobber.ReadBLOB(ctx, key, func(blobState iblobstorage.BLOBState) (err error) {
			state = blobState
			require.Nil(state)
			return nil
		}, writer, RLimiter_Null)
		require.ErrorIs(err, iblobstorage.ErrBLOBNotFound)
	})

	t.Run("Write blob to storage, return must be without errors", func(t *testing.T) {
		size, err := blobWriter(blobber, desc, reader, duration)
		require.NoError(err)
		require.EqualValues(len(blob), size)
	})

	t.Run("Read blob status, return must be without errors", func(t *testing.T) {
		bs, err := blobber.QueryBLOBState(ctx, key)
		require.NoError(err)
		require.Equal(desc.Name, bs.Descr.Name)
		require.Equal(desc.ContentType, bs.Descr.ContentType)
		require.Equal(time.Now().UnixMilli(), int64(bs.StartedAt))
		require.Equal(time.Now().UnixMilli(), int64(bs.FinishedAt))
		require.EqualValues(len(blob), bs.Size)
		require.Equal(iblobstorage.BLOBStatus_Completed, bs.Status)
		require.Empty(bs.Error)
		require.Equal(duration, bs.Duration)
	})

	t.Run("Read blob that present in storage and compare with reference", func(t *testing.T) {
		var (
			buf   bytes.Buffer
			state iblobstorage.BLOBState
		)
		writer := bufio.NewWriter(&buf)

		// Reset reader and read anew
		reader.Reset(blob)
		v, err := readData(ctx, reader)
		require.NoError(err)

		// Read
		err = blobber.ReadBLOB(ctx, key, func(blobState iblobstorage.BLOBState) (err error) {
			state = blobState
			log.Println(state.Error)
			return nil
		}, writer, RLimiter_Null)

		require.NoError(err)
		err = writer.Flush()
		require.NoError(err)

		// Compare
		require.Equal(v, buf.Bytes())
	})

	return blobber
}

func TestFewBucketsBLOB(t *testing.T) {
	var (
		key = iblobstorage.PersistentBLOBKeyType{
			ClusterAppID: 2,
			WSID:         2,
			BlobID:       2,
		}
		desc = iblobstorage.DescrType{
			Name:        "test",
			ContentType: "image/png",
		}
	)
	require := require.New(t)

	asf := mem.Provide(testingu.MockTime)
	asp := istorageimpl.Provide(asf)
	storage, err := asp.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)
	blobber := Provide(&storage, timeu.NewITime())
	ctx := context.TODO()

	// size is more than chunkSize*bucketSize -> bucket++. Will check the case when the bucket number is increased
	bigBLOB := make([]byte, chunkSize*bucketSize*2+1)

	// fill the blob with the random bytes
	_, err = rand.Read(bigBLOB)
	require.NoError(err)

	// write the blob
	reader := bytes.NewReader(bigBLOB)
	size, err := blobber.WriteBLOB(ctx, key, desc, reader, NewWLimiter_Size(iblobstorage.BLOBMaxSizeType(len(bigBLOB))))
	require.NoError(err)
	require.EqualValues(len(bigBLOB), size)

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	// Reset reader and read anew
	reader.Reset(bigBLOB)

	// Read
	err = blobber.ReadBLOB(ctx, &key, func(blobState iblobstorage.BLOBState) (err error) { return nil }, writer, RLimiter_Null)
	require.NoError(err)
	err = writer.Flush()
	require.NoError(err)

	// Compare
	require.Equal(bigBLOB, buf.Bytes())
}

func TestReadBLOBStopLimiter(t *testing.T) {
	outerRequire := require.New(t)
	asf := mem.Provide(testingu.MockTime)
	asp := istorageimpl.Provide(asf)
	storage, err := asp.AppStorage(istructs.AppQName_test1_app1)
	outerRequire.NoError(err)
	blobber := Provide(&storage, timeu.NewITime())
	ctx := context.TODO()
	desc := iblobstorage.DescrType{Name: "test", ContentType: "text/plain"}

	writeBLOB := func(t *testing.T, blobID int64, data []byte) iblobstorage.PersistentBLOBKeyType {
		require := require.New(t)
		key := iblobstorage.PersistentBLOBKeyType{
			ClusterAppID: 2,
			WSID:         2,
			BlobID:       istructs.RecordID(blobID),
		}
		_, err := blobber.WriteBLOB(ctx, key, desc, bytes.NewReader(data), NewWLimiter_Size(iblobstorage.BLOBMaxSizeType(len(data))))
		require.NoError(err)
		return key
	}

	t.Run("stops after first chunk", func(t *testing.T) {
		require := require.New(t)
		bigBLOB := bytes.Repeat([]byte("a"), int(chunkSize+1))
		key := writeBLOB(t, 3, bigBLOB)
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		remaining := chunkSize

		err := blobber.ReadBLOB(ctx, &key, nil, writer, func(wantReadBytes uint64) error {
			if remaining == 0 {
				return iblobstorage.ErrReadLimitReached
			}
			if wantReadBytes >= remaining {
				remaining = 0
			} else {
				remaining -= wantReadBytes
			}
			return nil
		})

		require.NoError(err)
		require.NoError(writer.Flush())
		require.Equal(bigBLOB[:chunkSize], buf.Bytes())
	})

	t.Run("stops immediately and still calls state callback", func(t *testing.T) {
		require := require.New(t)
		bigBLOB := bytes.Repeat([]byte("b"), int(chunkSize+1))
		key := writeBLOB(t, 4, bigBLOB)
		var (
			buf           bytes.Buffer
			callbackCalls int
			stateSize     uint64
		)
		writer := bufio.NewWriter(&buf)

		err := blobber.ReadBLOB(ctx, &key, func(state iblobstorage.BLOBState) error {
			callbackCalls++
			stateSize = state.Size
			return nil
		}, writer, func(uint64) error {
			return iblobstorage.ErrReadLimitReached
		})

		require.NoError(err)
		require.NoError(writer.Flush())
		require.Equal(1, callbackCalls)
		require.Equal(uint64(len(bigBLOB)), stateSize)
		require.Empty(buf.Bytes())
	})

	t.Run("propagates non limit error", func(t *testing.T) {
		require := require.New(t)
		bigBLOB := bytes.Repeat([]byte("c"), int(chunkSize+1))
		key := writeBLOB(t, 5, bigBLOB)
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		expectedErr := errors.New("stop")

		err := blobber.ReadBLOB(ctx, &key, nil, writer, func(uint64) error {
			return expectedErr
		})

		require.ErrorIs(err, expectedErr)
		require.NoError(writer.Flush())
		require.Empty(buf.Bytes())
	})

	t.Run("stops after first bucket", func(t *testing.T) {
		require := require.New(t)
		firstBucketSize := chunkSize * bucketSize
		bigBLOB := bytes.Repeat([]byte("d"), int(firstBucketSize+1))
		key := writeBLOB(t, 6, bigBLOB)
		var buf bytes.Buffer
		writer := bufio.NewWriter(&buf)
		remaining := firstBucketSize

		err := blobber.ReadBLOB(ctx, &key, nil, writer, func(wantReadBytes uint64) error {
			if remaining == 0 {
				return iblobstorage.ErrReadLimitReached
			}
			if wantReadBytes >= remaining {
				remaining = 0
			} else {
				remaining -= wantReadBytes
			}
			return nil
		})

		require.NoError(err)
		require.NoError(writer.Flush())
		require.Equal(bigBLOB[:int(firstBucketSize)], buf.Bytes())
	})
}

func TestQuotaExceed(t *testing.T) {
	var (
		key = iblobstorage.PersistentBLOBKeyType{
			ClusterAppID: 2,
			WSID:         2,
			BlobID:       2,
		}
		desc = iblobstorage.DescrType{
			Name:        "logo.png",
			ContentType: "image/png",
		}
	)
	require := require.New(t)
	asf := mem.Provide(testingu.MockTime)
	asp := istorageimpl.Provide(asf)
	storage, err := asp.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)
	blobber := Provide(&storage, timeu.NewITime())
	reader := provideTestData()
	ctx := context.Background()
	// Quota (maxSize -1 = 19265) assigned to reader less then filesize logo.png (maxSize)
	// So, it must be error
	_, err = blobber.WriteBLOB(ctx, key, desc, reader, NewWLimiter_Size(maxSize-1))
	require.Error(err, "Reading a file larger than the quota assigned to the reader. It must be a error.")
}

func provideTestData() (reader *bytes.Reader) {
	return bytes.NewReader(blob)
}

func readData(ctx context.Context, reader io.Reader) (data []byte, err error) {
	var bytesRead int
	var entity []byte
	nBytes := int64(0)
	buf := make([]byte, 0, chunkSize)
	for err == nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		bytesRead, err = reader.Read(buf[:cap(buf)])
		if bytesRead > 0 {
			buf = buf[:bytesRead]
			nBytes += int64(len(buf))
			entity = append(entity, buf...)
		}
	}
	if err == io.EOF {
		err = nil
	}
	return entity, err
}
