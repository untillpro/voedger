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

Design a logging subsystem architecture that provides:

- Ability to trace command, query, and event processing
- Tracking of vapp, reqid, wsid, and extension stages
- Duration measurement for each processing stage
- Structured logging format for consistent log analysis

Make sources use stage attribute