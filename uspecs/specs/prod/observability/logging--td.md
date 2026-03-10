# Context subsystem architecture: logging

## Concepts

### Logging Attribute

A key-value pair that provides additional context to log entries. Attributes are stored in context.Context and propagate with the context through the request processing pipeline.

Predefined standard attributes include:

- **reqid** (string): Unique request identifier
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
  - Purpose: Filter logs by workspace for multi-tenant debugging
  - Set by: Router from validated request data
  - Example: 1001

- **extension** (string): Extension or function being executed
  - Purpose: Identify specific command/query/function in logs
  - Example: `c.sys.UploadBLOBHelper`, `q.sys.Collection`
  - Set by: Processing initiator
    - Router: based on request resource/QName

- **feat** (string): Feature name within the application
  - Purpose: Track feature-level activity
  - Set by: logger from the `feat` argument in log calls
  - Examples: `routing`, `magicmenu`
  
- **stage** (string): Processing stage name
  - Purpose: Identify which stage of processing a log entry corresponds to
  - Examples: `routing`, `before_save_plog`, `after_save_plog`
    - `latency1`: `routing` stage for first response latency measurement, milliseconds
  - Set by: logger from the `stage` argument in log calls

## General scenarios

- App enriches request context with logging attributes (vapp, reqid, wsid, extension)
- App log specifying the `context`, `feat`, `stage`, []args as parameters
  - feat and stage arguments become standard log attributes with the keys "feat" and "stage"

## Per-component scenarios

- Router
- Command Processor
- Query Processor
- Actualizer

## Key components

📦 System components:

- [logger package](../../../../pkg/goutils/logger)
  - Provides structured logging with context-aware attribute propagation
  - Supports hierarchical log levels (Error, Warning, Info, Verbose, Trace)
  - Implements `*Ctx` functions that read attributes from `context.Context`
  - Used by: All request processing components (router, command processor, query processor, event processor)

- [logger.WithContextAttrs](../../../../pkg/goutils/logger/loggerctx.go)
  - Adds logging attributes to context for propagation through call chain
  - Implements linked-list chain for O(1) attribute addition with shadowing support
  - Used by: Router (initial request context), processors (stage tracking)

- [Context-aware logging functions](../../../../pkg/goutils/logger/loggerctx.go)
  - `VerboseCtx`, `ErrorCtx`, `InfoCtx`, `WarningCtx`, `TraceCtx`
  - Automatically append context attributes to log entries using slog
  - Used by: Command processor, query processor, event processor

- [Standard log attributes](../../../../pkg/goutils/logger/consts.go)
  - `LogAttr_Stage`: Stage name
  - `LogAttr_VApp`: Voedger application name (e.g., "untill.fiscalcloud")
  - `LogAttr_Feat`: Feature name (e.g., "magicmenu")
  - `LogAttr_ReqID`: Request ID for tracing (e.g., "20260306-42")
  - `LogAttr_WSID`: Workspace ID (e.g., 1001)
  - `LogAttr_Extension`: Extension/function name (e.g., "c.sys.UploadBLOBHelper")
  - Used by: Router, processors for consistent attribute naming

- [Router logging integration](../../../../pkg/router/utils.go)
  - Creates initial request context with vapp, reqid, wsid, extension attributes
  - Generates unique request IDs using server start time and atomic counter
  - Used by: HTTP request handlers

- [Command processor logging](../../../../pkg/processors/command/provide.go)
  - Logs command handling errors and success with request context
  - Includes command body in log entries for debugging
  - Used by: Command execution pipeline
