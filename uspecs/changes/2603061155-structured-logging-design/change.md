---
registered_at: 2026-03-06T11:55:11Z
change_id: 2603061155-structured-logging-design
baseline: 6a30e3a65a64454e2159c28cd3465bb0f9ddbe2c
issue_url: https://untill.atlassian.net/browse/AIR-3236
---

# Change request: Design logging subsystem architecture

## Why

We need a structured logging architecture that enables tracing of command, query, and event processing across the system. This will improve observability and debugging capabilities by allowing us to track requests through different stages and components.

See [issue.md](issue.md) for details.

## What

Design a structured logging subsystem that provides:

- Context-aware `*Ctx` logging functions with `stage` parameter, backed by `log/slog`
- Standard log attributes: `vapp`, `reqid`, `wsid`, `extension`, `feat`, `stage`
- Per-component logging spec: Router, Command Processor, Query Processor, Sync/Async Projectors
- Shared `processors.LogEventAndCUDs()` utility for event and CUD logging

See [logging--td.md](../../specs/prod/apps/logging--td.md) for details.

Testing:

- Unit tests for context-aware logging functions (`pkg/goutils/logger`)
