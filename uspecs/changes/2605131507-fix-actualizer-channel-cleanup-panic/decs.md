# Decisions: Fix double channel cleanup panic in async actualizer

## Root cause of `panic: channel terminated`

The panic is caused by `a.channelCleanup` retaining a closure that references an already-terminated channel across retry iterations of `asyncActualizer.Run`, not by `finit()` being invoked twice within a single iteration (confidence: high).

Rationale:

The panic stack shows `cleanupChannel` reaching the guard at [pkg/in10nmem/impl.go:265](../../../../../pkg/in10nmem/impl.go):

```go
if channel.terminated {
    panic(in10n.ErrChannelTerminated)
}
```

For this guard to fire, the same cleanup closure instance must be invoked twice on the same `*channel`. The codebase admits exactly one such path:

- `channel.terminated = true` is set ONLY inside `cleanupChannel` itself ([impl.go:274](../../../../../pkg/in10nmem/impl.go)). No other path can pre-mark a channel terminated.
- The cleanup closure is created uniquely per `NewChannel` call ([impl.go:58](../../../../../pkg/in10nmem/impl.go)), capturing one specific `&channel`, `channelID`, `metric` triple. It is the only call site of `cleanupChannel`.
- In the actualizer, `a.channelCleanup` is written ONLY at [async.go:222](../../../../../pkg/processors/actualizers/async.go) (multi-value assignment from `Broker.NewChannel`; on `NewChannel` error the field is overwritten to `nil`).
- `a.channelCleanup` is read ONLY at [async.go:237](../../../../../pkg/processors/actualizers/async.go) inside `finit()`.
- `finit()` is called from a single site ([async.go:109](../../../../../pkg/processors/actualizers/async.go)) — exactly once per `RetryNoResult` attempt; `RetryNoResult` calls its op once per attempt and retries on error ([retry/utils.go](../../../../../pkg/goutils/retry/utils.go)).
- `Run` is invoked from one goroutine per `asyncActualizer` (`PartitionActualizers.start` → `actualizers.NewAndRun` → `asyncActualizer.Run`), so no concurrent `finit`.

Combining these facts, the only way the same closure can run twice is across iterations:

1. Iteration N: `init()` succeeds → `a.channelCleanup` = closure_N → `keepReading()` → `finit()` invokes closure_N → channel marked `terminated`
2. After `finit()`, `a.pipeline = nil` is reset (per AIR-2302) but `a.channelCleanup` is NOT reset
3. Iteration N+1: `init()` returns early before reaching [async.go:222](../../../../../pkg/processors/actualizers/async.go) (e.g., `appParts.AppDef` returns error, `appdef.Projector` returns nil, or `readOffset` fails — all plausible during VIT reset when app parts are torn down/restored)
4. `a.channelCleanup` still references closure_N → `finit()` invokes closure_N again → `cleanupChannel` panics on the terminated channel

The existing comment at [async.go:111-114](../../../../../pkg/processors/actualizers/async.go) describes this exact bug class for `a.pipeline` (AIR-2302). The fix nilled `a.pipeline` but missed `a.channelCleanup`, which has identical lifecycle semantics.

Alternatives considered and ruled out:

- `finit()` invoked twice within the same iteration (confidence: ruled out)
  - `Run.func1` has a single linear `a.finit()` call. `RetryNoResult` is a standard one-call-per-attempt loop. No second invocation site exists.
- Concurrent `finit()` from another goroutine (confidence: ruled out)
  - Each `asyncActualizer` is owned by exactly one goroutine started by `PartitionActualizers.start`. The struct is not shared.
- Channel terminated via a different path (e.g., broker shutdown, `WatchChannel`, `Unsubscribe`) (confidence: ruled out)
  - `channel.terminated = true` has only one assignment site (inside `cleanupChannel`). Broker shutdown does not mark individual channels terminated.
- Closure aliasing across two distinct `NewChannel` calls (confidence: ruled out)
  - Each `NewChannel` invocation constructs a fresh `channel` value and a fresh closure capturing its address. No reuse across calls.

## Fix placement: reset in `Run` vs. inside `finit()` vs. broker idempotency

Reset `a.channelCleanup = nil` in `Run` immediately after `finit()`, next to `a.pipeline = nil` (confidence: high).

Rationale: matches the established AIR-2302 pattern at the same call site, keeps the two related lifecycle resets together, and preserves the broker invariant that `cleanupChannel` is called exactly once per channel (a double call remains a programming error, not a silently tolerated condition).

Alternatives:

- Reset inside `finit()` itself (confidence: medium)
  - Makes `finit` idempotent through better encapsulation, but diverges from the existing convention in this file where post-`finit` resets live in the caller
- Make `cleanupChannel` idempotent in the broker (confidence: low)
  - Would mask the contract violation broker-wide and hide future regressions in other callers

## Reproduction strategy in unit test

Drive `asyncActualizer.Run` through its natural retry loop with a `flakyAppParts` decorator over `IAppPartitions`, failing `WaitForBorrow` #2 then `AppDef` #2 (confidence: high).

Rationale: exercises the exact production code path (`init` → `keepReading` → `finit` → `init` again) without invasively double-calling `finit()`, mirroring the real-world cause (transient app-part availability during VIT reset).

Alternatives:

- Inject a failing projector function to fail `keepReading` (confidence: medium)
  - Projector errors are caught by `asyncErrorHandler` and do not propagate as `RetryNoResult` op errors, so iter 2 init-time failure cannot be triggered
- Cancel `readCtx` from outside via reflection (confidence: low)
  - Invasive, brittle, couples test to internal field names
