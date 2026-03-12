# Implementation plan: Design logging subsystem architecture

## Functional design

- [x] update: [prod/prod--domain.md](../../specs/prod/prod--domain.md)
  - update: Rename context from `monitoring` to `observability`
  - update: Context map mermaid diagram (monitoring -> observability)
  - update: Detailed relationships section (monitoring -> observability)
  - update: Context description section header (monitoring -> observability)
  - update: External actor relationships (monitoring -> observability)

## Technical design

- [x] create: [prod/apps/logging--td.md](../../specs/prod/apps/logging--td.md)
  - add: Logging subsystem architecture document
  - add: Overview of structured logging approach
  - add: Key components (logger package, context propagation, log attributes)
  - add: Key data models (log attributes: vapp, reqid, wsid, extension, duration)
  - add: Integration points with existing logger package
  - add: Tracing strategy for different processing stages

## Construction

### Stage naming conventions

Stage values describe the processing phase or operation context:

- Router stages:
  - `"request accepted"` - Request received and validated
  - `"send to vvm"` - Sending request to VVM
  - `"server"` - Server-level operations
  - `"latency1"` - First response latency measurement (routing stage duration in milliseconds)
- Command processor stages:
  - `"command handling"` - Command processing (success/error)
  - `"partition recovery"` - Partition recovery completion
  - `"partition restart"` - Partition restart warning
  - `"acl check"` - ACL validation
  - `"notify actualizers"` - Notifying async actualizers
  - `"marshal response"` - Response marshaling
- Event/CUD logging stages:
  - `"log event"` - Event logging
  - `"log cud"` - CUD (Create/Update/Delete) logging
- Actualizer stages:
  - `"projector error"` - Projector execution error
  - `"projector success"` - Projector execution success
  - `"read offset"` - Reading offset from storage
  - `"read plog"` - Reading PLog
  - `"notification received"` - N10n notification received
- Test stages:
  - `"test"` - All test logging calls

### Logger package updates

- [ ] update: [pkg/goutils/logger/consts.go](../../../pkg/goutils/logger/consts.go)
  - add: `LogAttr_Stage` constant with value "stage"

- [ ] update: [pkg/goutils/logger/loggerctx.go](../../../pkg/goutils/logger/loggerctx.go)
  - update: `VerboseCtx` signature to include `stage string` parameter
  - update: `ErrorCtx` signature to include `stage string` parameter
  - update: `InfoCtx` signature to include `stage string` parameter
  - update: `WarningCtx` signature to include `stage string` parameter
  - update: `TraceCtx` signature to include `stage string` parameter
  - update: `LogCtx` signature to include `stage string` parameter (after level parameter)
  - update: `logCtx` internal function to add stage as a log attribute with key `LogAttr_Stage`

### Update all usages

- [ ] update: [pkg/goutils/logger/logger_test.go](../../../pkg/goutils/logger/logger_test.go)
  - update: All `VerboseCtx`, `ErrorCtx`, `InfoCtx`, `WarningCtx`, `TraceCtx`, `LogCtx` calls to include stage parameter
  - use: `"test"` as stage value for all test calls

- [ ] update: [pkg/router/utils.go](../../../pkg/router/utils.go)
  - update: `logServeRequest()` - `LogCtx` call with stage `"request accepted"`

- [ ] update: [pkg/router/impl_http.go](../../../pkg/router/impl_http.go)
  - update: `log()` method - `LogCtx` call with stage `"server"`
  - update: Line 211 - `ErrorCtx` call with stage `"send to vvm"`
  - add: latency1 logging in `RequestHandler_V1` function
    - capture: start time after `logServeRequest(requestCtx)` call (line 207)
    - log: after `reply_v1()` completes (after line 217)
    - use: `InfoCtx` with stage `"latency1"` and message containing duration in milliseconds
    - format: `"<duration_ms>"`

- [ ] update: [pkg/router/impl_apiv2.go](../../../pkg/router/impl_apiv2.go)
  - update: Line 497 - `ErrorCtx` call with stage `"send to vvm"`
  - add: latency1 logging in `sendRequestAndReadResponse` function
    - capture: start time after `logServeRequest(requestCtx)` call (line 493)
    - log: after `reply_v2()` completes (after line 503)
    - use: `InfoCtx` with stage `"latency1"` and message containing duration in milliseconds
    - format: `"<duration_ms>"`

- [ ] update: [pkg/processors/command/provide.go](../../../pkg/processors/command/provide.go)
  - update: `logHandlingError()` - `LogCtx` call with stage `"command handling"`
  - update: `logSuccess()` - `LogCtx` call with stage `"command handling"`
  - update: Line 174 - `WarningCtx` call with stage `"partition restart"`

- [ ] update: [pkg/processors/command/impl.go](../../../pkg/processors/command/impl.go)
  - update: Line 320 - `InfoCtx` call with stage `"partition recovery"`
  - update: Line 461 - `VerboseCtx` call with stage `"acl check"`
  - update: Line 835 - `VerboseCtx` call with stage `"acl check"`
  - update: Line 868 - `VerboseCtx` call with stage `"notify actualizers"`
  - update: Line 897 - `ErrorCtx` call with stage `"marshal response"`

- [ ] update: [pkg/processors/utils.go](../../../pkg/processors/utils.go)
  - update: Line 123 - `LogCtx` call with stage `"log event"`
  - update: Line 149 - `LogCtx` call with stage `"log cud"`

- [ ] update: [pkg/processors/actualizers/async.go](../../../pkg/processors/actualizers/async.go)
  - update: Line 96 - `ErrorCtx` call with stage `"projector error"`
  - update: Line 98 - `ErrorCtx` call with stage `"projector error"`
  - update: Line 182 - `ErrorCtx` call with stage `"read offset"`
  - update: Line 249 - `VerboseCtx` call with stage `"notification received"`
  - update: Line 254 - `ErrorCtx` call with stage `"read plog"`
  - update: Line 460 - `VerboseCtx` call with stage `"projector success"`

- [ ] update: [pkg/processors/actualizers/impl.go](../../../pkg/processors/actualizers/impl.go)
  - update: All context-aware logging function calls to include stage parameter (if any exist)

### Tests

- [ ] run: Logger tests
  - `go test -short ./pkg/goutils/logger/...`

- [ ] Review
