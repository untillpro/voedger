# Implementation plan: registry — treat deactivated profile as missing login on IssuePrincipalToken

## Construction

- [x] update: [pkg/registry/utils.go](../../../../../pkg/registry/utils.go)
  - update: `GetCDocLogin` returns `loginExists=false` when the resolved `cdoc.registry.Login` has `sys.IsActive=false`
  - add: `logger.Verbose` emission on inactive-login hit (only the resolved `RecordID`, no login value, to avoid leaking credentials to logs); ctx is intentionally not threaded — matches the `logger.Verbose` style used by the surrounding deactivation cascade in `pkg/sys/workspace/impl_deactivate.go` and avoids placeholder `context.Background()` on command paths (`ExecCommandArgs` carries no ctx)
- [x] update: [pkg/registry/impl_resetpassword.go](../../../../../pkg/registry/impl_resetpassword.go)
  - update: in `provideQryInitiateResetPasswordByEmailExec` replace the inline `GetCDocLoginID` + direct `cdoc.registry.Login` read with a single `GetCDocLogin` call so the IsActive filter and `logger.Verbose` emission are not duplicated
- [x] update: [pkg/sys/workspace/impl_deactivate.go](../../../../../pkg/sys/workspace/impl_deactivate.go)
  - fix: cross-app token in `projectorApplyDeactivateWorkspace` — the projector now issues a separate `payloads.GetSystemPrincipalToken` for `ownerAppQName` when it differs from the projector app, so `c.sys.OnWorkspaceDeactivated` / `c.sys.OnChildWorkspaceDeactivated` calls into the owner app (e.g. `sys/registry` for profile WS) authenticate with a token issued for that app instead of the projector app
  - rename: `appQName`/`sysToken` -> `projectorAppQName`/`projectorAppToken` to disambiguate from `ownerAppQName`/`ownerAppToken`
  - `ownerAppQName` is parsed via `appdef.MustParseAppQName` (a malformed value cannot be persisted in `cdoc.sys.WorkspaceDescriptor.OwnerApp`)
- [x] update: [pkg/sys/it/impl_deactivateworkspace_test.go](../../../../../pkg/sys/it/impl_deactivateworkspace_test.go)
  - update: `waitForDeactivate` takes `(appQName, wsid, name)` and polls via `vit.PostApp` with a system token issued for `appQName`, so it works for cross-app cascades (profile WS in `test1/app1` triggers cascade to `sys/registry`)
  - add: `TestDeactivateUserProfile` runs the full production cascade — `c.sys.InitiateDeactivateWorkspace` on the user profile WS, `waitForDeactivate` until `Status=Inactive`, then asserts:
    - every login-touching registry function behaves as if the login does not exist (and never leaks 410 Gone), one subtest per function:
      - `q.registry.IssuePrincipalToken` -> HTTP 401 `login or password is incorrect`
      - `c.registry.ChangePassword` -> HTTP 401 `login {x} does not exist`
      - `q.registry.InitiateResetPasswordByEmail` -> HTTP 400 `login does not exist`
      - `c.registry.ResetPasswordByEmail` -> HTTP 401 `login {x} does not exist` (verified value token is captured before deactivation since `Email` is a verified field)
      - `c.registry.UpdateGlobalRoles` -> HTTP 401 `login {x} does not exist`
    - the "deactivated login treated as missing" `logger.Verbose` line is emitted in every registry subtest carrying the resolved `cdocLoginID`
      - capture via `logger.StartCapture(t, logger.LogLevelVerbose)`; reset the capture at the start of each registry subtest and assert with `EventuallyHasLine` at the end so the verification is isolated per subtest (VVM goroutines emit the line asynchronously)
- [x] update: [pkg/processors/query/impl.go](../../../../../pkg/processors/query/impl.go)
  - improve diagnostic of request body unmarshal failure in the query processor pipeline: replace `coreutils.WrapSysError(err, 400)` with `coreutils.NewHTTPErrorf(400, fmt.Errorf("failed to unmarshal request body: %w", err))` so the error surfaced to the client identifies the failing stage
- [x] Review
