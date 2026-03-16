# Logger

Structured logging with hierarchical levels, automatic caller
information, and context-aware attribute propagation. Provides
thread-safe logging with customizable output formatting and
level-based filtering.

## Problem

Go's standard `log` package lacks level-based filtering, caller
tracking, and context attribute propagation, forcing developers
to implement repetitive logging infrastructure manually.

<details>
<summary>Without logger</summary>

```go
// Manual level checking and formatting - boilerplate everywhere
var logLevel int = 3 // Info level
var mu sync.Mutex

func logError(args ...interface{}) {
    if logLevel >= 1 { // boilerplate: manual level check
        mu.Lock() // boilerplate: manual thread safety
        defer mu.Unlock()
        // boilerplate: manual stack trace
        pc, _, line, _ := runtime.Caller(1)
        fn := runtime.FuncForPC(pc)
        funcName := "unknown"
        if fn != nil {
            funcName = fn.Name()
        }
        timestamp := time.Now().Format("01/02 15:04:05.000")
        fmt.Fprintf(os.Stderr, "%s: ERROR: [%s:%d]: %s\n",
            timestamp, funcName, line, fmt.Sprint(args...))
    }
}

// boilerplate: duplicate code for every level
func logInfo(args ...interface{}) { /* same boilerplate */ }

// Structured attributes must be threaded manually through calls
func handleRequest(reqID int64, wsID int64) {
    // boilerplate: pass ids explicitly to every log call
    logInfo("started requestID=%d wsID=%d", reqID, wsID)
    processPayment(reqID, wsID) // must thread ids everywhere
}

func processPayment(reqID int64, wsID int64) {
    // boilerplate: repeat ids in every log call
    logInfo("payment processed reqID=%d wsID=%d", reqID, wsID)
}
```

</details>

<details>
<summary>With logger</summary>

```go
import (
    "context"
    "github.com/voedger/voedger/pkg/goutils/logger"
)

func handleRequest(ctx context.Context, reqID, wsID int64) {
    ctx = logger.WithContextAttrs(ctx, map[string]any{
        logger.LogAttr_ReqID: reqID,
        logger.LogAttr_WSID:  wsID,
    })
    logger.InfoCtx(ctx, "started")  // attrs included automatically
    processPayment(ctx)             // just pass ctx
}

func processPayment(ctx context.Context) {
    logger.InfoCtx(ctx, "payment processed") // attrs still there
}
```

</details>

## Features

- **Level filtering** - Hierarchical levels (`Error`→`Trace`) with
  atomic switching and restore-on-defer
  - [Level constants: logger.go#L22](logger.go#L22)
  - [Atomic level check: impl.go#L37](impl.go#L37)
  - [Level restoration: logger.go#L36](logger.go#L36)
- **Caller tracking** - Automatic function name and line number
  in every log entry
  - [Stack frame analysis: impl.go#L42](impl.go#L42)
  - [Formatted output: impl.go#L55](impl.go#L55)
  - [Skip frame control: logger.go#L64](logger.go#L64)
- **Context-aware logging** - `*Ctx` functions read logging key/value
  pairs stored in `context.Context` and append them to each entry
  - [WithContextAttrs: loggerctx.go#L23](loggerctx.go#L23)
  - [Ctx logging functions: loggerctx.go#L31](loggerctx.go#L31)
  - [sLogAttrsFromCtx: loggerctx.go#L68](loggerctx.go#L68)
  - **Predefined attribute key constants** ([consts.go](consts.go#L17))

    | Constant            | Key         | Example value |
    | ------------------- | ----------- | ------------- |
    | `LogAttr_VApp`      | `vapp`      | `my.app`      |
    | `LogAttr_Feat`      | `feat`      | `magicmenu`   |
    | `LogAttr_ReqID`     | `reqid`     | `42`          |
    | `LogAttr_WSID`      | `wsid`      | `1001`        |
    | `LogAttr_Extension` | `extension` | `myFunc`      |

- **Output customization** - Pluggable `PrintLine` with automatic
  stderr/stdout routing per level
  - [PrintLine hook: logger.go#L88](logger.go#L88)
  - [Default routing: logger.go#L90](logger.go#L90)
- **[Performance guards](logger.go#L68)** - `IsVerbose()`,
  `IsTrace()`, etc. prevent computing expensive arguments
- **slog level mapping** - Both `Verbose` and `Trace` internal levels
  map to `slog.LevelDebug` when emitting structured log records

## Use

See [basic usage test](logger_test.go#L25)

## Example output

```text
09/29 13:29:04.355: *****: [core-logger.Test_BasicUsage:22]: Error
09/29 13:29:04.374: ===: [core-logger.Test_BasicUsage:24]: My info
time=...T14:05:26.461+03:00 level=INFO msg="started" src=myapp.handleRequest:12 reqid=42 wsid=1001
```

## Links

- [Why does the TRACE level exist, and when should I use it rather than DEBUG?](https://softwareengineering.stackexchange.com/questions/279690/why-does-the-trace-level-exist-and-when-should-i-use-it-rather-than-debug)
  - [Good answer](https://softwareengineering.stackexchange.com/questions/279690/why-does-the-trace-level-exist-and-when-should-i-use-it-rather-than-debug/360810#360810)
