---
registered_at: 2026-03-05T15:28:31Z
change_id: 2603051528-use-loggerctx-in-actualizers
baseline: 7bccb40d1948b6a0b0700fbe72d6859aa1229d21
archived_at: 2026-03-10T14:00:55Z
---

# Change request: Use context-aware logging in actualizers

## Why

Actualizers have insufficient logging. Need to add log attribs and use loggerctx

## What

Replace plain logger calls with context-aware variants in the actualizers package:

- add `vapp`, `wsid`, `extension` to ctx as soon as that data is available
- eliminate `logger.Trace()`
- before the event handling is started:
  - log the event:
    - ctx: `woffset`, `poffset`, `evqname`
    - msg: `args=<JSON>`
  - if the projector is after insert/after update:
    - log only these CUDs that triggered this projector. Use the approach as done in command processor: per each CUD log `rectype`, `recid`, `op` etc
  - if the projector is after execute:
    - log all event's CUDs

