# Implementation plan: Support int8 and int16 in sqlquery view WHERE clauses

## Functional design

- [x] create: [apps/vsql-view-read.feature](../../../../specs/prod/apps/vsql-view-read.feature)
  - add: VADeveloper reads view records via VSQL with WHERE filters covering int8 and int16 key fields

## Construction

- [x] update: [pkg/sys/sqlquery/impl_viewrecords.go](../../../../../pkg/sys/sqlquery/impl_viewrecords.go)
  - add: `DataKind_int8` and `DataKind_int16` fallthrough into the `kb.PutNumber` branch
- [x] update: [pkg/vit/schemaTestApp1.vsql](../../../../../pkg/vit/schemaTestApp1.vsql)
  - add: `VIEW DailyIdxSmall` with `int16` partition key and `int8` clustering keys, populated by `ApplyDailyIdx`
  - add: `GRANT SELECT ON VIEW DailyIdxSmall TO sys.WorkspaceOwner`
- [x] update: [pkg/vit/shared_cfgs.go](../../../../../pkg/vit/shared_cfgs.go)
  - add: `QNameApp1_ViewDailyIdxSmall` constant
  - extend: `ApplyDailyIdx` projector to also populate `DailyIdxSmall` with cast int8 / int16 values
- [x] update: [pkg/sys/it/impl_sqlquery_test.go](../../../../../pkg/sys/it/impl_sqlquery_test.go)
  - add: `TestSqlQuery_view_records_smallInts` integration test exercising WHERE on int8 and int16 view keys
