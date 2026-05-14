---
registered_at: 2026-05-12T14:34:28Z
change_id: 2605121434-registry-token-deactivated-profile
baseline: 4f210eb8ab63962037c3dc2e58456093f27056a2
issue_url: https://untill.atlassian.net/browse/AIR-3892
archived_at: 2026-05-13T14:52:03Z
---

# Change request: registry — treat deactivated profile as missing login on IssuePrincipalToken

## Why

When a user profile is deactivated via `c.sys.InitiateDeactivateWorkspace`, `q.registry.IssuePrincipalToken` currently fails with HTTP 410 Gone (`workspace status is not active`), exposing the existence of a deactivated profile. That is wrong according to GDPR requirements. See [issue.md](issue.md) for details.

## What

Adjust `q.registry.IssuePrincipalToken` behavior for deactivated profiles:

- Detect deactivated profile workspace status during token issuance
- Return 401 status code and the same `login does not exist` error as for a missing login instead of propagating the 410 Gone status

Check if `cdoc.registry.Login.IsActive` becomes false
Check other places where `view.registry.LoginIdx` or `cdoc.registry.Login` are touched: if corresponding `cdoc.registry.Login.IsActive` is false then consider the login is missing
