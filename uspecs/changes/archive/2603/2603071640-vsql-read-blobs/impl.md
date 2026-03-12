# Implementation plan: VSQL BLOB reading via blobinfo() and blobtext()

## Functional design

- [x] create: [apps/vsql-blob-read.feature](../../../../specs/prod/apps/vsql-blob-read.feature)
  - add: Scenarios for `blobinfo()` returning JSON metadata (name, mimetype, size, status)
  - add: Scenarios for `blobtext()` returning blob content (base64 for binary, plain text otherwise, limited to 10000 bytes, optional startFrom)
  - add: Constraint scenarios: blob functions require docID or singleton, WHERE clause rejected

## Construction

### Blobber: expose size header

- [x] update: [pkg/coreutils/consts.go](../../../../pkg/coreutils/consts.go)
  - add: `BlobSize` constant for `X-BLOB-Size` header name
- [x] update: [pkg/goutils/httpu/consts.go](../../../../pkg/goutils/httpu/consts.go)
  - add: `ContentLength` constant for `Content-Length` header name
- [x] update: [pkg/processors/blobber/impl_read.go](../../../../pkg/processors/blobber/impl_read.go)
  - update: `initResponse` to emit `Content-Length` header from `bw.blobState.Size`
  - update: `catchReadError.DoSync` to call `errorResponder` with new `SysError` signature
- [x] update: [pkg/processors/blobber/interface.go](../../../../pkg/processors/blobber/interface.go)
  - update: `ErrorResponder` signature from `func(statusCode int, args ...interface{})` to `func(sysError coreutils.SysError)`
- [x] update: [pkg/processors/blobber/types.go](../../../../pkg/processors/blobber/types.go)
  - add: `IRequestHandlerPtr` type alias (`*IRequestHandler`)
- [x] update: [pkg/bus/types.go](../../../../pkg/bus/types.go)
  - add: `IRequestSenderPtr` type alias (`*IRequestSender`)

### Blobber: update ErrorResponder callers

- [x] update: [pkg/processors/blobber/impl_write.go](../../../../pkg/processors/blobber/impl_write.go)
  - update: `sendWriteResult.DoSync` to call `errorResponder(sysError)` with new signature
- [x] update: [pkg/router/impl_apiv2.go](../../../../pkg/router/impl_apiv2.go)
  - update: all blob request handlers to use new `ErrorResponder` signature (`func(sysErr coreutils.SysError)`)
- [x] update: [pkg/router/impl_blob.go](../../../../pkg/router/impl_blob.go)
  - update: `blobHTTPRequestHandler_Write` and `blobHTTPRequestHandler_Read` to use new `ErrorResponder` signature

### BLOBReader: expose blob size

- [x] update: [pkg/iblobstorage/types.go](../../../../pkg/iblobstorage/types.go)
  - add: `BLOBSize uint64` field to `BLOBReader` struct
- [x] update: [pkg/coreutils/federation/impl.go](../../../../pkg/coreutils/federation/impl.go)
  - refactor: extract shared `readBLOB` method used by both `ReadBLOB` and `ReadTempBLOB`
  - update: populate `BLOBSize` from `Content-Length` response header
- [x] update: [pkg/coreutils/federation/utils.go](../../../../pkg/coreutils/federation/utils.go)
  - add: `blobSizeFromHeader` helper to parse `Content-Length` header into `uint64`

### SqlQuery: parse blob functions and wire blobprocessor

- [x] update: [pkg/sys/sqlquery/provide.go](../../../../pkg/sys/sqlquery/provide.go)
  - update: `Provide` signature to accept `blobprocessor.IRequestHandlerPtr` and `bus.IRequestSenderPtr`; pass them to `provideExecQrySQLQuery`
- [x] update: [pkg/sys/sysprovide/provide.go](../../../../pkg/sys/sysprovide/provide.go)
  - update: `ProvideStateless` to accept `blobprocessor.IRequestHandlerPtr` and `bus.IRequestSenderPtr`, pass them to `sqlquery.Provide`
- [x] update: [pkg/sys/sqlquery/impl.go](../../../../pkg/sys/sqlquery/impl.go)
  - update: `provideExecQrySQLQuery` to accept `blobprocessor.IRequestHandlerPtr` and `bus.IRequestSenderPtr`
  - add: Parse `*sqlparser.FuncExpr` in SELECT walk to detect `blobinfo`/`blobtext` and extract field name + optional `startFrom`
  - add: Reject WHERE clause when blob functions are present
  - add: Validate blob functions require docID or singleton
  - add: Call `IRequestHandler.HandleRead_V2` with the post-wired `bus.IRequestSenderPtr` for requested blob functions; build `blobinfo` JSON / `blobtext` content from captured response headers and writer
  - refactor: extract record blob-function execution into dedicated helper to keep select flow simpler
- [x] create: [pkg/sys/sqlquery/impl_blobfuncs.go](../../../../pkg/sys/sqlquery/impl_blobfuncs.go)
  - add: `blobFuncDesc` struct and parsing logic (`parseBlobFuncExpr`)
  - add: `executeBlobFunctions`, `executeBlobRead` functions
  - add: group requested blob functions by field name so one blob read can serve both `blobinfo()` and `blobtext()` for the same field
  - add: `blobTextCapture` and `stopReadImmediately` so `blobtext()` can capture a bounded byte window and `blobinfo()` can stop before body writes
  - add: `mergeJSONWithBlobResults` for combining record data with blob results

### Storage limited reads

- [x] update: [pkg/iblobstorage/errors.go](../../../../pkg/iblobstorage/errors.go)
  - add: `iblobstorage.ErrReadLimitReached` to mark intentional limited reads
- [x] update: [pkg/iblobstoragestg/impl.go](../../../../pkg/iblobstoragestg/impl.go)
  - update: treat `ErrReadLimitReached` as a successful limited read and skip corruption mismatch reporting for intentional partial reads
- [x] update: [pkg/iblobstoragestg/impl_test.go](../../../../pkg/iblobstoragestg/impl_test.go)
  - add: `TestReadBLOBStopLimiter` coverage for first-chunk stop, immediate stop with `stateCallback`, non-limit error propagation, and first-bucket stop

### VVM wiring

- [x] update: [pkg/btstrp/types.go](../../../../pkg/btstrp/types.go)
  - add: `PostWireInterfacePtrs` to group post-wire blob/router storage, blob handler, and request sender pointers
- [x] update: [pkg/btstrp/impl.go](../../../../pkg/btstrp/impl.go)
  - update: `Bootstrap` signature to accept `PostWireInterfacePtrs` plus concrete `blobHandler` and `requestSender`; assign storage, handler, and sender values through the struct during bootstrap
- [x] update: [pkg/vvm/provide.go](../../../../pkg/vvm/provide.go)
  - update: `provideStatelessResources` and `provideBootstrapOperator` to accept `btstrp.PostWireInterfacePtrs`
  - add: `provideBlobHandlerPtr`, `provideIRequestSenderPtr`, and `providePostWireInterfacePtrs` factory functions for grouped bootstrap wiring
  - update: pass grouped post-wire pointers into `sysprovide.ProvideStateless` and `btstrp.Bootstrap`
- [x] Review
- [x] update: [pkg/vvm/wire_gen.go](../../../../pkg/vvm/wire_gen.go)
  - update: create post-wire interface pointers before `provideStatelessResources`, assemble `postWireInterfacePtrs`, and pass it into `provideStatelessResources` and `provideBootstrapOperator`

### Tests

- [x] update: [pkg/sys/it/impl_blob_test.go](../../../../pkg/sys/it/impl_blob_test.go)
  - add: `BLOBSize` assertions to existing persistent, temporary, APIv1v2, ODoc, and PseudoWSID blob tests
- [x] update: [pkg/sys/it/impl_bootstrap_test.go](../../../../pkg/sys/it/impl_bootstrap_test.go)
  - update: `Bootstrap` calls in all test scenarios to pass `btstrp.PostWireInterfacePtrs` plus concrete blob handler and request sender
  - add: `newPostWiredInterfacePtrs` helper and assertions for post-wire storage, handler, and sender values
- [x] update: [pkg/sys/it/impl_sqlquery_test.go](../../../../pkg/sys/it/impl_sqlquery_test.go)
  - add: Integration tests for `blobinfo()` on a doc with blob field
  - add: Integration tests for `blobtext()` with text and binary blobs, with and without `startFrom`
  - add: Integration test covering `blobinfo()` and `blobtext()` in the same request
  - add: Error tests: blob functions without docID on non-singleton, with WHERE clause, with non-existent field

## Quick start

Query blob metadata:

```sql
select blobinfo(Img1) from air.Restaurant.123.air.DocWithBLOBs.456
```

Query blob content (first 10000 bytes):

```sql
select blobtext(Img1) from air.Restaurant.123.air.DocWithBLOBs.456
```

Query blob content starting from byte offset:

```sql
select blobtext(Img1, 5000) from air.Restaurant.123.air.DocWithBLOBs.456
```
