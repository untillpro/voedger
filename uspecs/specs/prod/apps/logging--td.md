# Technical design: logging

## Overview

The logging subsystem provides structured, context-aware logging with automatic attribute propagation through the request processing pipeline. It uses Go's standard `log/slog` package as the underlying engine while maintaining custom log levels and adding context-based attribute management.

## Concepts

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

---

## Per-component logging

### Server core events

### HTTP

HTTP root context is derived from VVM context:

- `vapp="sys/voedger"`
- `extension` = server name: `sys._HTTPServer`, `sys._AdminHTTPServer`, `sys._HTTPSServer`, or `sys._ACMEServer`
Used for logging server start/stop operations and for all incoming HTTP requests:

- Router params validation failure: level `Error`, stage `endpoint.validation`, msg `<error message>`
- Start accepting connections success: level `Info`, stage `endpoint.listen.start`, msg `<addr>:<port>`
- Start accepting connections failure: level `Error`, stage `endpoint.listen.error`, msg `<error message>`
- Server stops accepting connections: level `Info`, stage `endpoint.shutdown`, msg (empty)
- Error on http server shutdown: level `Error`, stage `endpoint.shutdown.error`, msg `<error message>`
- Server exits unexpectedly: level `Error`, stage `endpoint.unexpectedstop`, msg `Serve() error: <err>` or `ServeTLS() error: <err>`

#### Application deployment

`btstrp.Bootstrap()` is called. Uses `vapp="sys/voedger"`, `extension="sys._Bootstrap"` attribs.

- Bootstrap starts: level `Info`, stage `bootstrap`, msg `started`
- Cluster app workspace initialized: level `Info`, stage `bootstrap`, msg `cluster app workspace initialized`
- For each built-in and sidecar app: level `Info`, stage `bootstrap.appdeploy`, msg `<appQName>`
- For each app partition: level `Info`, stage `bootstrap.apppartdeploy`, msg `<appQName>/<partID>`
- Bootstrap completes: level `Info`, stage `bootstrap`, msg `completed`
- On app deployment failure: panics with `failed to deploy app <appName>: <error>` (no logging)

#### Leadership acquisition

Uses `vapp="sys/voedger"`, `extension="sys._Leadership"`, `key` attribs.

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
- First response received from processor: level `Verbose`, stage `routing.latency1`, msg `<latency_ms>`
- Error sending request to VVM: level `Error`, stage `routing.send2vvm.error`, msg `<error message>`
- Error sending response to client: level `Error`, stage `routing.response.error`, msg `<error message>`

---

### Command Processor

**Request processing:**

The context with attributes is received from Router

- Logs the event details right after successful write to PLog using `processors.LogEventAndCUDs()` providing `cp.plog_saved` as the stage, callback that returns `true`, `oldfields={...}` and `nil` error for each CUD
  - note: old fields for each CUD that came with http request. No old fields for CUDs created by the command
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

**Event and CUD logging:**

- Done in shared `processors.LogEventAndCUDs()` utility
- Do nothing if `!logger.IsVerbose()`
- Args:
  - `stage string`
  - `skipStackFrames int`
  - `perCUDCallback func (ICUD) (shouldLog bool, msgAdds string, err error)`
    - `shouldLog` - whether to log the CUD. Always true for command processor, for actualizer it's false if the CUD is not the triggering one
    - `msgAdds` - additional message to append to the cud log message, e.g. `,oldfields={...}` for command processor, empty for actualizer
  - `plogOffset Offset`
  - `appDef IAppDef`, need to marshal a CUD to JSON
  - `event IPlogEvent`, need to get `woffset` and the event arguments
  - `eventMessageAdds string` - additional message to append to the pre-cuds event log message before iteration over CUDs. Empty for command processor, `triggeredby=<...>` for actualizers
- Enriches context with event attributes: `woffset`, `poffset`, `evqname`
- Logs event arguments as JSON `args={...}`, provided `stage` is used, level `Verbose`
- For each CUD:
  - calls `perCUDCallback` to get `shouldLog`, `msgAdds` and `err`
  - if `err` is not nil, fails with it
  - if `shouldLog` is false, skips the CUD
  - enriches context with: `rectype`, `recid`, `op`
  - Logs new fields as JSON: `msg=newfields={...}{msgAdds}`, stage `{stage}.log_cud`, level `Verbose`
- Returns the context enriched by `woffset`, `poffset`, `evqname`. Need for use on logging on sync projectors stage

**Partition recovery:**

- `vapp` attrib is replaced with `sys/voedger`
- `extension` attrib: `sys._Recovery`
- `partid` attrib: partition ID
- Partition recovery start: level `Info`, stage `cp.partition_recovery.start`, msg (empty)
- Partition recovery complete: level `Info`, stage `cp.partition_recovery.complete`, msg `completed`, nextPLogOffset and workspaces JSON
- Partition recovery failure: level `Error`, stage `cp.partition_recovery.error`, msg `<error message>`

---

### Query Processor

**Request processing:**

The context with attributes is received from Router

- query execution error: level `Error`, stage `qp.error`, msg `<error message>`

### Sync Projectors

Launched by command processor between `ApplyRecords` and `PutWLog` stages

- Use the context from `processors.LogEventAndCUDs()` with attribs `woffset`, `poffset`, `evqname`
- Command processor logs:
  - After all sync projectors success: level `Verbose`, stage `sp.success`, msg (empty)
  - Logs the projector error: level `Error`, stage `sp.error`, msg `<error message>`
- Each triggered sync projector:
  - Logs the trigger QName right before `IAppParts.Invoke()`: level `Verbose`, stage `sp.triggeredby`, msg `<triggered by qname>`, extension `<projector QName>`
  - After success Invoke: level `Verbose`, stage `sp.success`, `extension`=`sp.<projector QName>`, msg (empty)

### Async Projectors

Attributes:

- `vapp` is determined before event is sent to pipeline (asyncActualizer.pipeline), enriched context is sent to the pipeline
- `extension` is determined inside the pipeline based on event QName prefixed with `ap.`, context is enriched

**Event processing:**

Stage is `ap`

- Determines if the projector triggered by the current event via `ProjectorEvent()`
- Adds `wsid` to log context in `DoAsync()` when processing event
- Uses shared `processors.LogEventAndCUDs("ap")` utility with args:
  - cud callback filters CUDs based on trigger type:
    - Function-triggered: logs all CUDs
    - ODoc/ORecord-triggered: logs all CUDs
    - Other triggers: logs only CUDs matching trigger QName
  - eventMessageAdds: `triggeredby=<QName>`
- Constructs event context - merge the context got from `processors.LogEventAndCUDs` and the ctx with `vapp` and `extension` attribs
- Stores the event context in the pipeline workpiece to use it on error logging

**Error handling:**

Done in `asyncErrorHandler.OnError()` handler

- Uses the event context
- Logs the error: level `Error`, stage `ap.error`, msg `<error message>`

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

**[Shared event/CUD logging](../../../../pkg/processors/utils.go#L101)**

```go
func LogEventAndCUDs(logCtx context.Context, event istructs.IPLogEvent,
    pLogOffset istructs.Offset, appDef appdef.IAppDef,
    skipStackFrames int, stage string, perCUDLogCallback func(istructs.ICUDRow) (bool, string, error),
    eventMessageAdds string) (enrichedCtx context.Context, err error)
```

Shared utility for logging events and CUDs with consistent formatting.

- Enriches context with `woffset`, `poffset`, `evqname`
- Logs event arguments as JSON
- For each CUD: enriches context with `rectype`, `recid`, `op`
- Logs new fields as JSON
- Callback allows component-specific filtering and extra messages
- Used by command processor and actualizers

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
