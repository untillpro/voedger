# AIR-3801: sqlquery: support int8 and int16 to make possible to read from view.air.FdmLog

- Key: AIR-3801
- Type: Task
- Status: In Progress
- Assignee: d.gribanov@dev.untill.com
- URL: <https://untill.atlassian.net/browse/AIR-3801>

## Why

select * from air.FdmLog where year = 2026 and month = 5 and day = 1 causes unsupported data kinderror

## What

support int8 and in16 data kinds in where clauses in sql query
