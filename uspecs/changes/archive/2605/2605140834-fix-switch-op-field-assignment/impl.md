# Implementation plan: Fix wrong struct field assignment in switchOperator

## Construction

- [x] update: [pkg/pipeline/switch-operator-impl.go](../../../../../pkg/pipeline/switch-operator-impl.go)
  - remove: `currentBranchName` field from `switchOperator` struct
  - update: `DoSync` to use a local variable instead of `s.currentBranchName`
- [x] update: [pkg/pipeline/core-pipeline-design.archimate](../../../../../pkg/pipeline/core-pipeline-design.archimate)
  - remove: `currentBranchName` Artifact element and its inbound AccessRelationships and DiagramObject references
