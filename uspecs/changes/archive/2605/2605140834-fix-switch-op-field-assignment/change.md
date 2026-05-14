---
registered_at: 2026-05-14T08:26:18Z
change_id: 2605140826-fix-switch-op-field-assignment
baseline: 816987454e5ee9cd6c094bb07fd41cdffe850db9
issue_url: https://untill.atlassian.net/browse/AIR-3910
archived_at: 2026-05-14T08:34:53Z
---

# Change request: Fix wrong struct field assignment in switchOperator

## Why

In `pkg/pipeline/switch-operator-impl.go`, `switchOperator.DoSync` has a value receiver yet assigns to `s.currentBranchName`. The mutation is silently lost outside the call, the field is never read elsewhere, and the pattern is misleading and unsafe under concurrent calls. See [issue.md](issue.md) for details.

## What

Fix the misuse of the struct field in `switchOperator`:

- Replace the use of `s.currentBranchName` in `DoSync` with a local variable.
- Remove the now-unused `currentBranchName` field from the `switchOperator` struct.
- Update `pkg/pipeline/core-pipeline-design.archimate` to drop the `currentBranchName` artifact reference.
