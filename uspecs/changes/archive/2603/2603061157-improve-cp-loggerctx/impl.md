# Implementation plan: Improve loggerctx in command processor

## Construction

- [x] update: [pkg/goutils/logger/consts.go](../../../pkg/goutils/logger/consts.go)
  - rename: `LogAttr_App` to `LogAttr_VApp` and value from `"app"` to `"vapp"`
- [x] update: [pkg/goutils/logger/README.md](../../../pkg/goutils/logger/README.md)
  - rename: `LogAttr_App` to `LogAttr_VApp` and value from `"app"` to `"vapp"` in the constants table
- [x] update: [pkg/goutils/logger/logger_test.go](../../../pkg/goutils/logger/logger_test.go)
  - update: all `LogAttr_App` references to `LogAttr_VApp`; update `"app="` string assertions to `"vapp="`

- [x] update: [pkg/router/impl_http.go](../../../pkg/router/impl_http.go)
  - update: `preRun` — use `LogAttr_VApp` instead of `LogAttr_App`
- [x] update: [pkg/router/utils.go](../../../pkg/router/utils.go)
  - update: `withLogAttribs` — use `LogAttr_VApp` instead of `LogAttr_App`
  - update: `logServeRequest` — log message changed to `"request accepted"`

- [x] update: [pkg/processors/command/impl.go](../../../pkg/processors/command/impl.go)
  - update: `logEventAndCUDs` — add `"evqname"` (from `cmd.pLogEvent.QName()`) to event-level context
  - update: `logEventAndCUDs` — iterate `cmd.pLogEvent.CUDs` instead of `cmd.parsedCUDs` (actual IDs, not raw IDs)
  - update: `logEventAndCUDs` — build `oldRecs` map from `parsedCUDs.existingRecord`; log `newfields` and `oldfields` per CUD with per-CUD context (`rectype`, `recid`, `op`)
  - add: helper `cudOp(cud istructs.ICUDRow) string` — returns `"create"`, `"update"`, `"activate"`, or `"deactivate"`
