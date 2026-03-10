# Implementation plan: Design logging subsystem architecture

## Functional design

- [x] update: [prod/prod--domain.md](../../specs/prod/prod--domain.md)
  - update: Rename context from `monitoring` to `observability`
  - update: Context map mermaid diagram (monitoring -> observability)
  - update: Detailed relationships section (monitoring -> observability)
  - update: Context description section header (monitoring -> observability)
  - update: External actor relationships (monitoring -> observability)

## Technical design

- [x] create: [prod/observability/logging--td.md](../../specs/prod/observability/logging--td.md)
  - add: Logging subsystem architecture document
  - add: Overview of structured logging approach
  - add: Key components (logger package, context propagation, log attributes)
  - add: Key data models (log attributes: vapp, reqid, wsid, extension, duration)
  - add: Integration points with existing logger package
  - add: Tracing strategy for different processing stages
