# Implementation plan: Design logging subsystem architecture

## Construction

### Core logging infrastructure

- [x] update: [pkg/goutils/logger/consts.go](../../../pkg/goutils/logger/consts.go)
  - add: `LogAttr_Stage = "stage"` constant

- [x] update: [pkg/goutils/logger/loggerctx.go](../../../pkg/goutils/logger/loggerctx.go)
  - update: Add `stage string` parameter to `VerboseCtx`, `ErrorCtx`, `InfoCtx`, `WarningCtx`, `TraceCtx`
  - update: Add `stage string` parameter to `LogCtx` (after `level`)
  - update: `logCtx` internal to accept `stage string` and append it as `LogAttr_Stage` slog attribute

- [x] update: [pkg/goutils/logger/logger_test.go](../../../pkg/goutils/logger/logger_test.go)
  - update: All `*Ctx` call sites to include `stage` parameter
  - add: Test that `stage` attribute appears in log output

- [x] run: Logger tests
  - `go test -short ./pkg/goutils/logger/...`

- [x] Review

### Shared utilities & consts

- [x] update: [pkg/processors/utils.go](../../../pkg/processors/utils.go)
  - update: Add `stage string` and `skipStackFrames int` parameters to `LogEventAndCUDs`
  - update: Add `perCUDLogCallback func(istructs.ICUDRow) (bool, string, error)` and `eventMessageAdds string` parameters
  - update: Enrich context with `woffset`, `poffset`, `evqname` attributes
  - update: Log event arguments as JSON at Verbose level with stage `<stage>`, msg `args={...}{eventMessageAdds}`
  - update: For each CUD: call `perCUDLogCallback`; if `shouldLog`, enrich context with `rectype`, `recid`, `op`; log at Verbose with stage `<stage>.log_cud`, msg `newfields={...}{msgAdds}`
  - update: Return the enriched context

- [x] add: `sys.VApp_SysVoedger = "sys/voedger"` constant in [pkg/sys/const.go](../../../pkg/sys/const.go)

- [x] Review

### HTTP server

- [x] update: [pkg/router/impl_http.go](../../../pkg/router/impl_http.go)
  - update: `preRun` to add `extension` attrib (from `httpServer.name` field: `sys._HTTPServer`, `sys._AdminHTTPServer`, `sys._HTTPSServer`, or `sys._ACMEServer`) via `logger.WithContextAttrs`; use `sys.VApp_SysVoedger` for `vapp` attrib
  - update: `httpServer.log` removed — replaced with direct `logger.*Ctx` calls with appropriate stage and level
  - update: `preRun` log: level `Info`, stage `endpoint.listen.start`, msg `<addr>:<port>`
  - update: `Run` on unexpected Serve() error: level `Error`, stage `endpoint.unexpectedstop`, msg `Serve() error: <err>`
  - update: `httpsService.Run` on unexpected ServeTLS() error: level `Error`, stage `endpoint.unexpectedstop`, msg `ServeTLS() error: <err>`
  - update: `Stop` on Shutdown() failure: level `Error`, stage `endpoint.shutdown.error`, msg `<error message>`
  - add: On successful shutdown: level `Info`, stage `endpoint.shutdown`, msg (empty)

- [x] update: [pkg/router/impl_acme.go](../../../pkg/router/impl_acme.go)
  - update: ACME server logging handled via `name` field on `httpServer` set to `sys._ACMEServer` in `provide.go`; `acmeService` inherits `Run`/`Stop` from `httpServer`

- [x] update: [pkg/router/provide.go](../../../pkg/router/provide.go)
  - remove: `httpServ.name = "HTTPS server"` assignment (`name` field now set to `"sys._HTTPSServer"` via `getHTTPServer`)
  - update: call sites pass extension strings (`sys._HTTPServer`, etc.) as the `name` argument

- [x] Review

### Bootstrap

- [x] update: [pkg/btstrp/impl.go](../../../pkg/btstrp/impl.go)
  - add: Create log context with `vapp=sys.VApp_SysVoedger`, `extension="sys._Bootstrap"` using `logger.WithContextAttrs`
  - add: Bootstrap starts: level `Info`, stage `bootstrap`, msg `started`
  - update: logging cluster app workspace initied already, cluster app workspace init, and app deploys: level `Info`, stage `bootstrap`
  - add: For each built-in app: level `Info`, stage `bootstrap.appdeploy.builtin`, msg `<appQName>`
  - add: For each sidecar app: level `Info`, stage `bootstrap.appdeploy.sidecar`, msg `<appQName>`
  - add: For each built-in app partition: level `Info`, stage `bootstrap.apppartdeploy.builtin`, msg `<appQName>/<partID>`
  - add: For each sidecar app partition: level `Info`, stage `bootstrap.apppartdeploy.sidecar`, msg `<appQName>/<partID>`
  - add: Bootstrap completes: level `Info`, stage `bootstrap`, msg `completed`

- [x] Review

### Leadership

- [ ] update: [pkg/ielections/impl.go](../../../pkg/ielections/impl.go)
  - update: `AcquireLeadership` and `maintainLeadership` to accept/create a context with `vapp=sys.VApp_SysVoedger`, `extension="sys._Leadership"`, `key` attribs
  - update: Replace `logger.Verbose(fmt.Sprintf("Key=%v: leadership already acquired..."` with `logger.InfoCtx(ctx, "leadership.acquire.other", "leadership already acquired by someone else")`
  - update: Replace `logger.Error(fmt.Sprintf("Key=%v: InsertIfNotExist failed..."` with `logger.ErrorCtx(ctx, "leadership.acquire.error", "InsertIfNotExist failed:", err)`
  - update: Replace `logger.Info(fmt.Sprintf("Key=%v: leadership acquired"` with `logger.InfoCtx(ctx, "leadership.acquire.success", "success")`
  - update: First 10 renewal ticks: `logger.VerboseCtx(ctx, "leadership.maintain.10", "renewing leadership")`
  - update: Every 200 ticks: `logger.VerboseCtx(ctx, "leadership.maintain.200", "still leader for", duration)`
  - update: On compareAndSwap error: `logger.ErrorCtx(ctx, "leadership.maintain.stgerror", "compareAndSwap error:", err)`
  - update: On leadership stolen: `logger.ErrorCtx(ctx, "leadership.maintain.stolen", "compareAndSwap !ok => release")`
  - update: On retry deadline reached: `logger.ErrorCtx(ctx, "leadership.maintain.release", "retry deadline reached, releasing. Last error:", err)`
  - add: On error after processKillThreshold: `logger.ErrorCtx(ctx, "leadership.maintain.terminating", "the process is still alive after the time alloted for graceful shutdown -> terminating...")`
  - update: Drop all other logging not described in TD

- [ ] Review

### Router

- [ ] update: [pkg/router/utils.go](../../../pkg/router/utils.go)
  - update: `logServeRequest` to use stage `routing.accepted` and pass `stage` parameter
  - update: `withLogAttribs` — verify `reqid` format is `{MMDDHHmm}-{atomicCounter}`

- [ ] update: [pkg/router/impl_http.go](../../../pkg/router/impl_http.go)
  - update: `serveRequest` — on `SendRequest` error: stage `routing.send2vvm.error`
  - update: `reply_v1` — on response write error: stage `routing.response.error`
  - add: Log first response latency: level `Verbose`, stage `routing.latency1`, msg `<latency_ms>`

- [ ] update: [pkg/router/impl_apiv2.go](../../../pkg/router/impl_apiv2.go)
  - update: `sendRequestAndReadResponse` — on `SendRequest` error: stage `routing.send2vvm.error`
  - update: Response writing errors: stage `routing.response.error`
  - add: Log first response latency: level `Verbose`, stage `routing.latency1`, msg `<latency_ms>`

- [ ] update: [pkg/router/impl_validation.go](../../../pkg/router/impl_validation.go)
  - update: `withValidate` — validation failure log: replace `logger.Error` with `logger.ErrorCtx`, stage `endpoint.validation`, msg `<error message>`

- [ ] update: [pkg/router/impl_reverseproxy.go](../../../pkg/router/impl_reverseproxy.go)
  - drop: `logger.Info("reverse proxy route registered: "...)` (L33)
  - drop: `logger.Info("default route registered: "...)` (L57)
  - drop: `logger.Verbose(fmt.Sprintf("reverse proxy: incoming %s..."...))` (L128-130)

- [ ] Review

### Command processor

- [ ] update: [pkg/processors/command/impl.go](../../../pkg/processors/command/impl.go)
  - update: `logEventAndCUDs` to pass stage `cp.plog_saved` to `processors.LogEventAndCUDs`; per-CUD callback returns `shouldLog=true`, `msgAdds=",oldfields={...}"` for HTTP CUDs, empty for command-created CUDs; `eventMessageAdds` is empty
  - update: `recovery` to create context with `vapp=sys.VApp_SysVoedger`, `extension="sys._Recovery"`, `partid` attrib
  - update: Recovery start: level `Info`, stage `cp.partition_recovery.start`, msg (empty)
  - update: Recovery complete: level `Info`, stage `cp.partition_recovery.complete`, msg `completed, nextPLogOffset and workspaces JSON`
  - add: Recovery failure: level `Error`, stage `cp.partition_recovery.error`, msg `<error message>`
  - keep: `logger.VerboseCtx(..."newACL not ok, but oldACL ok..."...)` (2 locations: `checkExecPermissions` and CUD ACL check)
  - drop: `logger.VerboseCtx(..."async actualizers are notified..."...)` in `notifyAsyncActualizers`
  - replace logging "failed to marhsal response" with `panic("failed to marhsal response: <err>")` in `sendResponse`

- [ ] update: [pkg/processors/command/provide.go](../../../pkg/processors/command/provide.go)
  - update: `logHandlingError` — level `Error`, stage `cp.error`, msg `<error message>`, `body=<compacted request body>`
  - update: `logSuccess` — level `Verbose`, stage `cp.success`, msg `<command result>`
  - update: Partition restart warning — stage `cp.partition_recovery`, level `Warning`, with `vapp` replaced with `sys.VApp_SysVoedger`, `extension` replaced with `sys._Recovery`

- [ ] Review

### Query processor

- [ ] update: [pkg/processors/query/impl.go](../../../pkg/processors/query/impl.go)
  - update: Query execution error: level `Error`, stage `qp.error`, msg `<error message>` (replace current `logger.Error(fmt.Sprintf(...))` with `logger.ErrorCtx`)
  - keep: `logger.Verbose("newACL not ok, but oldACL ok."...)` in ACL check

- [ ] update: [pkg/processors/query2/impl.go](../../../pkg/processors/query2/impl.go)
  - update: Query execution error: level `Error`, stage `qp.error`, msg `<error message>` (replace current `logger.Error(fmt.Sprintf(...))` with `logger.ErrorCtx`)
  - drop: `logger.Error(fmt.Sprintf("failed to send the error %s: %s"...))` in error response sending

- [ ] update: [pkg/processors/query/operator-send-to-bus-impl.go](../../../pkg/processors/query/operator-send-to-bus-impl.go)
  - drop: `logger.Error("failed to send error from rowsProcessor to QP: "...)` in `OnError`

- [ ] Review

### Sync projectors

- [ ] update: [pkg/processors/actualizers/impl.go](../../../pkg/processors/actualizers/impl.go)
  - update: `newSyncBranch` — log trigger QName before `Invoke()`: level `Verbose`, stage `sp.triggeredby`, msg `<triggered by qname>`, `extension=<projector QName>`
  - add: After success Invoke: level `Verbose`, stage `sp.success`, `extension=sp.<projector QName>`, msg (empty)

- [ ] update: [pkg/processors/command/provide.go](../../../pkg/processors/command/provide.go)
  - add: After all sync projectors success: level `Verbose`, stage `sp.success`, msg (empty)
  - add: On sync projector error: level `Error`, stage `sp.error`, msg `<error message>`

- [ ] Review

### Async projectors

- [ ] update: [pkg/processors/actualizers/async.go](../../../pkg/processors/actualizers/async.go)
  - update: `DoAsync` — `logEventAndCUDs` call to pass stage `ap`; `perCUDCallback` returns correct `shouldLog` based on trigger type; `eventMessageAdds` is `triggeredby=<QName>`
  - update: `asyncErrorHandler.OnError` — log using event context: level `Error`, stage `ap.error`, msg `<error message>`
  - update: `logError` — use stage `ap.error` in `ErrorCtx` calls
  - drop: `logger.ErrorCtx(..."readOffset..."...)` in `DoAsync` init
  - drop: `logger.VerboseCtx(..."notified..."...)` in notification handler
  - drop: `logger.ErrorCtx(..."readPlogToOffset..."...)` in plog read error
  - drop: `logger.VerboseCtx(..."success..."...)` existing success log (replaced by TD-defined `ap.success`)

- [ ] Review

### Blob processor

- [ ] update: [pkg/processors/blobber/impl_write.go](../../../pkg/processors/blobber/impl_write.go)
  - add: After `validateQueryParams` success: level `Verbose`, stage `bp.meta`, msg `name=<name>,contenttype=<type>`, using `VerboseCtx`
  - add: After `registerBLOB`: level `Verbose`, stage `bp.register.success`, msg (empty)
  - add: After `blobStorage.WriteBLOB()` — add `blobid` attrib, then: level `Verbose`, stage `bp.write.success`, msg (empty)
  - add: After `setBLOBStatusCompleted`: level `Verbose`, stage `bp.setcompleted.success`, msg (empty)
  - update: `sendWriteResult.OnErr` — level `Error`, stage `bp.error`, msg `<error message>` with query and headers for 400 errors
  - drop: `logger.Verbose("blob write success:...")` and `logger.Verbose("blob write error:...")` calls
  - drop: `logger.Error("failed to send successfult BLOB write repply:...")`
  - add: Local constants for `ownerqname`, `ownerfield`, `ownerid`, `blobid` attribute keys

- [ ] update: [pkg/processors/blobber/impl_read.go](../../../pkg/processors/blobber/impl_read.go)
  - add: Read success: level `Verbose`, stage `bp.success`, msg (empty)
  - update: `catchReadError.DoSync` — level `Error`, stage `bp.error`, msg `<error message>` with query and headers for 400 errors
  - drop: `logger.Verbose("blob read error:...")` call
  - drop: `logger.Error(fmt.Sprintf("failed to read BLOB:..."...))` in `readBLOB`
  - add: Add `blobid` attrib to context at start of processing

- [ ] update: [pkg/processors/blobber/impl_requesthandler.go](../../../pkg/processors/blobber/impl_requesthandler.go)
  - add: Set `ownerqname`, `ownerfield`, `ownerid` attribs in context (if applicable)

- [ ] Review

### N10N processor

- [ ] update: [pkg/router/impl_n10n.go](../../../pkg/router/impl_n10n.go)
  - update: `subscribeAndWatchHandler` — call `withLogAttribs()` with `extension="sys._N10N_SubscribeAndWatch"`
  - update: `subscribeHandler` — call `withLogAttribs()` with `extension=entity QName`
  - update: `unSubscribeHandler` — call `withLogAttribs()` with `extension=entity QName`
  - update: Drop all existing `logger.Info/Error` calls not described in TD

- [ ] update: [pkg/processors/n10n/impl.go](../../../pkg/processors/n10n/impl.go)
  - update: `reportError` — level `Error`, stage `n10n.error`, msg `<error message>` with context; for 400 errors append body or projectionkey

- [ ] update: [pkg/processors/n10n/impl_subscribeandwatch.go](../../../pkg/processors/n10n/impl_subscribeandwatch.go)
  - add: `projectionkey` attrib if single projection key
  - add: `channelid` attrib after channel creation
  - add: After successful subscribe+watch: level `Verbose`, stage `n10n.subscribe&watch.success`; single key → empty msg; otherwise → msg `subscriptions=<subscriptions>`
  - update: `watchChannel` — log each SSE message: level `Verbose`, stage `n10n.sse_sent`, msg `<sse message>`
  - update: SSE send error: level `Error`, stage `n10n.watch.sse_error`, msg `<error>`
  - add: Local constants for `channelid`, `projectionkey` attribute keys

- [ ] update: [pkg/processors/n10n/impl_subscribeextra.go](../../../pkg/processors/n10n/impl_subscribeextra.go)
  - add: `projectionkey` attrib to context
  - add: After successful subscribe: level `Verbose`, stage `n10n.subscribe.success`

- [ ] update: [pkg/processors/n10n/impl_unsubscribe.go](../../../pkg/processors/n10n/impl_unsubscribe.go)
  - add: `projectionkey` attrib to context
  - add: After successful unsubscribe: level `Verbose`, stage `n10n.unsubscribe.success`

- [ ] update: [pkg/processors/consts.go](../../../pkg/processors/consts.go)
  - add: `APIPath_N10N_SubscribeAndWatch` constant

- [ ] update: [pkg/router/utils.go](../../../pkg/router/utils.go)
  - update: `apiPathToExtension()` — add case for `APIPath_N10N_SubscribeAndWatch` returning `"sys._N10N_SubscribeAndWatch"`

- [ ] Review

### N10N broker lifecycle

- [ ] update: [pkg/in10nmem/impl.go](../../../pkg/in10nmem/impl.go)
  - add: Create log context in `NewN10nBroker` with `vapp=sys.VApp_SysVoedger`, `extension="sys._N10NBroker"`
  - update: `notifier` start: level `Info`, stage `n10n.notifier.start`, msg (empty)
  - update: `notifier` stop: level `Info`, stage `n10n.notifier.stop`, msg (empty)
  - update: `heartbeat30` start: level `Info`, stage `n10n.heartbeat.start`, msg `Heartbeat30Duration: <duration>`
  - update: `heartbeat30` stop: level `Info`, stage `n10n.heartbeat.stop`, msg (empty)
  - add: Channel expired during `WatchChannel`: level `Error`, stage `n10n.channel.expired`, msg `<subjectLogin>`
  - add: Channel cleanup unsubscribe error: level `Error`, stage `n10n.cleanup.error`, msg `channelID=<id>, projectionKey=<key>: <error>`
  - drop: All `logger.Trace(...)` calls not described in TD (notifier loop, heartbeat loop, WatchChannel, channel management)

- [ ] update: [pkg/in10nmem/provide.go](../../../pkg/in10nmem/provide.go)
  - update: Pass log context through `NewN10nBroker` if needed

- [ ] Review

### Schedulers

- [ ] update: [pkg/processors/schedulers/impl_scheduler.go](../../../pkg/processors/schedulers/impl_scheduler.go)
  - update: `keepRunning` — log (re)schedule: level `Verbose`, stage `job.schedule`, msg `now=<timeNow>,next=<nextRunTime>`
  - update: `keepRunning` — log wake-up: level `Verbose`, stage `job.wake-up`, msg `<timeNow>`
  - update: `runJob` — log successful invoke: level `Verbose`, stage `job.success`, msg (empty)
  - drop: `logger.Info(a.name, "schedule"...)` and `logger.Info(a.name, "wake"...)` calls
  - drop: `logger.Trace(a.name, "started")` in `init`
  - drop: `logger.Trace(a.name + "s finalized")` in `finit`
  - drop: `logger.Verbose("invoked " + a.name)` in `runJob` (replaced by TD-defined `job.success`)
  - drop: `LogError` callback usage (`a.conf.LogError(...)`) in `Prepare` and `runJob` — not described in TD
  - update: Use `*Ctx` functions with context containing `vapp` and `extension=job.<job QName>` attribs

- [ ] Review
