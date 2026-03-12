# Implementation plan: Use context-aware logging in actualizers

## Construction

- [x] update: [pkg/appparts/impl.go](../../../pkg/appparts/impl.go)
  - update: pass the base `vvmCtx` into actualizer deployment and let actualizer startup attach actualizer-specific log attrs

- [x] update: [pkg/appparts/internal/actualizers/actualizers.go](../../../pkg/appparts/internal/actualizers/actualizers.go)
  - update: build the async actualizer base `logCtx` with `vapp` and `extension` when starting projector runtimes

- [x] update: [pkg/processors/utils.go](../../../pkg/processors/utils.go)
  - add: `cudOpToStringForLog(cud istructs.ICUDRow) string` — shared helper mapping `IsNew/IsActivated/IsDeactivated` to `"create"/"activate"/"deactivate"/"update"`
  - add: `processors.LogEventAndCUDs(...)` — shared event/CUD logging skeleton with args JSON logging, event attrs, per-CUD attrs, shared `newfields=%s` logging, `skipStackFrames`, and one callback that decides whether to log a CUD and what extra message to append

- [x] update: [pkg/processors/command/impl.go](../../../pkg/processors/command/impl.go)
  - update: delegate common event/CUD logging to `processors.LogEventAndCUDs(...)` and keep command-specific `oldfields=%s` formatting local
  - update: expose `Context()` and accurate event `PLogOffset()` on `cmdWorkpiece` and keep `appPartition` available during recovery so sync actualizers can use the same logging flow

- [x] update: [pkg/processors/command/provide.go](../../../pkg/processors/command/provide.go)
  - update: add a `setPLogOffset` pipeline step before raw-event building so command logging and sync actualizers use the same reserved PLog offset

- [x] update: [pkg/processors/command/types.go](../../../pkg/processors/command/types.go)
  - update: extend `cmdWorkpiece` with `pLogOffset` so the accurate event offset can be carried through command logging and sync actualizer execution

- [x] update: [pkg/processors/command/impl_test.go](../../../pkg/processors/command/impl_test.go)
  - update: assert shared per-CUD logging includes both `newfields=` and command-specific `oldfields=`

- [x] update: [pkg/processors/actualizers/types.go](../../../pkg/processors/actualizers/types.go)
  - update: make `ProjectorEvent(...)` return the triggering `QName` instead of `bool` so actualizer logging can distinguish execute, execute-with-param, and CUD-triggered events
  - add: `errWithCtx` for propagating failure logs with the enriched log context

- [x] update: [pkg/processors/actualizers/async.go](../../../pkg/processors/actualizers/async.go)
  - update: route failures through context-aware error logging and replace n10n trace logging with verbose loggerctx logging
  - update: `asyncProjector.DoAsync` — enrich the base log context with `wsid`, log the triggering projector, log event/CUDs before `Invoke`, and log `success` on success
  - update: `logEventAndCUDs(logCtx, event, pLogOffset, appDef, triggeredByQName)` — use the already resolved triggering `QName`, log all CUDs for function-triggered and object-document/object-record-triggered events, and otherwise log only CUDs whose `QName` matches it

- [x] update: [pkg/processors/actualizers/interface.go](../../../pkg/processors/actualizers/interface.go)
  - update: remove configurable async actualizer `LogError` hook so failures always flow through the loggerctx-based error handling in `async.go`

- [x] update: [pkg/processors/actualizers/impl.go](../../../pkg/processors/actualizers/impl.go)
  - update: make sync actualizers use the same shared event/CUD logging flow before projector invocation

- [x] update: [pkg/processors/actualizers/async_test.go](../../../pkg/processors/actualizers/async_test.go)
  - update: cover execute-projector logging, record-projector filtering, and context-aware failure logging

- [x] update: [pkg/processors/actualizers/types_test.go](../../../pkg/processors/actualizers/types_test.go)
  - update: assert `ProjectorEvent(...)` against triggering `QName` results instead of boolean trigger flags

- [x] update: [pkg/processors/actualizers/impl_helpers_test.go](../../../pkg/processors/actualizers/impl_helpers_test.go)
  - update: extend `cmdWorkpieceMock` with `Context()` and `PLogOffset()` so sync actualizer helper tests match the new command workpiece logging contract
