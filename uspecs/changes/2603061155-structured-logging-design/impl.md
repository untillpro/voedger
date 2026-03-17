# Implementation plan: Design logging subsystem architecture

## Functional design

- [x] update: [prod/prod--domain.md](../../specs/prod/prod--domain.md)
  - update: Rename context from `monitoring` to `observability`
  - update: Context map mermaid diagram (monitoring -> observability)
  - update: Detailed relationships section (monitoring -> observability)
  - update: Context description section header (monitoring -> observability)
  - update: External actor relationships (monitoring -> observability)

## Technical design

- [x] create: [prod/apps/logging--td.md](../../specs/prod/apps/logging--td.md)
  - Describe technical design

### Tests

- [ ] run: Logger tests
  - `go test -short ./pkg/goutils/logger/...`

- [ ] Review
