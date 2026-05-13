# AIR-3888: fix panic on TestVITResetPreservingStorage

- **Type**: Bug
- **Status**: In Progress
- **Assignee**: d.gribanov@dev.untill.com
- **URL**: <https://untill.atlassian.net/browse/AIR-3888>

## Description

Failure observed in CI: <https://github.com/voedger/voedger/actions/runs/25540670722/job/74965652880>

`TestVITResetPreservingStorage` sometimes fails with panic:

```text
panic: channel terminated

goroutine 37612 [running]:
github.com/voedger/voedger/pkg/in10nmem.(*N10nBroker).cleanupChannel(0xc00194e630, 0xc001b53c70, {0xc03122f3b0, 0x24}, 0xc03131b390)
    /home/runner/work/voedger/voedger/pkg/in10nmem/impl.go:265 +0x56a
github.com/voedger/voedger/pkg/in10nmem.(*N10nBroker).NewChannel.func1()
    /home/runner/work/voedger/voedger/pkg/in10nmem/impl.go:58 +0x69
github.com/voedger/voedger/pkg/processors/actualizers.(*asyncActualizer).finit(0xc0019c4a80)
    /home/runner/work/voedger/voedger/pkg/processors/actualizers/async.go:236 +0xaf
github.com/voedger/voedger/pkg/processors/actualizers.(*asyncActualizer).Run.func1()
    /home/runner/work/voedger/voedger/pkg/processors/actualizers/async.go:109 +0x79
github.com/voedger/voedger/pkg/goutils/retry.RetryNoResult.func1()
    /home/runner/work/voedger/voedger/pkg/goutils/retry/utils.go:33 +0x2f
github.com/voedger/voedger/pkg/goutils/retry.Retry[...].func1()
    /home/runner/work/voedger/voedger/pkg/goutils/retry/utils.go:25 +0x38
github.com/voedger/voedger/pkg/goutils/retry.(*Retrier).Run(0xc002887c38, {0x17fe8a8, 0xc001003b80}, 0xc002887c60)
    /home/runner/work/voedger/voedger/pkg/goutils/retry/impl.go:66 +0xa4
github.com/voedger/voedger/pkg/goutils/retry.Retry[...]({0x17fe8a8, 0xc001003b80}, {0x45ba94, 0x0?, 0xc001c528e0?, 0x91?}, 0xc0023e6cf8?)
    /home/runner/work/voedger/voedger/pkg/goutils/retry/utils.go:23 +0x205
github.com/voedger/voedger/pkg/goutils/retry.RetryNoResult({0x17fe8a8, 0xc001003b80}, {0xc0023e6d78?, 0x4dbd45?, 0xc001c528e0?, 0xe0?}, 0xc0023e6d58)
    /home/runner/work/voedger/voedger/pkg/goutils/retry/utils.go:32 +0xc5
github.com/voedger/voedger/pkg/processors/actualizers.(*asyncActualizer).Run(0xc0019c4a80, {0x17fe8a8, 0xc001003b80})
    /home/runner/work/voedger/voedger/pkg/processors/actualizers/async.go:104 +0xd4
github.com/voedger/voedger/pkg/processors/actualizers.(*actualizers).NewAndRun(0xc0025f6c30, {0x17fe8a8, 0xc001003b80}, {{0x1756380?, 0xbcb7c0?}, {0x175c387?, 0xc6?}}, 0x0, {{0xc0001422b0, 0x8}, ...})
    /home/runner/work/voedger/voedger/pkg/processors/actualizers/actualizers.go:53 +0x555
github.com/voedger/voedger/pkg/appparts/internal/actualizers.(*PartitionActualizers).start.func1()
    /home/runner/work/voedger/voedger/pkg/appparts/internal/actualizers/actualizers.go:76 +0x227
created by github.com/voedger/voedger/pkg/appparts/internal/actualizers.(*PartitionActualizers).start in goroutine 37496
    /home/runner/work/voedger/voedger/pkg/appparts/internal/actualizers/actualizers.go:69 +0x345
FAIL    github.com/voedger/voedger/pkg/sys/it    44.748s
```
