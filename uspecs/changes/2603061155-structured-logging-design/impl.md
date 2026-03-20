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

- [x] update: [pkg/ielections/impl.go](../../../pkg/ielections/impl.go)
  - add: `leadershipLogCtx[K any](key K) context.Context` — helper that builds log context with `vapp=sys.VApp_SysVoedger`, `extension="sys._Leadership"`, `key` attribs
  - update: `AcquireLeadership` — use `leadershipLogCtx(key)` instead of inline map literal; on `isFinalized`: `logger.WarningCtx(logCtx, "leadership.acquire.finalized", "elections cleaned up; cannot acquire leadership")`
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
  - add: `releaseLeadership(ctx, key)` — add `ctx context.Context` param; when key not found: `logger.WarningCtx(ctx, "leadership.release.notleader", "we're not the leader")`
  - add: `releaseLeadership` — on success before cancel: `logger.InfoCtx(li.ctx, "leadership.released", "")`
  - update: `ReleaseLeadership` — use `leadershipLogCtx(key)` and pass it to `releaseLeadership`
  - update: `renewWithRetry` — pass its existing `ctx` to `releaseLeadership`

- [x] Review

### Router

- [x] update: [pkg/router/utils.go](../../../pkg/router/utils.go)
  - update: `logServeRequest` to use stage `routing.accepted` with empty msg
  - verified: `withLogAttribs` reqid format is `{MMDDHHmm}-{atomicCounter}`

- [x] update: [pkg/router/impl_http.go](../../../pkg/router/impl_http.go)
  - update: `RequestHandler_V1` — on `SendRequest` error: stage `routing.send2vvm.error`
  - add: call `logLatency(requestCtx, sentAt)` immediately after `SendRequest` returns successfully

- [x] update: [pkg/router/impl_reply_v1.go](../../../pkg/router/impl_reply_v1.go)
  - add: on response write error: `logger.ErrorCtx` with stage `routing.response.error`

- [x] update: [pkg/router/impl_apiv2.go](../../../pkg/router/impl_apiv2.go)
  - update: `sendRequestAndReadResponse` — on `SendRequest` error: stage `routing.send2vvm.error`
  - add: call `logLatency(requestCtx, sentAt)` immediately after `SendRequest` returns successfully

- [x] update: [pkg/router/impl_reply_v2.go](../../../pkg/router/impl_reply_v2.go)
  - add: on response write error: `logger.ErrorCtx` with stage `routing.response.error`

- [x] update: [pkg/router/impl_validation.go](../../../pkg/router/impl_validation.go)
  - update: `withValidate` — validation failure log: replace `logger.Error` with `logger.ErrorCtx`, stage `routing.validation`

- [x] update: [pkg/router/impl_reverseproxy.go](../../../pkg/router/impl_reverseproxy.go)
  - drop: `logger.Info("reverse proxy route registered: "...)`
  - drop: `logger.Info("default route registered: "...)`
  - drop: `logger.Verbose(fmt.Sprintf("reverse proxy: incoming %s..."...))` and unused variables `srcURL`, `srcHost`

- [x] Review

### Command processor

- [x] update: [pkg/processors/command/impl.go](../../../pkg/processors/command/impl.go)
  - update: `logEventAndCUDs` to pass stage `cp.plog_saved` to `processors.LogEventAndCUDs`; per-CUD callback returns `shouldLog=true`, `msgAdds=",oldfields={...}"` for HTTP CUDs, empty for command-created CUDs; `eventMessageAdds` is empty
  - update: `recovery` to create context with `vapp=sys.VApp_SysVoedger`, `extension="sys._Recovery"`, `partid` attrib
  - update: Recovery start: level `Info`, stage `cp.partition_recovery.start`, msg (empty)
  - update: Recovery complete: level `Info`, stage `cp.partition_recovery.complete`, msg `completed, nextPLogOffset and workspaces JSON`
  - add: ReadPLog failure: level `Error`, stage `cp.partition_recovery.readplog.error`, msg `<error message>`
  - add: Last event re-apply: call `processors.LogEventAndCUDs()` with stage `cp.partition_recovery.reapply` to log which event is being re-applied; result saved to `cmd.logCtx`
  - add: `LogEventAndCUDs` failure during re-apply: level `Error`, stage `cp.partition_recovery.logeventandcuds.error`, msg `<error message>`
  - add: StoreOp failure: level `Error`, stage `cp.partition_recovery.storeop.error`, msg `<error message>`
  - keep: `logger.VerboseCtx(..."newACL not ok, but oldACL ok..."...)` (2 locations: `checkExecPermissions` and CUD ACL check)
  - drop: `logger.VerboseCtx(..."async actualizers are notified..."...)` in `notifyAsyncActualizers`
  - replace logging "failed to marhsal response" with `panic("failed to marhsal response: <err>")` in `sendResponse`

- [x] update: [pkg/processors/command/provide.go](../../../pkg/processors/command/provide.go)
  - update: `logHandlingError` — level `Error`, stage `cp.error`, msg `<error message>`, `body=<compacted request body>`
  - update: `logSuccess` — level `Verbose`, stage `cp.success`, msg `<command result>`
  - update: Partition restart warning — stage `cp.partition_recovery`, level `Warning`, with `vapp` replaced with `sys.VApp_SysVoedger`, `extension` replaced with `sys._Recovery`

- [x] update: [pkg/processors/command/impl_test.go](../../../pkg/processors/command/impl_test.go)
  - update: `TestLogEventAndCUDs` — assert `stage=cp.plog_saved` in captured log output
  - update: `TestBasicUsage` — capture logs via `syncBuffer`; assert `stage=cp.success` on success, `stage=cp.error` + error message on failure
  - update: `TestRecovery` — capture logs after restart; assert `stage=cp.partition_recovery.start`, `stage=cp.partition_recovery.complete`, `vapp=sys/voedger`, `extension=sys._Recovery`, `partid=`
  - update: `TestRecoveryOnSyncProjectorError` — capture logs after sync projector failure; assert `stage=cp.partition_recovery`, `vapp=sys/voedger`, `extension=sys._Recovery`, `"partition will be restarted"`
  - add: `TestSyncProjectorLogging` — two subtests: success (assert `stage=sp.triggeredby`, `stage=sp.success`, `extension=<projQName>`) and error (assert `stage=sp.triggeredby`, `stage=sp.error`, error message, `extension=<projQName>`)

- [x] Review

### Query processor

- [x] update: [pkg/processors/query/impl.go](../../../pkg/processors/query/impl.go)
  - update: Query execution error: level `Error`, stage `qp.error`, msg `<error message>` (replace current `logger.Error(fmt.Sprintf(...))` with `logger.ErrorCtx`)
  - add: Query execution success: level `Verbose`, stage `qp.success`, msg (empty) — logged right after `execQuery` returns `nil`
  - keep: `logger.Verbose("newACL not ok, but oldACL ok."...)` in ACL check

- [x] update: [pkg/processors/query2/impl.go](../../../pkg/processors/query2/impl.go)
  - update: Query execution error: level `Error`, stage `qp.error`, msg `<error message>` (replace current `logger.Error(fmt.Sprintf(...))` with `logger.ErrorCtx`)
  - add: Query execution success: level `Verbose`, stage `qp.success`, msg (empty) — logged right after `exec` handler returns `nil`
  - drop: `logger.Error(fmt.Sprintf("failed to send the error %s: %s"...))` in error response sending

- [x] update: [pkg/processors/query/operator-send-to-bus-impl.go](../../../pkg/processors/query/operator-send-to-bus-impl.go)
  - drop: `logger.Error("failed to send error from rowsProcessor to QP: "...)` in `OnError`

- [x] update: [pkg/processors/query/impl_test.go](../../../pkg/processors/query/impl_test.go)
  - update: `TestRateLimiter` — capture logs via `syncBuffer` at `LogLevelVerbose`; assert `stage=qp.success` and no `qp.error` on successful requests; assert `stage=qp.error` and no `qp.success` on rate-limited request

- [x] Review

### Sync projectors

- [x] update: [pkg/processors/actualizers/impl.go](../../../pkg/processors/actualizers/impl.go)
  - update: `newSyncBranch` — no per-projector `logEventAndCUDs` call; use `work.LogCtxForSyncProjector()` enriched with `extension=<projector QName>` for all logs
  - add: Right before `Invoke()`: level `Verbose`, stage `sp.triggeredby`, msg `<triggered by qname>`, `extension=<projector QName>`
  - add: After successful `Invoke()`: level `Verbose`, stage `sp.success`, `extension=<projector QName>`, msg (empty)
  - add: On `Invoke()` failure: level `Error`, stage `sp.error`, `extension=<projector QName>`, msg `<error message>`

- [x] update: [pkg/processors/command/provide.go](../../../pkg/processors/command/provide.go)
  - add: After all sync projectors success: level `Verbose`, stage `sp.success`, msg (empty), using `cmd.logCtx` from `logEventAndCUDs`
  - add: On sync projector error: level `Error`, stage `sp.error`, msg `<error message>`, using `cmd.logCtx`

- [x] update: [pkg/processors/command/types.go](../../../pkg/processors/command/types.go)
  - add: `logCtx context.Context` field to `cmdWorkpiece` to carry enriched log context to sync projectors

- [x] update: [pkg/processors/command/impl.go](../../../pkg/processors/command/impl.go)
  - update: `logEventAndCUDs` — save `enrichedLogCtx` to `cmd.logCtx` instead of discarding it

- [x] Review

### Async projectors

- [x] update: [pkg/processors/actualizers/async.go](../../../pkg/processors/actualizers/async.go)
  - update: `DoAsync` — enriches `w.logCtx` with `wsid`; calls `logEventAndCUDs` with stage `"ap"` storing result in `w.logCtx`
  - add: After successful `Invoke()`: level `Verbose`, stage `ap.success`, msg (empty), using `w.logCtx`
  - drop: `asyncErrorHandler.OnError` logging (error logging moved to `retrierCfg.OnError`)
  - drop: `logger.ErrorCtx(..."readOffset..."...)` in init
  - drop: `logger.VerboseCtx(..."notified..."...)` in notification handler
  - drop: `logger.ErrorCtx(..."readPlogToOffset..."...)` in plog read error
  - drop: `logger.ErrorCtx` in `handleEvent` (errors propagate to retrier)
  - update: `readPlogByBatches`, `readPlogToTheEnd`, `readPlogToOffset`, `keepReading`, `handleEvent` — pass context through to store in workpiece

- [x] update: [pkg/processors/actualizers/async_test.go](../../../pkg/processors/actualizers/async_test.go)
  - update: `Test_AsynchronousActualizer_Logs/execute projector` — assert `stage=ap` and `stage=ap.success`
  - update: `Test_AsynchronousActualizer_Logs/record projector` — assert `stage=ap` and `stage=ap.success`
  - update: `Test_AsynchronousActualizer_ErrorAndRestore` — assert `stage=ap.error`

- [x] Review

### Blob processor

- [x] update: [pkg/processors/blobber/impl_write.go](../../../pkg/processors/blobber/impl_write.go)
  - add: `logCtx` field to `blobWorkpiece` in `types.go` to carry enriched logging context through the pipeline
  - add: `getBLOBMessageWrite` initializes `bw.logCtx` from `requestCtx`; if `ownerRecord != NullQName` enriches with `ownerqname` and `ownerfield` attribs
  - add: After `validateQueryParams` success: level `Verbose`, stage `bp.meta`, msg `name=<name>,contenttype=<type>`, using `bw.logCtx`
  - add: After `registerBLOB`: enrich `bw.logCtx` with `blobid` attrib, then level `Verbose`, stage `bp.register.success`
  - add: After `blobStorage.WriteBLOB()`: level `Verbose`, stage `bp.write.success`, using `bw.logCtx` (already has `ownerqname`, `ownerfield`, `blobid`)
  - add: After `setBLOBStatusCompleted`: level `Verbose`, stage `bp.setcompleted.success`, using `bw.logCtx`
  - update: `sendWriteResult.OnErr` — level `Error`, stage `bp.error`, using `bw.logCtx`, with query and headers for 400 errors
  - drop: `logger.Verbose("blob write success:...")` and `logger.Verbose("blob write error:...")` calls
  - drop: `logger.Error("failed to send successfult BLOB write repply:...")`
  - add: Local constants for `ownerqname`, `ownerfield`, `ownerid`, `blobid` attribute keys

- [x] update: [pkg/processors/blobber/impl_read.go](../../../pkg/processors/blobber/impl_read.go)
  - add: `getBLOBMessageRead` initializes `bw.logCtx` from `requestCtx`; if `ownerRecord != NullQName` enriches with `ownerqname`, `ownerfield`, `ownerid` attribs
  - add: `getBLOBIDFromOwner` enriches `bw.logCtx` with `blobid` as soon as the ID is determined — for non-APIv2/temp (early return): from existing `existingBLOBIDOrSUUID`; for APIv2 persistent: from the resolved `blobID` before returning
  - add: Read success: level `Verbose`, stage `bp.success`, msg (empty), using `bw.logCtx`
  - update: `catchReadError.DoSync` — level `Error`, stage `bp.error`, msg `<error message>` with query and headers for 400 errors, using `bw.logCtx`
  - drop: `logger.Verbose("blob read error:...")` call
  - drop: `logger.Error(fmt.Sprintf("failed to read BLOB:..."...))` in `readBLOB`

- [x] add tests: [pkg/processors/blobber/impl_write_test.go](../../../pkg/processors/blobber/impl_write_test.go)
  - subtest `bp.meta on write success` — on successful write, assert `stage=bp.meta` and msg contains `name=` and `contenttype=`
  - subtest `bp.register.success on write success` — assert `stage=bp.register.success` and `blobid=`
  - subtest `bp.write.success on write success` — assert `stage=bp.write.success` and `blobid=`
  - subtest `bp.setcompleted.success on write success` — assert `stage=bp.setcompleted.success` and `blobid=`
  - subtest `bp.error on write error` — assert `stage=bp.error` and `<error message>`
  - note: for owner-based writes also assert `ownerqname=`, `ownerfield=` on all stages

- [x] add tests: [pkg/processors/blobber/impl_read_test.go](../../../pkg/processors/blobber/impl_read_test.go)
  - subtest `bp.success on read success` — on successful read, assert `stage=bp.success` and `blobid=`
  - subtest `bp.error on read error` — assert `stage=bp.error` and `<error message>` and `blobid=`
  - note: for owner-based reads also assert `ownerqname=`, `ownerfield=`, `ownerid=` on all stages

- [x] Review

### N10N processor

- [x] update: [pkg/router/impl_n10n.go](../../../pkg/router/impl_n10n.go)
  - update: `subscribeAndWatchHandler` — call `withLogAttribs()` with `extension="sys._N10N_SubscribeAndWatch"`
  - update: `subscribeHandler` — call `withLogAttribs()` with `extension=entity QName`
  - update: `unSubscribeHandler` — call `withLogAttribs()` with `extension=entity QName`
  - update: Drop all existing `logger.Info/Error` calls not described in TD

- [x] update: [pkg/processors/n10n/impl.go](../../../pkg/processors/n10n/impl.go)
  - update: `reportError` — level `Error`, stage `n10n.error`, msg `<error message>` with context; for 400 errors append body or projectionkey

- [x] update: [pkg/in10n/impl.go](../../../pkg/in10n/impl.go)
  - [x] add: `ProjectionKeysToJSON(keys []ProjectionKey) string` — serializes a slice of keys as a JSON array

- [x] update: [pkg/processors/n10n/impl_subscribeandwatch.go](../../../pkg/processors/n10n/impl_subscribeandwatch.go)
  - [x] add: `projectionkey` attrib (JSON array via `ProjectionKeysToJSON`) whenever at least one key is subscribed
  - [x] add: `channelid` attrib after channel creation
  - [x] add: After successful subscribe+watch: level `Verbose`, stage `n10n.subscribe&watch.success`, msg empty
  - [x] update: `watchChannel` — log each SSE message: level `Verbose`, stage `n10n.sse_sent`, msg `<sse message>`
  - [x] update: SSE send error: level `Error`, stage `n10n.watch.sse_error`, msg `<error>`
  - [x] add: Local constants for `channelid`, `projectionkey` attribute keys
  - [x] add: `watchChannel` goroutine finish: level `Verbose`, stage `n10n.watch.done`, msg (empty)

- [x] update: [pkg/router/impl_n10n.go](../../../pkg/router/impl_n10n.go)
  - [x] add: `serveN10NChannel` finish: level `Verbose`, stage `n10n.watch.done`, msg (empty)
  - [x] add: `subscribeHandler` success: level `Verbose`, stage `n10n.subscribe.success`, msg empty
  - [x] add: `subscribeHandler` — enrich `logCtx` with `projectionkey` (JSON array) after error check when at least one key
  - [x] add: `unSubscribeHandler` success: level `Verbose`, stage `n10n.unsubscribe.success`, msg empty
  - [x] add: `unSubscribeHandler` — enrich `logCtx` with `projectionkey` (JSON array) after error check when at least one key
  - [x] add: `subscribeAndWatchHandler` — enrich `logCtx` with `channelid` after `NewChannel` succeeds
  - [x] add: `subscribeAndWatchHandler` — enrich `logCtx` with `projectionkey` (JSON array) after subscribe loop when at least one key
  - [x] add: `subscribeAndWatchHandler` success: level `Verbose`, stage `n10n.subscribe&watch.success`, msg empty
  - [x] refactor: remove `projKeys` parameter from `serveN10NChannel` (was only used for success msg conditional)

- [x] update: [pkg/router/consts.go](../../../pkg/router/consts.go)
  - [x] add: `logAttrib_ProjectionKey = "projectionkey"` constant
  - [x] add: `logAttrib_ChannelID = "channelid"` constant

- [x] update: [pkg/processors/n10n/impl_subscribeextra.go](../../../pkg/processors/n10n/impl_subscribeextra.go)
  - [x] add: `projectionkey` attrib (JSON array via `ProjectionKeysToJSON`) to context
  - [x] add: After successful subscribe: level `Verbose`, stage `n10n.subscribe.success`, msg empty
  - [x] fix: remove redundant `""` arg from `logSubscribeSuccess`

- [x] update: [pkg/processors/n10n/impl_unsubscribe.go](../../../pkg/processors/n10n/impl_unsubscribe.go)
  - [x] add: `projectionkey` attrib (JSON array via `ProjectionKeysToJSON`) to context
  - [x] add: After successful unsubscribe: level `Verbose`, stage `n10n.unsubscribe.success`, msg empty
  - [x] fix: remove redundant `""` arg from `logUnsubscribeSuccess`

- [x] update: [pkg/processors/consts.go](../../../pkg/processors/consts.go)
  - add: `APIPath_N10N_SubscribeAndWatch` constant

- [x] update: [pkg/router/utils.go](../../../pkg/router/utils.go)
  - update: `apiPathToExtension()` — add case for `APIPath_N10N_SubscribeAndWatch` returning `"sys._N10N_SubscribeAndWatch"`

- [x] add tests: [pkg/processors/n10n/impl_test.go](../../../pkg/processors/n10n/impl_test.go)
  - helper `newN10nWP(channelID, projectionKey)` — creates `n10nWorkpiece` with `logCtx` enriched with `channelid=<channelID>` and `projectionkey=<projectionKey>`
  - subtest `n10n.error`: call `reportError` with a workpiece whose `logCtx` has `channelid` and `projectionkey`; assert log contains `stage=n10n.error`, error message, `channelid=`, `projectionkey=`

- [x] add tests: [pkg/processors/n10n/impl_subscribeandwatch_test.go](../../../pkg/processors/n10n/impl_subscribeandwatch_test.go)
  - subtest `n10n.subscribe&watch.success single key`: call success-log step with a workpiece with 1 subscription; assert log contains `stage=n10n.subscribe&watch.success`, `projectionkey=<key>`, `channelid=<id>`; assert msg is empty
  - subtest `n10n.subscribe&watch.success multi key`: call success-log step with a workpiece with 2+ subscriptions; assert log contains `stage=n10n.subscribe&watch.success`, `channelid=<id>`, `projectionkey=` (JSON array of both keys)
  - subtest `n10n.sse_sent`: call the SSE send log path with a workpiece with `channelid`; assert log contains `stage=n10n.sse_sent`, SSE message body, `channelid=`
  - subtest `n10n.watch.sse_error`: simulate SSE write failure in `watchChannel`; assert log contains `stage=n10n.watch.sse_error`, error message, `channelid=`
  - [x] subtest `n10n.watch.done` (APIv2): call `watchChannel` with a `immediateWatchBroker` that returns immediately; assert log contains `stage=n10n.watch.done`, `channelid=`

- [x] add tests: [pkg/processors/n10n/impl_subscribeextra_test.go](../../../pkg/processors/n10n/impl_subscribeextra_test.go)
  - subtest `n10n.subscribe.success`: call the subscribe-success log step with a workpiece that has `entityFromURL`, `channelID`; assert log contains `stage=n10n.subscribe.success`, `projectionkey=<entity>`, `channelid=<id>`

- [x] add tests: [pkg/processors/n10n/impl_unsubscribe_test.go](../../../pkg/processors/n10n/impl_unsubscribe_test.go)
  - subtest `n10n.unsubscribe.success`: call the unsubscribe-success log step with a workpiece that has `entityFromURL`, `channelID`; assert log contains `stage=n10n.unsubscribe.success`, `projectionkey=<entity>`, `channelid=<id>`

- [x] Review

### N10N broker lifecycle

- [x] update: [pkg/in10nmem/impl.go](../../../pkg/in10nmem/impl.go)
  - add: Create log context in `NewN10nBroker` with `vapp=sys.VApp_SysVoedger`, `extension="sys._N10NBroker"`
  - update: `notifier` start: level `Info`, stage `n10n.notifier.start`, msg (empty)
  - update: `notifier` stop: level `Info`, stage `n10n.notifier.stop`, msg (empty)
  - update: `heartbeat30` start: level `Info`, stage `n10n.heartbeat.start`, msg `Heartbeat30Duration: <duration>`
  - update: `heartbeat30` stop: level `Info`, stage `n10n.heartbeat.stop`, msg (empty)
  - add: Channel expired during `WatchChannel`: level `Error`, stage `n10n.channel.expired`, msg `<subjectLogin>`
  - add: Channel cleanup unsubscribe error: level `Error`, stage `n10n.cleanup.error`, attribs `channelid=<id>`, `projectionkey=[<key>]`, msg `<error>`
  - drop: All `logger.Trace(...)` calls not described in TD (notifier loop, heartbeat loop, WatchChannel, channel management)
  - drop: `logTrace` helper in `utils.go` (no longer used)

- [x] update: [pkg/in10nmem/provide.go](../../../pkg/in10nmem/provide.go)
  - add: `logCtx` created with `logger.WithContextAttrs` using `vapp=sys.VApp_SysVoedger`, `extension="sys._N10NBroker"`
  - update: `logCtx` stored on broker struct and passed to `notifier` goroutine

- [x] update: [pkg/in10nmem/types.go](../../../pkg/in10nmem/types.go)
  - add: `logCtx context.Context` field to `N10nBroker` struct

- [x] drop: [pkg/in10nmem/utils.go](../../../pkg/in10nmem/utils.go) — `logTrace` helper no longer used

- [x] Review

### Schedulers

- [x] update: [pkg/processors/schedulers/impl_scheduler.go](../../../pkg/processors/schedulers/impl_scheduler.go)
  - add: `logCtx context.Context` field to `scheduler` struct
  - update: `keepRunning` — log (re)schedule: level `Verbose`, stage `job.schedule`, msg `now=<timeNow>,next=<nextRunTime>`
  - update: `keepRunning` — log wake-up: level `Verbose`, stage `job.wake-up`, msg `<timeNow>`
  - update: `runJob` — log successful invoke: level `Verbose`, stage `job.success`, msg (empty)
  - update: `runJob` defer error — level `Error`, stage `job.error`, msg `<error>`, using `ErrorCtx`
  - update: `Prepare` retrier OnError — level `Error`, stage `job.error`, using `ErrorCtx`:
    - if `appparts.ErrNotFound`: msg `appparts <error>, will try again`, retries
    - otherwise: msg `<error>`, aborts
  - drop: `logger.Info(a.name, "schedule"...)` and `logger.Info(a.name, "wake"...)` calls
  - drop: `logger.Trace(a.name, "started")` in `init`
  - drop: `logger.Trace(a.name + "s finalized")` in `finit`
  - drop: `logger.Verbose("invoked " + a.name)` in `runJob` (replaced by TD-defined `job.success`)

- [x] update: [pkg/processors/schedulers/impl_schedulers.go](../../../pkg/processors/schedulers/impl_schedulers.go)
  - add: `logCtx` created with `logger.WithContextAttrs` using `vapp=app`, `extension="job.<job QName>"`, `wsid=wsid`
  - update: `logCtx` passed to `scheduler` struct on creation in `NewAndRun`

- [ ] Review
