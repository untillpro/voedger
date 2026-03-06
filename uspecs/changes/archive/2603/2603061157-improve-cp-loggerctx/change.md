---
registered_at: 2026-03-05T12:29:51Z
change_id: 2603051229-improve-cp-loggerctx
baseline: a6684f7d9bf2c97180ac071782af45613329e7a1
archived_at: 2026-03-06T11:57:08Z
---

# Change request: Improve loggerctx in command processor

## Why

The command processor logger context lacks important attributes and does not log key events, making it harder to trace and debug command execution in production.

## What

Rename the app clog attribute:

- Rename `app` clog attribute to `vapp`

Log the event after it is saved to plog (requires recid, which is the result of `SavePLog`):

- ctx: `woffset`, `poffset`, `evqname`
- msg: args JSON

For each CUD from the resulting event (use actual IDs, not raw IDs; take CUDs from the saved event, not from those received from the client):

- create new context with attribs:
  - `rectype` (e.g. `untill.cdoc.article`)
  - `recid` (e.g. `78097`)
  - `op` — create, update, activate, deactivate etc
- log msg: new fields, old fields with this context
- do not use this context anymore

