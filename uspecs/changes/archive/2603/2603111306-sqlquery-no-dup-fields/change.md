---
registered_at: 2026-03-11T12:18:41Z
change_id: 2603111218-sqlquery-no-dup-fields
baseline: db41f9b5fc01294774f9edc07484dfa4c65ced00
archived_at: 2026-03-11T13:06:47Z
---

# Change request: Sqlquery deny duplicate field selection

## Why

Selecting the same field more than once in a SQL query — including via `blobinfo` and `blobtext` functions — produces redundant or ambiguous results and should be rejected with a clear error.

## What

Validation is added to `sqlquery` to reject queries that reference the same field more than once in the SELECT clause:

- Detect duplicate field names in the SELECT list, including fields implicitly produced by `blobinfo()` and `blobtext()` functions
- Return an error when a duplicate is detected, before query execution
- Update integration tests in `pkg/sys/it/impl_sqlquery_test.go` to cover the new validation
