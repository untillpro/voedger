# Implementation plan: Sqlquery deny duplicate field selection

## Functional design

- [x] update: [apps/vsql-blob-read.feature](../../specs/prod/apps/vsql-blob-read.feature)
  - add: Edge case scenario — duplicate `blobinfo(field)` call rejected with error
  - add: Edge case scenario — duplicate `blobtext(field)` call rejected with error

## Construction

- [x] update: [pkg/sys/sqlquery/impl.go](../../../pkg/sys/sqlquery/impl.go)
  - add: Duplicate output-field detection in the SELECT clause loop: track seen output field names; for a regular column the key is the field name, for a blob function the key is `funcname(fieldname)`; return HTTP 400 error `"field '%s' is selected more than once"` on first duplicate found

- [x] update: [pkg/sys/it/impl_sqlquery_test.go](../../../pkg/sys/it/impl_sqlquery_test.go)
  - add: Test cases inside `TestBlobFunctionsErrors` for `select blobinfo(Blob), blobinfo(Blob)` and `select blobtext(Blob), blobtext(Blob)` — expect error `"field 'blobinfo(Blob)' is selected more than once"` and `"field 'blobtext(Blob)' is selected more than once"`
  - add: Test case for duplicate regular field selection e.g. `select name, name from app1pkg.payments where id = X` — expect error `"field 'name' is selected more than once"`
