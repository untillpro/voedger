# AIR-3910: pipeline: SwitchOperator: fix wrong struct field assignment in func with non-pointer receiver

- **Key:** AIR-3910
- **Type:** Sub-task
- **Status:** In Progress
- **Assignee:** d.gribanov@dev.untill.com
- **URL:** https://untill.atlassian.net/browse/AIR-3910

## Why

[pkg/pipeline/switch-operator-impl.go#L21](https://github.com/voedger/voedger/blob/816987454e5ee9cd6c094bb07fd41cdffe850db9/pkg/pipeline/switch-operator-impl.go#L21):

```go
func (s switchOperator) DoSync(ctx context.Context, work IWorkpiece) (err error) {
    s.currentBranchName, err = s.switchLogic.Switch(work) // <-- error here, switchOperator.currentBranchName accessed wrong
    if err != nil {
        return err
    }
    return s.branches[s.currentBranchName].DoSync(ctx, work)
}
```

## What

Use a local var instead of the `currentBranchName` field.
