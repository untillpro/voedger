# How: VSQL BLOB reading via blobinfo() and blobtext()

## Approach

- Extend the SELECT parser in `pkg/sys/sqlquery/impl.go` to recognize `*sqlparser.FuncExpr` alongside regular selected fields
- Parse only `blobinfo` and `blobtext`
  - recover the real field name through app definition metadata
  - parse the optional `startFrom` only for `blobtext`
  - store `startFrom` as `uint64`
- Enforce blob-function constraints in `pkg/sys/sqlquery/impl.go`
  - reject `WHERE` when blob functions are present
  - require a concrete record ID unless the source type is a singleton
- Emit `Content-Length` from blobber `initResponse` so sqlquery can build `blobinfo()` from headers only
- Reuse the same `Content-Length` header in federation through `blobSizeFromHeader`, so the BLOB size source stays consistent across readers
- Thread `blobprocessor.IRequestHandlerPtr` and `bus.IRequestSenderPtr` through `sqlquery.Provide` → `sysprovide.ProvideStateless` → VVM wiring
- Group post-wire interface pointers in `btstrp.PostWireInterfacePtrs` so sqlquery receives the blob handler and request sender through one bootstrap-owned struct whose placeholders are filled during bootstrap
- Group requested blob functions by field name and execute one `HandleRead_V2` call per field
  - `blobinfo(field)` and `blobtext(field)` for the same field share one blobber read
  - `ownerRecord` is the source document QName
  - `fieldName` is the BLOB field argument
  - `ownerID` is the query record ID
- Capture `X-BLOB-Name`, `Content-Type`, and `Content-Length` in `okResponseIniter` and build `blobinfo()` as `{name, mimetype, size}`
- Pass `iblobstorage.RLimiterType` through sqlquery → blobprocessor → storage
  - for `blobinfo()` only, use a limiter that returns `iblobstorage.ErrReadLimitReached` immediately so the read stops before the first chunk body write
  - for `blobtext()`, use `blobTextCapture` as both the writer and the source of the limiter callback
- In `blobTextCapture`
  - track the requested window as `[startFrom, startFrom + 10000)`
  - stop future chunk reads once the end of the window is reached
  - crop bytes inside the accepted chunk before appending to the buffer
- In `pkg/iblobstoragestg/impl.go`, treat `iblobstorage.ErrReadLimitReached` as an intentional limited read and skip corruption mismatch checks for such reads
- Build `blobtext()` from the captured bytes
  - return plain text for `text/*` and `application/json`
  - return base64 for other MIME types
- Merge blob-function results into the same row JSON as regular selected fields through dedicated helpers in `impl_blobfuncs.go`

References:

- [pkg/sys/sqlquery/impl.go](../../../../../pkg/sys/sqlquery/impl.go)
- [pkg/sys/sqlquery/impl_blobfuncs.go](../../../../../pkg/sys/sqlquery/impl_blobfuncs.go)
- [pkg/sys/sqlquery/provide.go](../../../../../pkg/sys/sqlquery/provide.go)
- [pkg/sys/sysprovide/provide.go](../../../../../pkg/sys/sysprovide/provide.go)
- [pkg/processors/blobber/interface.go](../../../../../pkg/processors/blobber/interface.go)
- [pkg/processors/blobber/impl_read.go](../../../../../pkg/processors/blobber/impl_read.go)
- [pkg/iblobstorage/errors.go](../../../../../pkg/iblobstorage/errors.go)
- [pkg/iblobstoragestg/impl.go](../../../../../pkg/iblobstoragestg/impl.go)
- [pkg/btstrp/types.go](../../../../../pkg/btstrp/types.go)
- [pkg/btstrp/impl.go](../../../../../pkg/btstrp/impl.go)
- [pkg/vvm/provide.go](../../../../../pkg/vvm/provide.go)
- [pkg/coreutils/federation/impl.go](../../../../../pkg/coreutils/federation/impl.go)
- [pkg/coreutils/federation/utils.go](../../../../../pkg/coreutils/federation/utils.go)
