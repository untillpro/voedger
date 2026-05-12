---
registered_at: 2026-05-01T14:57:02Z
change_id: 2605011457-sqlquery-int8-int16-view-keys
baseline: 4f210eb8ab63962037c3dc2e58456093f27056a2
issue_url: https://untill.atlassian.net/browse/AIR-3801
archived_at: 2026-05-02T14:03:34Z
---

# Change request: Support int8 and int16 in sqlquery view WHERE clauses

## Why

`q.sys.SqlQuery` cannot read from views whose key fields use `DataKind_int8` or `DataKind_int16` (e.g. `view.air.FdmLog`) because `pkg/sys/sqlquery/impl_viewrecords.go` rejects these kinds with `errUnsupportedDataKind` when binding WHERE literals to the `KeyBuilder`. See [issue.md](issue.md) for details.

## What

Bring sqlquery in line with the int8/int16 storage support already present in voedger:

- Extend the `DataKind` switch in `pkg/sys/sqlquery/impl_viewrecords.go` to fall through `DataKind_int8` and `DataKind_int16` into the existing `kb.PutNumber` case
- Add coverage for the new kinds in `pkg/sys/it/impl_sqlquery_test.go` (or a focused unit test)
