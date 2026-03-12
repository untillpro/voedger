# How: Use context-aware logging in actualizers

## Approach

- Pass the base `vvmCtx` from `pkg/appparts/impl.go` into actualizer deployment
- Build the async actualizer base `logCtx` with `vapp` and `extension` when projector runtimes are started in `pkg/appparts/internal/actualizers/actualizers.go`
- In `asyncProjector.DoAsync`, resolve `triggeredByQName := ProjectorEvent(...)` and skip events that do not trigger the projector
- Add `wsid` to the async actualizer `logCtx` in `DoAsync`
- Keep async error propagation context-aware by wrapping failures into `errWithCtx{error, logCtx}` and letting `asyncActualizer.logError` emit `logger.ErrorCtx(...)`
- Extract shared verbose event and CUD logging into `pkg/processors.LogEventAndCUDs(...)`
  - enrich the context with `woffset`, `poffset`, `evqname`
  - log `args=<JSON>`
  - log each selected CUD with `rectype`, `recid`, `op`
  - emit shared `newfields=...`
  - let the caller decide whether to log a CUD and which extra text to append through `func(istructs.ICUDRow) (bool, string, error)`
- Make command logging call the shared helper and append `oldfields=...` in its callback
- Preserve the reserved event offset for command logging and sync actualizers
  - add `pLogOffset` to `cmdWorkpiece`
  - expose `Context()` and `PLogOffset()` on `cmdWorkpiece`
  - insert `setPLogOffset` before raw event building in the command pipeline
- Make sync actualizers reuse the same shared logging flow before projector invocation
- In actualizers, choose logged CUDs from the resolved `triggeredByQName`
  - if the triggering kind is function, `ODoc`, or `ORecord`, log all event CUDs
  - otherwise log only CUDs whose `QName` matches `triggeredByQName`
- Keep async `success` logging in `DoAsync`
- Update tests to cover shared logging output, `ProjectorEvent()` QName semantics, async failure logging, and sync actualizer helper mocks
