# Implementation plan: Add read limiter arg to blobprocessor IRequestHandler blob reads

## Construction

### Read API surface

- [x] update: [pkg/processors/blobber/interface.go](../../../../../pkg/processors/blobber/interface.go)
  - update: add `iblobstorage.RLimiterType` arg to `HandleRead`, `HandleRead_V2`, and `HandleReadTemp_V2`

### Read request plumbing

- [x] update: [pkg/processors/blobber/impl_requesthandler.go](../../../../../pkg/processors/blobber/impl_requesthandler.go)
  - update: accept the read limiter in read handlers and pass it into the read message
- [x] update: [pkg/processors/blobber/types.go](../../../../../pkg/processors/blobber/types.go)
  - add: `rLimiter iblobstorage.RLimiterType` to the read message payload
- [x] update: [pkg/processors/blobber/impl_read.go](../../../../../pkg/processors/blobber/impl_read.go)
  - update: use the request-provided read limiter when calling `blobStorage.ReadBLOB`

### Caller updates

- [x] update: [pkg/router/impl_blob.go](../../../../../pkg/router/impl_blob.go)
  - update: pass `iblobstoragestg.RLimiter_Null` to API v1 blob reads
- [x] update: [pkg/router/impl_apiv2.go](../../../../../pkg/router/impl_apiv2.go)
  - update: pass `iblobstoragestg.RLimiter_Null` to API v2 blob reads

### Limited read handling in storage

- [x] update: [pkg/iblobstorage/errors.go](../../../pkg/iblobstorage/errors.go)
  - add: `iblobstorage.ErrReadLimitReached` to mark intentional read stop requests
- [x] update: [pkg/iblobstoragestg/impl.go](../../../pkg/iblobstoragestg/impl.go)
  - update: treat `ErrReadLimitReached` as a successful limited read
  - update: skip corruption mismatch reporting for intentionally limited reads
- [x] update: [pkg/iblobstoragestg/impl_test.go](../../../pkg/iblobstoragestg/impl_test.go)
  - add: extend `TestReadBLOBStopLimiter` to cover stopping after the first chunk, stopping immediately with `stateCallback`, propagating non-limit errors, and stopping after the first bucket

### SQL BLOB read behavior

- [x] update: [pkg/sys/sqlquery/impl_blobfuncs.go](../../../pkg/sys/sqlquery/impl_blobfuncs.go)
  - update: use `iblobstorage.RLimiterType` for bounded `blobtext(...)` reads instead of the previous limited-writer-only behavior
  - update: stop `blobinfo(...)`-only reads immediately instead of draining content into `io.Discard`
  - update: keep `startFrom` handling in `uint64` and validate it through unsigned parsing for `blobtext(...)`

### SQL BLOB read tests

- [x] update: [pkg/sys/sqlquery/impl_blobfuncs_test.go](../../../pkg/sys/sqlquery/impl_blobfuncs_test.go)
  - add: verify metadata-only reads stop before body content is written
- [x] update: [pkg/sys/it/impl_sqlquery_test.go](../../../pkg/sys/it/impl_sqlquery_test.go)
  - add: verify `blobtext(...)` applies `startFrom` together with the max returned bytes limit
  - update: verify invalid `startFrom` values return the current parse error

### Tests

- [x] verify: [pkg/sys/it/impl_blob_test.go](../../../../../pkg/sys/it/impl_blob_test.go)
  - verify: existing coverage for persistent, temporary, and API v2 blob reads passes after threading `iblobstoragestg.RLimiter_Null` through the read handlers
