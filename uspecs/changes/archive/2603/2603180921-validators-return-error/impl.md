# Implementation plan: Router validators return error instead of reply

## Construction

- [x] update: [pkg/router/types.go](../../../pkg/router/types.go)
  - update: `validatorFunc` signature — remove `rw http.ResponseWriter` parameter, change return type from `(validatedData, bool)` to `(validatedData, error)`

- [x] update: [pkg/router/impl_validation.go](../../../pkg/router/impl_validation.go)
  - update: `readBody` — remove `rw http.ResponseWriter` parameter, return `error` instead of writing to response; drop logger call (moved to `withValidate`)
  - update: `cookiesTokenToHeaders` — remove `rw http.ResponseWriter` parameter, return `error` instead of calling `WriteTextResponse`
  - update: `validateRequest` — remove `rw http.ResponseWriter` parameter, return `error` instead of calling `ReplyCommonError` directly
  - update: `validate` — remove `rw http.ResponseWriter` parameter, return `error`; no longer logs or replies
  - update: `withValidate` — becomes the single place that logs and replies HTTP 400 on error from `validate`

- [x] update: [pkg/router/impl_test.go](../../../pkg/router/impl_test.go)
  - review: no changes needed — signatures are internal, existing integration tests unaffected; build confirmed clean
