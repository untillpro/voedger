---
registered_at: 2026-03-11T13:31:14Z
change_id: 2603111331-special-chars-authnz-paths
baseline: 0d7582b825ebbb003ffb61ded265d9e8fe12b84d
archived_at: 2026-03-11T18:35:21Z
---

# Change request: Special chars in authnz paths of API v2

## Why

Special characters (e.g. `"`, `\`) can be provided in user-supplied fields such as login, password, or display name. When these values are embedded into JSON strings for internal API calls they are formatted with `fmt.Sprintf("%s", ...)` without JSON escaping, causing the resulting JSON to be malformed.

## What

Properly escape user-supplied string values before embedding them into JSON arguments for internal API calls. The following handlers are affected:

- `auth/login` (`pkg/processors/query2/impl_auth_login_handler.go`): `login` and `password` are embedded unescaped into the args JSON passed to `registry.IssuePrincipalToken`
- `users/change-password` (`pkg/router/impl_apiv2.go`): `login`, `oldPassword`, and `newPassword` are embedded unescaped into the body JSON passed to `registry.ChangePassword`
- `users` create (`pkg/router/impl_apiv2.go`): `pwd` is embedded unescaped into the body JSON passed to `registry.CreateEmailLogin`
