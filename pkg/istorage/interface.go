/*
* Copyright (c) 2021-present unTill Pro, Ltd.
* @author Maxim Geraskin
 */

package istorage

import (
	"context"
	"github.com/voedger/voedger/pkg/pipeline"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
)

// used by cached app storage and in bootstrap to create storages for sys/router and sys/bobber applications
type IAppStorageProvider interface {
	pipeline.IService
	// converts AppQname to string and calls internal IAppStorageFactory.AppStorage
	// storage for a brand new app is queried and failed to create the storage for it -> storage init error is persisted and returned until admin is handle with that incident
	// result is cached, the new isntance is created if there is no in the cache yet
	// could return ErrStorageNotFound, ErrStorageExistsAlready
	AppStorage(appName appdef.AppQName) (storage IAppStorage, err error)
}

// implemented by a certain driver
type IAppStorageFactory interface {
	// creates new instance on each call. Init() must be called for the provided appName
	// returns ErrStorageNotFound
	AppStorage(appName SafeAppName) (storage IAppStorage, err error)

	// initializes the new underlying storage (cassandra keyspace, bbolt file etc)
	// returns ErrStorageExistsAlready
	Init(appName SafeAppName) error

	// used by TCK in tests
	Time() coreutils.ITime

	// stops all goroutines started by the factory
	StopGoroutines()
}

type IAppStorage interface {
	// cCols - clustering columns
	// len(cCols) may be 0 (nil or empty array)
	// Clustering columns MUST be filled from left to right
	// Example: PRIMARY KEY(wsid, qname_id, id)
	//   Clusterting columns: qname_id, id
	//   qname_id bytes must be written first, then id bytes
	// @ConcurrentAccess
	Put(pKey []byte, cCols []byte, value []byte) (err error)

	PutBatch(items []BatchItem) (err error)

	// len(cCols) may be 0, in this case the record which was written with zero len(cCols) will be returned
	// ok == false means that viewrecord does not exist
	// @ConcurrentAccess
	Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error)

	// get and appends result to items[i].Data
	// items[i].Ok==false means record is not found
	// items[i].Ok & Data are undefined in case of error
	GetBatch(pKey []byte, items []GetBatchItem) (err error)

	// startCCols can be empty (nil or zero len), in this case reads from start of partition.
	// finishCCols can be empty (nil or zero len) too. In this case reads to the end of partition
	// @ConcurrentAccess
	Read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb ReadCallback) (err error)
	// ********* Working with TTL records ***********************

	// Implementors of this interface may choose to store TTL records in a separate keyspace
	// to optimize performance of non-TTL operations. This is an implementation detail and
	// not required. Both approaches (same or separate keyspace) must provide identical
	// behavior from the caller's perspective.

	// InsertIfNotExists performs a conditional insert operation similar to Cassandra's INSERT IF NOT EXISTS.
	// It atomically inserts a new record only if no record exists for the given primary key.
	//
	// Parameters:
	//   - pKey: Primary key bytes
	//   - cCols: Clustering columns bytes (if any)
	//   - value: Value bytes to be inserted
	//   - ttlSeconds: Time-to-live in seconds after which the record will be automatically deleted. Ignored if zero.
	//
	// Returns:
	//   - ok: true if insert was successful (record didn't exist), false if record already exists
	//   - err: error if operation failed
	InsertIfNotExists(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error)

	// CompareAndSwap performs an atomic compare-and-swap operation similar to Cassandra's UPDATE IF.
	// It updates a record's value only if its current value matches oldValue.
	//
	// Parameters:
	//   - pKey: Primary key bytes
	//   - cCols: Clustering columns bytes (if any)
	//   - oldValue: Expected current value bytes to compare against
	//   - newValue: New value bytes to swap in if comparison succeeds
	//   - ttlSeconds: Time-to-live in seconds to set/update on the record. Ignored if zero.
	//
	// Returns:
	//   - ok: true if swap was successful (record existed and matched oldValue), false otherwise
	//   - err: error if operation failed
	CompareAndSwap(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error)

	// CompareAndDelete performs an atomic compare-and-delete operation similar to Cassandra's DELETE IF.
	// It deletes a record only if it exists and optionally matches a specific value.
	//
	// Parameters:
	//   - pKey: Primary key bytes
	//   - cCols: Clustering columns bytes (if any)
	//   - expectedValue: Optional value bytes to compare against. If nil, only checks existence
	//
	// Returns:
	//   - ok: true if deletion was successful (record existed and matched value if provided), false otherwise
	//   - err: error if operation failed
	CompareAndDelete(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error)

	// TTLGet retrieves a record that might have been written with a TTL.
	// The TTL information is not returned, only the record's existence and value.
	// ok == false means that record does not exist or has expired
	TTLGet(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error)

	// TTLRead reads records that might have been written with a TTL.
	// The TTL information is not returned in the callback.
	// Only existing and non-expired records are returned through the callback.
	TTLRead(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb ReadCallback) (err error)
}

// ccols and viewRecord are temporary internal values, must NOT be changed
type ReadCallback func(ccols []byte, viewRecord []byte) (err error)

type BatchItem struct {
	PKey  []byte
	CCols []byte
	Value []byte
}

type GetBatchItem struct {
	CCols []byte
	Ok    bool
	Data  *[]byte
}
