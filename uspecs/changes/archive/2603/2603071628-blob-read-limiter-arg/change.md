---
registered_at: 2026-03-07T09:53:02Z
change_id: 2603070953-blob-read-limiter-arg
baseline: 2a51ec2ecd4ffaf4e39815c79704b9b80cb48425
archived_at: 2026-03-07T16:28:44Z
---

# Change request: Add read limiter arg to blobprocessor IRequestHandler blob reads

## Why

Blob reads handled through `blobprocessor.IRequestHandler` need to accept a read limiter so callers can explicitly control read limiting behavior. The interface should expose `RLimiterType` now while keeping current behavior unchanged by using `RLimiter_Null`.

## What

This change updates the blob read API surface of `blobprocessor.IRequestHandler`:

- Add a read limiter argument to read blob methods
- Use `RLimiterType` as the argument type
- Pass `RLimiter_Null` in current call paths so behavior stays unchanged

This change keeps the first step intentionally narrow:

- Limit the initial change to interface and related signatures needed for compilation
- Preserve existing runtime behavior until non-null read limiter handling is introduced
