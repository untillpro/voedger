# Technical design: logging

## Overview

The logging subsystem provides structured, context-aware logging with automatic attribute propagation through the request processing pipeline. It uses Go's standard `log/slog` package as the underlying engine while maintaining custom log levels and adding context-based attribute management.

## Concepts

### Testing

Tests that ensures that certain attributes and messages are logged in basic scenarios should be implemented

### Logging Attribute

A key-value pair that provides additional context to log entries. Attributes are stored in `context.Context` and propagate with the context through the request processing pipeline using a linked-list chain structure.

**Implementation details:**

- Attributes are stored in a linked-list chain (`logAttrs` struct) attached to context
- Later calls shadow earlier ones for the same key (newest-first lookup)
- O(1) attribute addition via `logger.WithContextAttrs()`
- Attributes are extracted and appended to log entries automatically by `*Ctx` logging functions

**Standard attributes:**

- **feat** (string): Feature name within the application
  - Constant: `logger.LogAttr_Feat`
  - Example: "magicmenu"
  - Set by: Application-specific handlers
  - Purpose: Track feature-level activity

- **reqid** (string): Unique request identifier
  - Constant: `logger.LogAttr_ReqID`
  - Purpose: Trace single request through all processing stages
  - Set by: Router using global atomic counter
  - Format: "{Server start time (MMDDHHmm)}-{atomicCounter}"
  - Example: "03141504-42"

- **vapp** (string): Voedger application qualified name
  - Purpose: Identify which application is processing the request
  - Set by: Processing initiator
    - Router at request entry point: `sys/registry`, `untill/fiscalcloud`
    - Voedger on bootstrapping: `sys/voedger`

- **wsid** (int): Workspace ID
  - Constant: `logger.LogAttr_WSID`
  - Example: 1001
  - Set by: Router from validated request data, actualizers when processing events
  - Purpose: Filter logs by workspace for multi-tenant debugging

- **extension** (string): Extension or function being executed
  - Constant: `logger.LogAttr_Extension`
  - Example: `c.sys.UploadBLOBHelper`, `q.sys.Collection`, `sys._Docs`, `sys._CP`
    - `sys._Docs`: API v2, working with documents
  - Purpose: Identify specific command/query/function in logs
  - Set by: Processing initiator
    - Router: based on request resource/QName
    - Actualizer: based on event QName

- **stage** (string): Processing stage within the component that emits the log entry
  - Constant: `logger.LogAttr_Stage`
  - Purpose: Categorize log entries by processing phase, enabling filtering and inter-stage latency measurement across components
  - Set by: caller, provided as the `stage` parameter of `logger.*Ctx()` funcs

## General scenarios

- App enriches request context with logging attributes (vapp, reqid, wsid, extension) using `logger.WithContextAttrs()`
- App calls context-aware logging functions with `context`, `stage`, and message `args` as parameters
  - Context attributes (vapp, reqid, wsid, extension, etc.) are automatically extracted and added to the log entry
  - `stage` value is appended to the log as `stage` attribute
  - Message args are formatted via `fmt.Sprint()` and used as the log message
- Affected parts of both APIv1 and APIv2 must have the same logging
- If there is already some logging in the affected source code files that is not described in this document then this logging must be dropped

---

## Per-component logging

### Server core events

### HTTP

HTTP root context is derived from VVM context:

- `vapp="sys/voedger"`
- `extension` = server name: `sys._HTTPServer`, `sys._AdminHTTPServer`, `sys._HTTPSServer`, or `sys._ACMEServer`
  Used for logging server start/stop operations and for all incoming HTTP requests:

- Router params validation failure: level `Error`, stage `routing.validation`, msg `<error message>`
- Start accepting connections success: level `Info`, stage `endpoint.listen.start`, msg `<addr>:<port>`
- Start accepting connections failure: level `Error`, stage `endpoint.listen.error`, msg `<error message>`
- Server stops accepting connections: level `Info`, stage `endpoint.shutdown`, msg (empty)
- Error on http server shutdown: level `Error`, stage `endpoint.shutdown.error`, msg `<error message>`
- Server exits unexpectedly: level `Error`, stage `endpoint.unexpectedstop`, msg `Serve() error: <err>` or `ServeTLS() error: <err>`

#### Application deployment

`btstrp.Bootstrap()` is called. Uses `vapp="sys/voedger"`, `extension="sys._Bootstrap"` attribs.

- Bootstrap starts: level `Info`, stage `bootstrap`, msg `started`
- Cluster app workspace initialized: level `Info`, stage `bootstrap`, msg `cluster app workspace initialized`
- For each built-in app: level `Info`, stage `bootstrap.appdeploy.builtin`, msg `<appQName>`
- For each sidecar app: level `Info`, stage `bootstrap.appdeploy.sidecar`, msg `<appQName>`
- For each built-in app partition: level `Info`, stage `bootstrap.apppartdeploy.builtin`, msg `<appQName>/<partID>`
- For each sidecar app partition: level `Info`, stage `bootstrap.apppartdeploy.sidecar`, msg `<appQName>/<partID>`
- Bootstrap completes: level `Info`, stage `bootstrap`, msg `completed`
- On app deployment failure: panics with `failed to deploy app <appName>: <error>` (no logging)

#### Leadership acquisition

Uses `vapp="sys/voedger"`, `extension="sys._Leadership"`, `key` attribs.

- When elections are finalized: level `Warning`, stage `leadership.acquire.finalized`, msg `elections cleaned up; cannot acquire leadership`
- On each attempt when another node holds leadership: level `Info`, stage `leadership.acquire.other`, msg `leadership already acquired by someone else`
- On storage error: level `Error`, stage `leadership.acquire.error`, msg `InsertIfNotExist failed: <err>`
- On acquire success: level `Info`, stage `leadership.acquire.success`, msg `success`

#### Leadership maintenance

Uses `vapp="sys/voedger"`, `extension="sys._Leadership"`, `key` attribs.

- First 10 renewal ticks: level `Verbose`, stage `leadership.maintain.10`, msg `renewing leadership`
- Every 200 ticks after initial 10: level `Verbose`, stage `leadership.maintain.200`, msg `still leader for <duration>`
- On transient storage error (retried every second within the interval): level `Error`, stage `leadership.maintain.stgerror`, msg `compareAndSwap error: <err>`
- On leadership stolen: level `Error`, stage `leadership.maintain.stolen`, msg `compareAndSwap !ok => release`
- On all retries exhausted within interval: level `Error`, stage `leadership.maintain.release`, msg `retry deadline reached, releasing. Last error: <err>`
- On error after `processKillThreshold` (TTL/4), before `os.Exit(1)`: level `Error`, stage `leadership.maintain.terminating`, msg `the process is still alive after the time alloted for graceful shutdown -> terminating...`

#### Leadership release

Uses `vapp="sys/voedger"`, `extension="sys._Leadership"`, `key` attribs.

- When `ReleaseLeadership` is called but this node is not the leader: level `Warning`, stage `leadership.release.notleader`, msg `we're not the leader`
- On successful release: level `Info`, stage `leadership.released`, msg (empty)

---

### Router

**Request handling:**

- Validates request data (app, workspace, resource)
- Generates unique request ID: `fmt.Sprintf("%s-%d", globalServerStartTime, reqID.Add(1))`
  - `globalServerStartTime` format: "MMDDHHmm" (e.g., "03061504" for March 6, 15:04)
  - `reqID` is atomic counter incremented per request
- Creates request context with attributes:
  - `vapp`: Application QName from validated data
  - `reqid`: Generated request ID
  - `wsid`: Workspace ID from validated data
  - `extension`: Resource name (API v1) or QName/API path (API v2)
  - `origin`: HTTP Origin header value
- Request received: level `Verbose`, stage `routing.accepted`, msg (empty)
- First response from bus (immediately after `SendRequest` returns): level `Verbose`, stage `routing.latency1`, msg `<latency_ms>`
- Error sending request to VVM: level `Error`, stage `routing.send2vvm.error`, msg `<error message>`
- Error sending response to client: level `Error`, stage `routing.response.error`, msg `<error message>`

---

### Command Processor

**Request processing:**

The context with attributes is received from Router

- Logs the event details right after successful write to PLog: calls `processors.LogEventAndCUDs()` (see [LogEventAndCUDs](#logeventandcuds)) with stage `cp.plog_saved`; per-CUD callback always returns `shouldLog=true`; `msgAdds` is `,oldfields={...}` for CUDs that arrived with the HTTP request, empty for CUDs created by the command; `eventMessageAdds` is empty
- Right before sending the response to the bus:
  - Command handling error: level `Error`, stage `cp.error`, msg `<error message>`, `body`=`<compacted request body>`
  - Command executed successfully: level `Verbose`, stage `cp.success`, msg `<command result>`
- Additional log on errors:
  - if error happens on any of:
    - sync actualizers run
    - apply records
    - put to WLog or PLog
  - then logs that partition restart is scheduled
    - stage `cp.partition_recovery`
    - level `Warning`
    - `vapp` attrib is replaced with `sys/voedger`
    - `extension` attrib is replaced with `sys._Recovery`
    - msg `partition will be restarted due of an error on <syncActualizers, applyRecords, etc>: <error message>`

**Partition recovery:**

- `vapp` attrib is replaced with `sys/voedger`
- `extension` attrib: `sys._Recovery`
- `partid` attrib: partition ID
- Partition recovery start: level `Info`, stage `cp.partition_recovery.start`, msg (empty)
- Partition recovery complete: level `Info`, stage `cp.partition_recovery.complete`, msg `completed`, nextPLogOffset and workspaces JSON
- ReadPLog failure: level `Error`, stage `cp.partition_recovery.readplog.error`, msg `<error message>`
- Last event re-apply: `processors.LogEventAndCUDs()` called with stage `cp.partition_recovery.reapply` to log which event is being re-applied (with `woffset`, `poffset`, `evqname` attribs); the enriched context is stored in `cmdWorkpiece.logCtx` and used by sync projectors during re-apply
- `LogEventAndCUDs` failure during re-apply: level `Error`, stage `cp.partition_recovery.logeventandcuds.error`, msg `<error message>`
- StoreOp failure (re-apply last event): level `Error`, stage `cp.partition_recovery.storeop.error`, msg `<error message>`

---

### Query Processor

**Request processing:**

The context with attributes is received from Router

- query execution error: level `Error`, stage `qp.error`, msg `<error message>`
- Logs success right after `execQuery` returns: level `Verbose`, stage `qp.success`, msg (empty)

### Sync Projectors

Launched by command processor between `ApplyRecords` and `PutWLog` stages

**Context propagation:**

The enriched context returned by `processors.LogEventAndCUDs()` (with attribs `woffset`, `poffset`, `evqname`) is stored in the `cmdWorkpiece.logCtx` field and used for all sync projector logging:

- `logEventAndCUDs` (called after PLog write) saves the enriched context to `cmdWorkpiece.logCtx`
- During partition recovery, `LogEventAndCUDs()` is called for the re-applied event and its result is also stored in `cmdWorkpiece.logCtx`
- Command processor sync projector handler reads `cmd.logCtx` for `sp.success` and `sp.error` logs
- Each projector branch receives `cmd.logCtx` (via `syncActualizerWorkpiece.LogCtxForSyncProjector()`) for its per-projector logs

**Command processor logs** (using `cmd.logCtx`):

- After all sync projectors succeed: level `Verbose`, stage `sp.success`, msg (empty)
- Sync projector error: level `Error`, stage `sp.error`, msg `<error message>`

**Each triggered sync projector** (using `LogCtxForSyncProjector()`):

The event is already logged by the command processor (`cp.plog_saved`), so there is no separate `logEventAndCUDs` call per projector. The projector uses `LogCtxForSyncProjector()` directly to obtain the enriched context and extends it with `extension`=`<projector QName>`:

- Right before `IAppParts.Invoke()`: level `Verbose`, stage `sp.triggeredby`, msg `<triggered by qname>`, `extension`=`<projector QName>`
- After successful `Invoke()`: level `Verbose`, stage `sp.success`, `extension`=`<projector QName>`, msg (empty)
- On `Invoke()` failure: level `Error`, stage `sp.error`, `extension`=`<projector QName>`, msg `<error message>`

### Async Projectors

Attributes:

- `vapp` and `extension` attribs are set on app partition deployment on start new actualizers stage. `Extension` is `ap.<projector QName>`
  **Event processing:**

Stage is `ap`

- Determines if the projector triggered by the current event via `ProjectorEvent()`
- Adds `wsid` to log context in `DoAsync()` when processing event
- Calls `processors.LogEventAndCUDs("ap")` (see [LogEventAndCUDs](#logeventandcuds)):
  - `perCUDCallback`: returns `shouldLog=true` for all CUDs when triggered by a function or ODoc/ORecord; for other triggers only CUDs matching the trigger QName; `msgAdds` is empty
  - `eventMessageAdds`: `triggeredby=<QName>`
- Constructs event context - merge the context got from `processors.LogEventAndCUDs` and the ctx with `vapp` and `extension` attribs
- Stores the event context in the pipeline workpiece to use it on error logging
- Logs success: level `Verbose`, stage `ap.success`, msg (empty)

**Error handling:**

Done in `retryercfg.OnError()` handler

- Logs the error: level `Error`, stage `ap.error`, msg `<error message>`
  - the error is pipeline.IPipelineErr -> the context is taken from the workpiece (the one created in `DoAsync` with wsid)
  - otherwise -> the context is `readCtx.ctx`

### Blob Processor

Note: BLOB ID is a string — either a numeric record ID or a SUUID, depending on whether the BLOB is persistent or temporary.

**Router (both API v1 and API v2):**

API v1 (`blobHTTPRequestHandler_Write`, `blobHTTPRequestHandler_Read`) already calls `withLogAttribs()`. API v2 blob handlers must also be updated to call `withLogAttribs()`, so both paths set identical context attributes and emit `routing.accepted`.

- `extension`:
  - Write: `sys._Blob_Write`
  - Read: `sys._Blob_Read`

**Attributes set by Blob Processor (both read and write):**

Attribute key strings are defined as local constants within the blob processor package.

- `ownerqname` (string): QName of the record that owns the BLOB (e.g. `air.Bill`)
- `ownerfield` (string): field name on the owner record that holds the BLOB reference
- `ownerid` (string): record ID of the owner record (for read only; for write the owner record does not exist yet at this point)
- `blobid` (string): BLOB ID (numeric record ID or SUUID):
  - **Read**: added to context at the start of processing (BLOB ID is known from the request URL)
  - **Write**: added to context right after `blobStorage.WriteBLOB()` completes successfully

**Read:**

- Logs success: level `Verbose`, stage `bp.success`, msg (empty)

**Write:**

- After all validation succeeds (`validateQueryParams` completes): level `Verbose`, stage `bp.meta`, msg `name=<name>,contenttype=<type>`
- After `registerBLOB`: level `Verbose`, stage `bp.register.success`, msg (empty)
- After `blobStorage.WriteBLOB()` — adds `blobid` attrib to context, then: level `Verbose`, stage `bp.write.success`, msg (empty)
- After setting BLOB status to `completed`: level `Verbose`, stage `bp.setcompleted.success`, msg (empty)

**Error handling:**

Context may include `blobid`, `ownerqname`, `ownerfield`, `ownerid` attribs.

- Logs error: level `Error`, stage `bp.error`, msg `<error message>`
  - If the error is `SysError` with 400 Bad Request status code then url query and BLOBs-related headers are added to the msg comma-separated

### N10N Processor

**Router (both API v1 and API v2):**

API v1 n10n handlers must be updated to call `withLogAttribs()`, matching API v2 behavior. Both paths set `vapp`, `reqid`, `wsid`, `origin`, and `extension`:

- Subscribe+Watch (`POST .../notifications`): `extension` = `sys._N10N_SubscribeAndWatch`
  - Requires adding `processors.APIPath_N10N_SubscribeAndWatch` and a new case in `apiPathToExtension()` returning `"sys._N10N_SubscribeAndWatch"`
- Subscribe-extra (`PUT .../subscriptions/{entity}`): `extension` = entity QName (e.g. `air.view_price`)
- Unsubscribe (`DELETE .../subscriptions/{entity}`): `extension` = entity QName (e.g. `air.view_price`)

**Component-local attribute keys** (defined as local constants in the n10n processor package, not in `logger/consts.go`):

- `channelid` (string): N10N channel UUID — added to context after channel creation
- `projectionkey` (string): JSON array of all projection keys serialized via `in10n.ProjectionKeysToJSON()` — always added to context when there is at least one projection key

**Errors handling:**

- Logs error: level `Error`, stage `n10n.error`, msg `<error message>`
  - The context from the processor pipeline is used (may include `projectionkey` and `channelid` attribs)
  - If the error is `SysError` with 400 Bad Request status code then:
    - Subscribe+Watch → `,body=<compacted request body>` is appended to the msg
    - Subscribe-extra → `,projectionkey=<projection key>` is appended to the msg
    - Unsubscribe → `,projectionkey=<projection key>` is appended to the msg

**Subscribe+Watch flow:**

- Adds `projectionkey` attrib (JSON array of all keys) after subscribe loop; adds `channelid` attrib after channel creation
- Logs successful `IN10NBroker.SubscribeAndWatch()` call: level `Verbose`, stage `n10n.subscribe&watch.success`, msg empty
- Logs each SSE message: level `Verbose`, stage `n10n.sse_sent`, msg `<sse message>`
- Logs error on fail to send SSE message: level `Error`, stage `n10n.watch.sse_error`, msg `<error>`
- Logs `WatchChannel` goroutine finish: level `Verbose`, stage `n10n.watch.done`, msg (empty) — logged in both APIv1 (`serveN10NChannel`) and APIv2 (`watchChannel` goroutine)

**Subscribe-extra flow (APIv1 `subscribeHandler` and APIv2 `impl_subscribeextra.go`):**

- Adds `projectionkey` attrib (JSON array of all keys) to context when at least one projection key is present
- Logs successful `IN10NBroker.Subscribe()` call: level `Verbose`, stage `n10n.subscribe.success`, msg empty

**Unsubscribe flow (APIv1 `unSubscribeHandler` and APIv2 `impl_unsubscribe.go`):**

- Adds `projectionkey` attrib (JSON array of all keys) to context
- Logs successful `IN10NBroker.Unsubscribe()` call: level `Verbose`, stage `n10n.unsubscribe.success`, msg empty

**N10N Broker lifecycle (in10nmem):**

A dedicated log context is created inside `NewN10nBroker` with `vapp="sys/voedger"` and `extension="sys._N10NBroker"`. All broker lifecycle log calls use this context.

- Notifier goroutine started: level `Info`, stage `n10n.notifier.start`, msg (empty)
- Notifier goroutine stopped: level `Info`, stage `n10n.notifier.stop`, msg (empty)
- Heartbeat goroutine started: level `Info`, stage `n10n.heartbeat.start`, msg `Heartbeat30Duration: <duration>`
- Heartbeat goroutine stopped: level `Info`, stage `n10n.heartbeat.stop`, msg (empty)
- Channel expired during `WatchChannel`: level `Error`, stage `n10n.channel.expired`, msg `<subjectLogin>`
- Channel cleanup unsubscribe error: level `Error`, stage `n10n.cleanup.error`, attribs `channelid=<id>`, `projectionkey=[<key>]`, msg `<error>`

## Schedulers

**Attributes:**

- `vapp`, `extension`, and `wsid` attribs are set on app partition deployment on start new actualizers stage. `extension` is `job.<job QName>`, `wsid` is the workspace ID where the job runs

**Job execution:**

- Logs schedule: level `Verbose`, stage `job.schedule`, msg `now=<timeNow>,next=<nextRunTime>`
- Logs wake-up: level `Verbose`, stage `job.wake-up`, msg `<timeNow>`
- Logs successful invoke: level `Verbose`, stage `job.success`, msg (empty)
- Logs invocation error (`runJob` defer): level `Error`, stage `job.error`, msg `<error>`
- Logs retrier error (`Prepare` OnError):
  - If `appparts.ErrNotFound`: level `Error`, stage `job.error`, msg `appparts <error>, will try again`
  - Otherwise: level `Error`, stage `job.error`, msg `<error>`

---

## Key components

### Core logging infrastructure

**[logger package](../../../../pkg/goutils/logger)**

Provides structured logging with context-aware attribute propagation.

- **Files:**
  - `logger.go`: Core logging functions and level management
  - `loggerctx.go`: Context-aware logging functions
  - `consts.go`: Standard attribute constants and slog configuration
  - `types.go`: Internal types for context key and attribute chain
  - `impl.go`: Implementation details (level checking, caller tracking, formatting)

- **Key features:**
  - Hierarchical log levels (Error, Warning, Info, Verbose, Trace)
  - Atomic level checking for thread-safe filtering
  - Automatic caller tracking (function name and line number)
  - Context-based attribute propagation
  - slog integration for structured output

- **Used by:** All request processing components (router, command processor, query processor, actualizers)

### Context management

**[logger.WithContextAttrs](../../../../pkg/goutils/logger/loggerctx.go#L23)**

```go
func WithContextAttrs(ctx context.Context, attrs map[string]any) context.Context
```

Adds logging attributes to context for propagation through call chain.

- **Implementation:**
  - Stores attributes in linked-list chain (`logAttrs` struct)
  - O(1) attribute addition by prepending new node
  - Shadowing support: later calls override earlier ones for same key
  - Thread-safe: immutable chain structure

- **Usage pattern:**

  ```go
  ctx = logger.WithContextAttrs(ctx, map[string]any{
      logger.LogAttr_ReqID: "03061504-42",
      logger.LogAttr_WSID:  1001,
  })
  ```

- **Used by:**
  - Router: initial request context
  - Command processor: event and CUD attributes
  - Actualizers: workspace and event attributes

**[sLogAttrsFromCtx](../../../../pkg/goutils/logger/loggerctx.go#L89)**

Internal function that extracts attributes from context chain.

- Walks linked list from newest to oldest
- First-seen-wins per key (implements shadowing)
- Returns slice of `slog.Any` attributes

### Logging functions

**Context-aware functions ([loggerctx.go](../../../../pkg/goutils/logger/loggerctx.go#L31))**

```go
func VerboseCtx(ctx context.Context, stage string, args ...interface{})
func ErrorCtx(ctx context.Context, stage string, args ...interface{})
func InfoCtx(ctx context.Context, stage string, args ...interface{})
func WarningCtx(ctx context.Context, stage string, args ...interface{})
func TraceCtx(ctx context.Context, stage string, args ...interface{})
func LogCtx(ctx context.Context, skipStackFrames int, level TLogLevel, stage string, args ...interface{})
```

Automatically append context attributes and stage to log entries using slog.

- **Parameters:**
  - `ctx`: Context containing logging attributes (vapp, reqid, wsid, extension, etc.)
  - `stage`: Processing stage that emits the log entry; added to the log as the `stage` slog attribute with key `logger.LogAttr_Stage`. Convention: `<component>.<substage>` (e.g., `"routing"`, `"cp.received"`, `"cp.plog_saved"`)
  - `args`: Message components to be formatted via `fmt.Sprint()`
  - `skipStackFrames` (LogCtx only): Number of stack frames to skip for source location
  - `level` (LogCtx only): Log level for the entry

- **Implementation:**
  - Extracts attributes from context via `sLogAttrsFromCtx()`
  - Adds `stage` parameter as a log attribute with key `logger.LogAttr_Stage`
  - Adds source location (`src` attribute with function:line)
  - Formats message via `fmt.Sprint(args...)`
  - Routes to slogOut (stdout) or slogErr (stderr) based on level
  - Respects global log level via `isEnabled()` check

- **Used by:**
  - Router: request acceptance, error logging
  - Processors: error, success, event/CUD logging
  - Sync Actualizers: error, success logging
  - Async Actualizers: error, success, event/CUD logging

- **Usage example:**

  ```go
  logger.VerboseCtx(ctx, "routing", "request accepted")
  logger.ErrorCtx(ctx, "cp.error", "command failed:", err)
  logger.InfoCtx(ctx, "cp.partition_recovery.complete", "completed, nextPLogOffset:", offset)
  ```

**Standard functions ([logger.go](../../../../pkg/goutils/logger/logger.go#L44))**

```go
func Error(args ...interface{})
func Warning(args ...interface{})
func Info(args ...interface{})
func Verbose(args ...interface{})
func Trace(args ...interface{})
func Log(skipStackFrames int, level TLogLevel, args ...interface{})
```

Non-context-aware logging functions (legacy).

- Used by query processor (opportunity for migration)
- Used by components that don't have request context

### Standard attributes

**[Attribute constants](../../../../pkg/goutils/logger/consts.go#L18)**

```go
const (
    LogAttr_VApp      = "vapp"      // Voedger application QName
    LogAttr_Feat      = "feat"      // Feature name
    LogAttr_ReqID     = "reqid"     // Request ID
    LogAttr_WSID      = "wsid"      // Workspace ID
    LogAttr_Extension = "extension" // Extension/function name
    LogAttr_Stage     = "stage"     // Processing stage name
)
```

Ensures consistent attribute naming across all components.

Component-specific attribute keys used only within a single package (e.g. `blobid`, `ownerqname`, `ownerfield`, `ownerid`, `channelid`, `projectionkey`) are defined as local string constants within their respective packages, not in `logger/consts.go`.

### LogEventAndCUDs

**[processors.LogEventAndCUDs](../../../../pkg/processors/utils.go#L101)**

```go
func LogEventAndCUDs(logCtx context.Context, event istructs.IPLogEvent,
    pLogOffset istructs.Offset, appDef appdef.IAppDef,
    skipStackFrames int, stage string,
    perCUDLogCallback func(istructs.ICUDRow) (bool, string, error),
    eventMessageAdds string) (enrichedCtx context.Context, err error)
```

Shared utility for logging PLog events and their CUDs with consistent context enrichment. Used by command processor and async actualizers.

- Does nothing and returns `logCtx` unchanged if `!logger.IsVerbose()`
- Enriches context with `woffset`, `poffset`, `evqname` attributes
- Logs event arguments as JSON at Verbose level: stage `<stage>`, msg `args={...}{eventMessageAdds}`
- For each CUD:
  - Calls `perCUDLogCallback` to get `shouldLog`, `msgAdds`, `err`
  - If `err != nil`, returns the error
  - If `shouldLog` is false, skips the CUD
  - Enriches context with `rectype`, `recid`, `op`
  - Logs new fields as JSON at Verbose level: stage `<stage>.log_cud`, msg `newfields={...}{msgAdds}`
- Returns the context enriched with `woffset`, `poffset`, `evqname`; callers must use it for subsequent logging

**Parameters:**

- `stage`: stage name for the event log entry; CUD entries use `<stage>.log_cud`
- `skipStackFrames`: call-stack frames to skip for the correct `src` attribute
- `perCUDLogCallback`: per-CUD filter; `shouldLog` — whether to log this CUD; `msgAdds` — text appended to the `newfields` message
- `eventMessageAdds`: extra text appended to the pre-CUD event log message (e.g. `triggeredby=<QName>` for actualizers, empty for command processor)
- `plogOffset`: used as the `poffset` context attribute
- `appDef`: used to marshal CUD field values to JSON
- `event`: source of `woffset`, event arguments, and CUDs

### slog integration

**[Handler configuration](../../../../pkg/goutils/logger/consts.go#L26)**

```go
ctxHandlerOpts = &slog.HandlerOptions{
    Level: slog.LevelDebug,
}
slogOut = slog.New(slog.NewTextHandler(os.Stdout, ctxHandlerOpts))
slogErr = slog.New(slog.NewTextHandler(os.Stderr, ctxHandlerOpts))
```

- slog level is DEBUG already. Actual log level controlling is done by logger's funcs
- Separate handlers for stdout and stderr

### Component integrations

**[Router logging](../../../../pkg/router/utils.go#L143)**

```go
func withLogAttribs(ctx context.Context, data validatedData,
    busRequest bus.Request, req *http.Request) context.Context {
    extension := busRequest.Resource
    if busRequest.IsAPIV2 {
        if busRequest.QName == appdef.NullQName {
            extension = apiPathToExtension(processors.APIPath(busRequest.APIPath))
        } else {
            extension = busRequest.QName.String()
        }
    }
    newReqID := fmt.Sprintf("%s-%d", globalServerStartTime, reqID.Add(1))
    return logger.WithContextAttrs(ctx, map[string]any{
        logger.LogAttr_ReqID:     newReqID,
        logger.LogAttr_WSID:      data.wsid,
        logger.LogAttr_VApp:      data.appQName,
        logger.LogAttr_Extension: extension,
        logAttrib_Origin:         req.Header.Get(httpu.Origin),
    })
}
```

Creates initial request context with logging attributes.

## Key data models

### logAttrs (internal)

```go
type logAttrs struct {
    attrs  map[string]any
    parent *logAttrs
}
```

Linked-list node for storing logging attributes in context.

- Immutable chain structure for thread safety
- Parent pointer creates chain
- Newest attributes shadow older ones with same key

### ctxKey (internal)

```go
type ctxKey struct{}
```

Unexported context key type for storing `logAttrs` in context.

- Prevents key collisions with other context values
- Type-safe context value access

### TLogLevel

```go
type TLogLevel int32

const (
    LogLevelNone = TLogLevel(iota)
    LogLevelError
    LogLevelWarning
    LogLevelInfo
    LogLevelVerbose
    LogLevelTrace
)
```

Log level enumeration with atomic operations support.
