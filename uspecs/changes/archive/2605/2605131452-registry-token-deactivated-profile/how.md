# How: registry — treat deactivated profile as missing login on IssuePrincipalToken

## Approach

- Centralize the "login exists" semantics in `GetCDocLogin` in `pkg/registry/utils.go`: return `loginExists=false` when the resolved `cdoc.registry.Login` has `sys.IsActive=false`, so all current callers (`q.registry.IssuePrincipalToken`, `c.registry.ChangePassword`, `q.registry.InitiateResetPasswordByEmail`, `q.registry.IssueVerifiedValueTokenForResetPassword`, `c.registry.ResetPasswordByEmail`) automatically respond with `errLoginOrPasswordIsIncorrect` / their existing "login does not exist" path instead of leaking 410 Gone
  - Emit `logger.Verbose` from `GetCDocLogin` when an inactive `cdoc.registry.Login` is encountered, stating that the profile is deactivated and is being treated as a missing login (only the resolved `RecordID` in the message, no login value, to avoid leaking credentials to logs); ctx is intentionally not threaded — matches the `logger.Verbose` style of the surrounding deactivation cascade in `pkg/sys/workspace/impl_deactivate.go` and avoids a placeholder `context.Background()` on command paths whose `ExecCommandArgs` carries no ctx
- `cdoc.registry.Login.IsActive` is the sole signal: when set to `false` the registry treats the login as missing. The flip is driven by the existing deactivation cascade `c.sys.InitiateDeactivateWorkspace` -> `projectorApplyDeactivateWorkspace` -> `c.sys.OnChildWorkspaceDeactivated` -> `cmdOnChildWorkspaceDeactivatedExec` in `pkg/sys/workspace/impl_deactivate.go`. For profile workspaces this cascade crosses apps (profile WS lives in e.g. `test1/app1`, the registry login lives in `sys/registry`), so the projector is fixed to issue a system token for the owner app in addition to its own app token; both `c.sys.OnWorkspaceDeactivated` and `c.sys.OnChildWorkspaceDeactivated` calls into the owner app are then accepted by per-app token validation
- Audit the remaining direct readers of `view.registry.LoginIdx` / `cdoc.registry.Login` so the same "treat inactive as missing" rule is uniform:
  - `GetCDocLoginID` in `utils.go` — keep returning the `RecordID`; the IsActive filter stays in `GetCDocLogin`
  - `vit.GetCDocLoginID` in `pkg/vit/utils.go` and any other direct `q.sys.SqlQuery` against `LoginIdx` are test helpers and stay as-is
- Update `impl_deactivateworkspace_test.go` to cover the deactivated-login case via the full production cascade: deactivate the user profile WS with `c.sys.InitiateDeactivateWorkspace`, wait for `Status=Inactive`, then assert each login-touching registry function returns its missing-login response (and never leaks 410 Gone)

## Out of scope

- `c.registry.CreateLogin` / `c.registry.CreateEmailLogin` allowing recreation of a previously deactivated login — separate task

References:

- [pkg/registry/impl_issueprincipaltoken.go](../../../../../pkg/registry/impl_issueprincipaltoken.go)
- [pkg/registry/utils.go](../../../../../pkg/registry/utils.go)
- [pkg/registry/impl_resetpassword.go](../../../../../pkg/registry/impl_resetpassword.go)
- [pkg/registry/impl_changepassword.go](../../../../../pkg/registry/impl_changepassword.go)
- [pkg/registry/consts.go](../../../../../pkg/registry/consts.go)
- [pkg/registry/appws.vsql](../../../../../pkg/registry/appws.vsql)
- [pkg/sys/workspace/impl_deactivate.go](../../../../../pkg/sys/workspace/impl_deactivate.go)
- [pkg/processors/errors.go](../../../../../pkg/processors/errors.go)
