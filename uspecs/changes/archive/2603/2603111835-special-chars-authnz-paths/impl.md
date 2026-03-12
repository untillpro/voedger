# Implementation plan: Special chars in authnz paths of API v2

## Construction

- [x] update: [pkg/processors/query2/impl_auth_login_handler.go](../../../../../pkg/processors/query2/impl_auth_login_handler.go)
  - fix: use `encoding/json` marshaling instead of `fmt.Sprintf` when building the JSON args for `registry.IssuePrincipalToken`; `login` and `password` must be properly escaped
- [x] update: [pkg/router/impl_apiv2.go](../../../../../pkg/router/impl_apiv2.go)
  - fix: in `requestHandlerV2_changePassword` — escape `login`, `oldPassword`, `newPassword` before embedding into the body JSON for `registry.ChangePassword`
  - fix: in `requestHandlerV2_create_user` — escape `pwd` before embedding into the body JSON for `registry.CreateEmailLogin`
- [x] update: [pkg/vit/utils.go](../../../../../pkg/vit/utils.go)
  - fix: `signUp` — use `json.Marshal` instead of `fmt.Sprintf` for the request body so that special characters in `password` and `displayName` are properly escaped
  - fix: `SignIn` — use `json.Marshal` instead of `fmt.Sprintf` for the request body so that special characters in `login` and `password` are properly escaped
- [x] update: [pkg/sys/it/impl_qpv2_test.go](../../../../../pkg/sys/it/impl_qpv2_test.go)
  - add: subtest in `TestQueryProcessor2_AuthLogin` covering login and password containing JSON special characters (e.g. `"`, `\`)
- [x] update: [pkg/sys/it/impl_changepassword_test.go](../../../../../pkg/sys/it/impl_changepassword_test.go)
  - add: subtest in `TestBasicUsage_ChangePassword_APIv2` covering oldPassword/newPassword containing JSON special characters
- [x] update: [pkg/sys/it/impl_signupin_test.go](../../../../../pkg/sys/it/impl_signupin_test.go)
  - add: subtest in `TestBasicUsage_SignUpIn` covering password containing JSON special characters (exercises `requestHandlerV2_create_user` via `vit.SignUp`)
