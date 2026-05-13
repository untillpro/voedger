# Implementation plan: Fix double channel cleanup panic in async actualizer

## Construction

- [x] update: [async.go](../../../../../pkg/processors/actualizers/async.go)
  - fix: reset `a.channelCleanup = nil` after `finit()` in `Run` retry loop, alongside `a.pipeline = nil`, to prevent stale cleanup closure invocation when `init()` returns early before re-acquiring a channel
  - add rationale comment mirroring the AIR-2302 pipeline comment, referencing AIR-3888 and the `N10nBroker.cleanupChannel` "channel terminated" panic
- [x] add: [async_test.go](../../../../../pkg/processors/actualizers/async_test.go) — `Test_AsynchronousActualizer_ChannelCleanupPanicReproduction`
  - drive `asyncActualizer.Run` through the production retry loop with a `flakyAppParts` decorator that fails `WaitForBorrow` #2 (iter 1 `keepReading`) and `AppDef` #2 (iter 2 `init` before `NewChannel`)
  - assert no panic via `recover()` + `runtime/debug.Stack()`; report the original `N10nBroker.cleanupChannel` stack on failure
  - test fails (panics) without the fix and passes with it
