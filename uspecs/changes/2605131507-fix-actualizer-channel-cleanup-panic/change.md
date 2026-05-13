---
registered_at: 2026-05-13T15:07:03Z
change_id: 2605131507-fix-actualizer-channel-cleanup-panic
baseline: 5b5dd6d7881494fafcc7571ebcdf196acd987d82
issue_url: https://untill.atlassian.net/browse/AIR-3888
---

# Change request: Fix double channel cleanup panic in async actualizer

## Why

`TestVITResetPreservingStorage` sometimes fails with `panic: channel terminated` originating from `N10nBroker.cleanupChannel` when invoked twice on the same channel via `asyncActualizer.finit`. See [issue.md](issue.md) for details.

## What

Prevent stale `channelCleanup` reuse across retry iterations of `asyncActualizer.Run`:

- Reset `a.channelCleanup` to `nil` after `finit()` in the retry loop, alongside the existing `a.pipeline = nil` reset
- Add a regression test that drives the retry loop through the production failure path via a flaky `IAppPartitions` decorator
