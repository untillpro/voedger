# AIR-3892: registry: consider user profile status on IssuePrincipalToken

- **Type**: Sub-task
- **Status**: In Progress
- **Assignee**: d.gribanov@dev.untill.com
- **URL**: <https://untill.atlassian.net/browse/AIR-3892>

## Why

If a user profile (the login, not `cdoc.registry.Login`) is deactivated via `c.sys.InitiateDeactivateWorkspace`, `IssuePrincipalToken` currently returns status code `410 Gone` with the message `workspace status is not active`. This is incorrect according to GDPR requirements.

## What

Update `q.registry.IssuePrincipalToken` so that if the profile has been deactivated, it returns the same result as when the login does not exist, as if it were missing.

## How

If the profile has been deactivated, `cdoc.registry.Login.IsActive` is set to `false` (already done).
