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

- **vapp** (string): Voedger application qualified name
  - Constant: `logger.LogAttr_VApp`
  - Example: "untill/fiscalcloud", "untill/airsbp", "sys/voedger"
  - Set by: Router at request entry point
  - Purpose: Identify which application is processing the request

- **feat** (string): Feature name within the application
  - Constant: `logger.LogAttr_Feat`
  - Example: "magicmenu"
  - Set by: Application-specific handlers
  - Purpose: Track feature-level activity

- **reqid** (string): Unique request identifier
  - Constant: `logger.LogAttr_ReqID`
  - Format: "{serverStartTime-mmddHHMM}-{atomicCounter}"
  - Example: "03061504-42"
  - Set by: Router using server start time and atomic counter
  - Purpose: Trace single request through all processing stages
  - Set by: Router using global atomic counter
  - Format: "{Server start time (MMDDHHmm)}-{atomicCounter}"
  - Example: "26031402-42"

- **vapp** (string): Voedger application qualified name
  - Purpose: Identify which application is processing the request
  - Set by: Processing initiator
    - Router at request entry point: `sys.registry`, `untill.fiscalcloud`
    - Voedger on bootstrapping: `sys.voedger`
  
- **wsid** (int): Workspace ID
  - Constant: `logger.LogAttr_WSID`
  - Example: 1001
  - Set by: Router from validated request data, actualizers when processing events
  - Purpose: Filter logs by workspace for multi-tenant debugging
  - Set by: Router from validated request data
  - Example: 1001

- **extension** (string): Extension or function being executed
  - Constant: `logger.LogAttr_Extension`
  - Example: "c.sys.UploadBLOBHelper", "q.sys.Collection", "sys._Docs"
    - "sys._Docs": API v2, working with documents
  - Set by: Router based on request resource/QName or API path
  - Purpose: Identify specific command/query/function in logs
  - Example: `c.sys.UploadBLOBHelper`, `q.sys.Collection`
  - Set by: Processing initiator
    - Router: based on request resource/QName

- **feat** (string): Feature name within the application
  - Purpose: Track feature-level activity
  - Set by: logger from the `feat` argument of context-aware logging functions
  - Examples: `magicmenu`
  
- **stage** (string): Processing stage name
  - Purpose: Identify which stage of processing a log entry corresponds to
  - Examples: `request parsed`, `before save plog`, `after save plog`
    - `latency1`: `routing` stage for first response latency measurement, milliseconds
  - Set by: logger from the `stage` argument of context-aware logging functions

## General scenarios

- App enriches request context with logging attributes (vapp, reqid, wsid, extension)
- App log specifying the `context`, `stage`, []args as parameters
  - stage argument becomes a log attribute with the key `stage`

## Per-component scenarios

### Router

**Initialization:**

- Creates root log context with `vapp="sys/voedger"` in `preRun()`
- Sets as base context for all HTTP connections

**Request handling:**

- Validates request data (app, workspace, resource)
- Generates unique request ID: `fmt.Sprintf("%s-%d", globalServerStartTime, reqID.Add(1))`
  - `globalServerStartTime` format: "mmddHHMM" (e.g., "03061504" for March 6, 15:04)
  - `reqID` is atomic counter incremented per request
- Creates request context with attributes:
  - `vapp`: Application QName from validated data
  - `reqid`: Generated request ID
  - `wsid`: Workspace ID from validated data
  - `extension`: Resource name (API v1) or QName/API path (API v2)
  - `origin`: HTTP Origin header value
- Logs "request accepted" at Verbose level when request is received
- Logs errors when sending request to VVM fails at Error level with body

**Logging functions used:**

- `logger.LogCtx(ctx, skipFrames, level, args...)` for request acceptance
- `logger.ErrorCtx(ctx, args...)` for request sending errors

### Command Processor

**Request processing:**

- Receives context with attributes from Router
- Logs command handling errors at Error level with compacted request body
- Logs successful command execution at Verbose level with result and compacted body
- Logs partition restart warnings at Warning level with partition ID and error

**Event and CUD logging:**

- Uses shared `processors.LogEventAndCUDs()` utility
- Enriches context with event attributes: `woffset`, `poffset`, `evqname`
- Logs event arguments as JSON
- For each CUD, enriches context with: `rectype`, `recid`, `op`
- Logs new fields as JSON and old fields as JSON (for updates)
- Guarded by `logger.IsVerbose()` check

**Partition recovery:**

- Logs partition recovery completion at Info level with nextPLogOffset and workspaces JSON

**Logging functions used:**

- `logger.LogCtx(ctx, skipFrames, level, args...)` for errors and success
- `logger.ErrorCtx(ctx, args...)` via `logHandlingError()`
- `logger.VerboseCtx(ctx, args...)` via `logSuccess()` and event/CUD logging
- `logger.WarningCtx(ctx, args...)` for partition restart warnings
- `logger.InfoCtx(ctx, args...)` for partition recovery

**Metrics reported:**

- `CommandsTotal`: Total commands processed
- `CommandsSeconds`: Command processing duration
- `ErrorsTotal`: Total errors encountered
- `ProjectorsSeconds`: Sync projector execution duration

### Query Processor

**Request processing:**

- Receives context with attributes from Router
- Logs query execution errors at Error level with WSID and QName
- Logs rowsProcessor errors at Error level
- Logs response sending errors at Error level

**Current limitations:**

- Uses standard `logger.Error()` instead of context-aware `logger.ErrorCtx()`
- Does not propagate request context attributes to log entries
- Opportunity for improvement: migrate to context-aware logging

**Logging functions used:**

- `logger.Error(args...)` for query execution errors and rowsProcessor errors

**Metrics reported:**

- `QueriesTotal`: Total queries processed
- `QueriesSeconds`: Query processing duration
- `ErrorsTotal`: Total errors encountered
- `ExecSeconds`: Query execution duration
- `BuildSeconds`: Rows processor build duration
- `ExecFieldsSeconds`, `ExecEnrichSeconds`, `ExecFilterSeconds`, `ExecOrderSeconds`, `ExecCountSeconds`, `ExecSendSeconds`: Detailed execution metrics

### Actualizer (Async Projectors)

**Initialization:**

- Creates base log context with `vapp` and `extension` (projector QName) when projector runtime starts
- Context propagates from VVM context through actualizer deployment

**Event processing:**

- Adds `wsid` to log context in `DoAsync()` when processing event
- Determines triggering QName via `ProjectorEvent()`
- Uses shared `processors.LogEventAndCUDs()` utility
- Enriches context with event attributes: `woffset`, `poffset`, `evqname`
- Logs event with `triggeredby=<QName>` prefix
- Filters CUDs based on trigger type:
  - Function-triggered: logs all CUDs
  - ODoc/ORecord-triggered: logs all CUDs
  - Other triggers: logs only CUDs matching trigger QName

**Error handling:**

- Wraps errors with context via `errWithCtx{error, logCtx}`
- `asyncActualizer.logError()` extracts context and uses `logger.ErrorCtx()`
- Falls back to VVM context if error doesn't contain context
- Errors trigger `ProjectorsInError` metric increment

**Logging functions used:**

- `logger.ErrorCtx(ctx, args...)` for projector errors
- `logger.VerboseCtx(ctx, args...)` via shared event/CUD logging
- `logger.LogCtx(ctx, skipFrames, level, args...)` via shared utilities

**Metrics reported:**

- `ProjectorsInError`: Number of projectors currently in error state
- `aaFlushesTotal`: Total flushes performed (internal)
- `aaCurrentOffset`: Current processing offset (internal)
- `aaStoredOffset`: Last stored offset (internal)

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
func VerboseCtx(ctx context.Context, args ...interface{})
func ErrorCtx(ctx context.Context, args ...interface{})
func InfoCtx(ctx context.Context, args ...interface{})
func WarningCtx(ctx context.Context, args ...interface{})
func TraceCtx(ctx context.Context, args ...interface{})
func LogCtx(ctx context.Context, skipStackFrames int, level TLogLevel, args ...interface{})
```

Automatically append context attributes to log entries using slog.

- **Implementation:**
  - Extracts attributes from context via `sLogAttrsFromCtx()`
  - Adds source location (`src` attribute with function:line)
  - Formats message via `fmt.Sprint(args...)`
  - Routes to slogOut (stdout) or slogErr (stderr) based on level
  - Respects global log level via `isEnabled()` check

- **Used by:**
  - Router: request acceptance, error logging
  - Command processor: error, success, event/CUD logging
  - Actualizers: error and event logging

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
)
```

Ensures consistent attribute naming across all components.

### slog integration

**[Handler configuration](../../../../pkg/goutils/logger/consts.go#L26)**

```go
ctxHandlerOpts = &slog.HandlerOptions{
    Level: slog.LevelDebug - slogLevelTrace,
    ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
        if a.Key == slog.LevelKey {
            switch a.Value.Any().(slog.Level) {
            case slog.LevelDebug:
                a.Value = slog.StringValue("VERBOSE")
            case slog.LevelDebug - slogLevelTrace:
                a.Value = slog.StringValue("TRACE")
            }
        }
        return a
    },
}
slogOut = slog.New(slog.NewTextHandler(os.Stdout, ctxHandlerOpts))
slogErr = slog.New(slog.NewTextHandler(os.Stderr, ctxHandlerOpts))
```

- Maps logger levels to slog levels
- Customizes level names (VERBOSE, TRACE instead of DEBUG, DEBUG-4)
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
    skipStackFrames int, perCUDLogCallback func(istructs.ICUDRow) (bool, string, error),
    preCUDMessage string) (enrichedCtx context.Context, err error)
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
