# Decisions: VSQL BLOB reading via blobinfo() and blobtext()

## 1. BLOB access path and read control

Use `blobprocessor.IRequestHandler.HandleRead_V2(..., rLimiter)` as the only read path from sqlquery

Rationale: `change.md` explicitly requires `blobprocessor.IRequestHandler`. The actual implementation wires `blobprocessor.IRequestHandlerPtr` and `bus.IRequestSenderPtr` through `sqlquery.Provide` → `sysprovide.ProvideStateless` → VVM bootstrap wiring, then calls `HandleRead_V2` for each requested BLOB field. The current handler signature also accepts `iblobstorage.RLimiterType`, which lets sqlquery stop reads early without bypassing BLOB ID resolution or `q.sys.DownloadBLOBAuthnz`

Alternatives:

- `federation.IFederation.ReadBLOB` (confidence: low)
  - adds an HTTP round-trip and bypasses the in-process blobber path used by other reads
- direct storage reads from sqlquery (confidence: low)
  - would duplicate blobber lookup and authorization logic

## 2. blobinfo() metadata comes from headers and must not require body download

Use headers captured by `okResponseIniter` and stop metadata-only reads immediately with `iblobstorage.ErrReadLimitReached`

Rationale: the blobber pipeline already knows `name`, `mimetype`, and `size` before streaming body chunks. The final implementation emits `Content-Length` from `initResponse`, captures `X-BLOB-Name`, `Content-Type`, and `Content-Length` in sqlquery, then uses a limiter that stops before the first chunk write when only `blobinfo()` is requested. This keeps `blobinfo()` metadata-only in practice, not only in intent

Alternatives:

- read the whole BLOB into `io.Discard` and count bytes (confidence: low)
  - wastes I/O and defeats the metadata-only use case
- add a separate metadata-only blob API (confidence: medium)
  - possible, but larger than needed because the existing read pipeline already exposes the metadata

## 3. Bounded blobtext() reads use both a limiter and a capture writer

Use `iblobstorage.RLimiterType` to stop future chunk reads and `blobTextCapture` to crop the wanted byte window inside accepted chunks

Rationale: `blobtext(field, startFrom)` needs two properties at once:

- exact slicing inside the current chunk
- early stop once the requested range is fully covered

The final implementation keeps stream position in `blobTextCapture`, passes `writer.limit` into `HandleRead_V2`, and has storage treat `iblobstorage.ErrReadLimitReached` as an intentional stop rather than corruption. This replaces the older writer-only limiting approach while preserving exact `startFrom` behavior on chunked storage

Alternatives:

- writer-only limiting such as `limitedBlobWriter` (confidence: medium)
  - simpler, but still reads unnecessary chunks after the result is already complete
- limiter-only slicing (confidence: low)
  - cannot express partial-chunk `startFrom` and end-window cropping by itself

## 4. blobtext() offset parsing uses unsigned values

Parse `startFrom` as `uint64` and store it as `uint64` in blob-function descriptors and capture helpers

Rationale: the final implementation parses the second argument with `strconvu.ParseUint64` and carries it through `blobFuncDesc`, `fieldRequest`, `executeBlobRead`, and `newBlobTextCapture` as `uint64`. This matches the actual domain of byte offsets and naturally rejects negative values or oversized integers through unsigned parsing

Alternatives:

- parse as signed integer and reject negatives later (confidence: medium)
  - works, but models a byte offset with a wider value space than needed

## 5. SELECT clause handling stays inside sqlquery and merges blob results into row JSON

Parse `*sqlparser.FuncExpr` in the SELECT loop, group requests by field name, and merge blob results into the row JSON returned by sqlquery

Rationale: the existing SELECT parser in `impl.go` already walks `s.SelectExprs`. The final implementation collects `blobinfo` and `blobtext` descriptors there, validates them there, executes one BLOB read per field, then merges the produced values into the same JSON object as regular selected fields. This keeps the feature inside normal `select` execution instead of inventing a separate query shape

Alternatives:

- make blob functions a separate statement type in `dml` (confidence: low)
  - breaks the desired SQL shape and spreads blob-function semantics outside sqlquery

## 6. Blob-function constraints are enforced in sqlquery, not in dml

Reject blob-function queries with a `WHERE` clause and without a concrete record ID on non-singletons in `impl.go`

Rationale: `dml` parses routing-level query structure, but sqlquery owns the AST and knows whether blob functions are present. The final implementation enforces both constraints where the blob-function descriptors are already available, which keeps validation close to execution

Alternatives:

- reject in `dml` (confidence: low)
  - would require `dml` to understand sqlquery-only function semantics

## 7. Post-wire interfaces are grouped in PostWireInterfacePtrs

Group post-wire interface pointers in `btstrp.PostWireInterfacePtrs` and pass that struct through VVM wiring

Rationale: the final implementation creates placeholder pointer cells during wiring for blobber app storage, router app storage, blob handler, and request sender, then fills them during bootstrap. Grouping them in `PostWireInterfacePtrs` keeps that ownership in `btstrp`, reduces loose parameters across bootstrap and VVM provider functions, and gives sqlquery one bootstrap-owned container for the handler and sender that become available only after wiring

Alternatives:

- pass separate pointer parameters through bootstrap and VVM wiring (confidence: medium)
  - works, but spreads one concept across more signatures and test helpers
- introduce a VVM-local grouping type (confidence: low)
  - hides a bootstrap-owned concept outside the package that actually settles it
