# Decisions: Use context-aware logging in actualizers

## 1. Build the async actualizer base log context before event handling

Build the base async actualizer `logCtx` with `vapp` and `extension` when the projector runtime starts, then add `wsid` per event in `DoAsync` (confidence: high)

Rationale: `vapp` and projector `extension` are known when async actualizers are started in `pkg/appparts/internal/actualizers/actualizers.go`, so they should be attached there and reused by later logs. `wsid` is only known from the current event, so `DoAsync` enriches the base context right before handling that event

Alternatives:

- Build the whole log context only in `DoAsync` (confidence: medium)
  - Simpler, but logs emitted before event handling lose `vapp` and `extension`

## 2. Actualizer CUD logging is driven by the resolved triggering QName

Use `ProjectorEvent(...)` once to resolve `triggeredByQName`, then decide which CUDs to log from that QName and its type kind (confidence: high)

Rationale: the final implementation changed `ProjectorEvent(...)` to return the triggering `QName`, not only a boolean. Async and sync actualizers reuse that resolved value. When the triggering kind is a function, `ODoc`, or `ORecord`, logging all event CUDs matches execute-style projector behavior. Otherwise actualizers log only CUDs whose `QName` matches `triggeredByQName`, which keeps after-insert, after-update, after-activate, and after-deactivate logs focused without repeating trigger checks inline

Alternatives:

- Re-run `prj.Triggers(...)` for every CUD inside logging (confidence: medium)
  - Possible, but duplicates trigger-resolution logic that already lives in `ProjectorEvent(...)`
- Log all CUDs for every projector type (confidence: low)
  - Simpler, but too noisy for record-triggered projectors

## 3. Async success is logged in `DoAsync`, failures are logged through `errWithCtx`

Keep explicit `success` logging in `DoAsync` and propagate failures as `errWithCtx` so the outer async actualizer error path logs them with the enriched context (confidence: high)

Rationale: the final implementation wraps errors after `wsid` is attached and keeps the actual error emission in `asyncActualizer.logError`. This preserves one failure-logging path while still carrying the event-specific logger context. Adding a second explicit `failure` message in `DoAsync` would duplicate failure reporting without adding context

Alternatives:

- Emit an explicit `failure` log in `DoAsync` before every error return (confidence: medium)
  - Symmetric with `success`, but duplicates the existing error log path
- Remove `errWithCtx` and log every error at each return site (confidence: low)
  - More repetitive and easier to drift over time

## 4. Event args logging defaults to `{}` and serializes only real argument objects

Initialize `argsJSON` as `{}` and serialize `event.ArgumentObject()` only when it exists and its `QName` is not `appdef.NullQName` (confidence: high)

Rationale: the shared helper in `pkg/processors/utils.go` follows this exact contract. It keeps the log message shape stable as `args=...` for every event while avoiding unnecessary object-to-map conversion for CUD-only and other null-argument events

Alternatives:

- Skip the `args=` log entirely when there is no argument object (confidence: medium)
  - Reduces one log line variant, but makes event logs less uniform
- Always try to serialize the argument object without the null guard (confidence: low)
  - Adds pointless work and relies on null-object behavior staying benign

## 5. Shared event and CUD logging lives in `pkg/processors` and stays caller-extensible

Extract common verbose event and CUD logging into `processors.LogEventAndCUDs(...)` and keep caller-specific behavior in a per-CUD callback that returns `(shouldLog, extraMsg, err)` (confidence: high)

Rationale: command processor, async actualizers, and sync actualizers now share the same verbose guard, event attrs, args logging, per-CUD attrs, shared `newfields=...` output, and stack-frame handling. The remaining differences stay local: command logging appends `oldfields=...`, actualizers decide whether a CUD should be logged, and sync actualizers reuse the same helper through `cmdWorkpiece.Context()` and `PLogOffset()`

Alternatives:

- Keep separate event/CUD logging implementations in command processor and actualizers (confidence: medium)
  - Simpler locally, but leaves duplicated logic in multiple call sites
- Move old-record formatting and projector-specific filtering into one larger shared helper (confidence: low)
  - Shares more code, but makes the common API heavier and less readable

## 6. Sync actualizers log against the reserved command event offset

Expose `Context()` and `PLogOffset()` on `cmdWorkpiece` and reserve the `pLogOffset` before raw event building so sync actualizers log the same event coordinates as the command processor (confidence: high)

Rationale: the final implementation inserts `setPLogOffset` into the command pipeline before raw event building, stores that value on `cmdWorkpiece`, and keeps it available during recovery. This lets sync actualizers call the shared logging helper with the same request context and reserved event offset as the command path

Alternatives:

- Let sync actualizers log with a recomputed or missing `PLogOffset` (confidence: low)
  - Would make event logs less trustworthy
- Keep a separate sync actualizer logging path that does not depend on command workpiece state (confidence: medium)
  - Avoids the extra workpiece fields, but duplicates the logging flow again
