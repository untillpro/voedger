/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Maxim Geraskin
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package logger_test

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/testingu"
)

func Test_BasicUsage(t *testing.T) {

	// "Hello world"
	{
		logger.Error("Error", "arg1", "arg2")
		logger.Warning("My warning")
		logger.Info("My info")

		// IsVerbose() is used to avoid unnecessary calculations
		if logger.IsVerbose() {
			logger.Verbose("!!! You should NOT see this verbose message since default level is INFO")
		}

		// IsTrace() is used to avoid unnecessary calculations
		if logger.IsTrace() {
			logger.Trace("!!! You should NOT see this trace message since default level is INFO")
		}
	}

	// Changing LogLevel
	{
		logger.SetLogLevel(logger.LogLevelTrace)
		if logger.IsTrace() {
			logger.Trace("Now you should see my Trace")
		}
		if logger.IsVerbose() {
			logger.Verbose("Now you should see my Verbose")
		}
		logger.SetLogLevel(logger.LogLevelError)
		logger.Trace("!!! You should NOT see my Trace")
		logger.Warning("!!! You should NOT see my warning")
		logger.SetLogLevel(logger.LogLevelInfo)
		logger.Warning("You should see my warning")
		logger.Warning("You should see my info")
	}
}

func Test_BasicUsage_SetLogLevelWithRestore(t *testing.T) {

	// Log level is set to LogLevelTrace and then restored to the previous value
	defer logger.SetLogLevelWithRestore(logger.LogLevelTrace)()

	logger.Trace("You SHOULD see this trace")

}

func Test_SetLogLevelWithRestore(t *testing.T) {

	trySetLevelWithRestore := func() {
		defer logger.SetLogLevelWithRestore(logger.LogLevelTrace)()
		logger.Trace("You SHOULD see this trace")
	}
	trySetLevelWithRestore()
	logger.Trace("You should NOT see this trace")
}

func loggerHelperWithSkipStackFrames(skipStackFrames int, msg string) error {
	logger.Log(skipStackFrames, logger.LogLevelTrace, "myStunningPrefix:", msg)
	return nil
}

func Test_BasicUsage_SkipStackFrames(t *testing.T) {

	logger.SetLogLevel(logger.LogLevelTrace)

	// [logger_test.loggerHelperWithSkipStackFrames:...]: myStunningPrefix: hello
	_ = loggerHelperWithSkipStackFrames(0, "hello")

	// logger_test.Test_SkipStackFrames:...]: myStunningPrefix: hello
	_ = loggerHelperWithSkipStackFrames(1, "hello")
}

func Test_BasicUsage_CustomPrintLine(t *testing.T) {

	require := require.New(t)

	// Define myPrintLine
	myPrintLine := func(level logger.TLogLevel, line string) {
		line += "myPrintLine"
		logger.DefaultPrintLine(level, line)
	}

	// Use myPrintLine as logger.PrintLine

	logger.PrintLine = myPrintLine
	defer func() {
		logger.PrintLine = logger.DefaultPrintLine
	}()

	{
		logger.SetLogLevel(logger.LogLevelTrace)
		strStdout, strStderr, err := testingu.CaptureStdoutStderr(func() error {
			logger.Trace("hello")
			return nil
		})
		require.NoError(err)
		require.Empty(strStderr)
		require.Contains(strStdout, "myPrintLine")
	}

}

func Test_SkipStackFrames(t *testing.T) {

	require := require.New(t)
	logger.SetLogLevel(logger.LogLevelTrace)

	const funcNamePattern = "loggerHelperWithSkipStackFrames"

	{
		// [logger_test.loggerHelperSkip0StackFrames:69]: myStunningPrefix: hello
		strStdout, strStderr, err := testingu.CaptureStdoutStderr(func() error {
			return loggerHelperWithSkipStackFrames(0, "hello")
		})
		require.NoError(err)
		require.Empty(strStderr)
		require.Contains(strStdout, funcNamePattern)
	}

	{
		// logger_test.Test_SkipStackFrames:80]: myStunningPrefix: hello
		strStdout, strStderr, err := testingu.CaptureStdoutStderr(func() error {
			return loggerHelperWithSkipStackFrames(1, "hello")
		})
		require.NoError(err)
		require.Empty(strStderr)
		require.NotContains(strStdout, funcNamePattern)
	}

}

func Test_StdoutStderr_LogLevel(t *testing.T) {

	require := require.New(t)

	// LogLevelError
	{
		logger.SetLogLevel(logger.LogLevelError)
		strStdout, strStderr, err := testingu.CaptureStdoutStderr(func() error {
			logger.Error("Error", "arg1", "arg2")
			logger.Warning("My warning")
			return nil
		})
		require.NoError(err)
		require.Contains(strStderr, "Error arg1 arg2")
		require.Empty(strStdout)
	}

	// LogLevelWarning
	{
		logger.SetLogLevel(logger.LogLevelWarning)
		strStdout, strStderr, err := testingu.CaptureStdoutStderr(func() error {
			logger.Error("Error", "arg1", "arg2")
			logger.Warning("My warning")
			return nil
		})
		require.NoError(err)
		require.Contains(strStderr, "Error arg1 arg2")
		require.Contains(strStdout, "My warning")
	}

}

func Test_CheckSetLevels(t *testing.T) {

	require := require.New(t)

	defer logger.SetLogLevelWithRestore(logger.LogLevelNone)()
	require.False(logger.IsError())
	require.False(logger.IsWarning())
	require.False(logger.IsInfo())
	require.False(logger.IsVerbose())
	require.False(logger.IsTrace())

	defer logger.SetLogLevelWithRestore(logger.LogLevelError)()
	require.True(logger.IsError())
	require.False(logger.IsWarning())
	require.False(logger.IsInfo())
	require.False(logger.IsVerbose())
	require.False(logger.IsTrace())

	defer logger.SetLogLevelWithRestore(logger.LogLevelWarning)()
	require.True(logger.IsError())
	require.True(logger.IsWarning())
	require.False(logger.IsInfo())
	require.False(logger.IsVerbose())
	require.False(logger.IsTrace())

	defer logger.SetLogLevelWithRestore(logger.LogLevelInfo)()
	require.True(logger.IsError())
	require.True(logger.IsWarning())
	require.True(logger.IsInfo())
	require.False(logger.IsVerbose())
	require.False(logger.IsTrace())

	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()
	require.True(logger.IsError())
	require.True(logger.IsWarning())
	require.True(logger.IsInfo())
	require.True(logger.IsVerbose())
	require.False(logger.IsTrace())

	defer logger.SetLogLevelWithRestore(logger.LogLevelTrace)()
	require.True(logger.IsError())
	require.True(logger.IsWarning())
	require.True(logger.IsInfo())
	require.True(logger.IsVerbose())
	require.True(logger.IsTrace())

}

func TestLoggerCtx_BasicUsage(t *testing.T) {
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()
	ctx := logger.WithContextAttrs(context.Background(), map[string]any{
		logger.LogAttr_VApp:      "untill.fiscalcloud",
		logger.LogAttr_Feat:      "magicmenu",
		logger.LogAttr_ReqID:     42,
		logger.LogAttr_WSID:      100,
		logger.LogAttr_Extension: "c.sys.UploadBLOBHelper",
	})
	logger.InfoCtx(ctx, "hello ctx")
	logger.VerboseCtx(ctx, "hello ctx")
	logger.ErrorCtx(ctx, "hello ctx")
}

// captureCtxOutput captures output produced by *Ctx logging functions.
// It redirects slog writers to the captured pipes for the duration of f.
func captureCtxOutput(f func()) (stdout, stderr string) {
	stdout, stderr, err := testingu.CaptureStdoutStderr(func() error {
		logger.SetCtxWriters(os.Stdout, os.Stderr)
		f()
		return nil
	})
	if err != nil {
		// notest
		panic(err)
	}
	logger.SetCtxWriters(os.Stdout, os.Stderr)
	return
}

func Test_WithContextAttrs(t *testing.T) {
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()

	t.Run("attrs appear in stdout", func(t *testing.T) {
		require := require.New(t)
		ctx := logger.WithContextAttrs(context.Background(), map[string]any{
			logger.LogAttr_VApp: "untill.fiscalcloud",
			logger.LogAttr_Feat: "magicmenu",
		})
		stdout, stderr := captureCtxOutput(func() {
			logger.VerboseCtx(ctx, "hello ctx")
		})
		require.Empty(stderr)
		require.Contains(stdout, "hello ctx")
		require.Contains(stdout, "vapp=untill.fiscalcloud")
		require.Contains(stdout, "feat=magicmenu")
	})

	t.Run("attrs accumulate across WithContextAttrs calls", func(t *testing.T) {
		require := require.New(t)
		ctx := logger.WithContextAttrs(context.Background(), map[string]any{logger.LogAttr_VApp: "myapp"})
		ctx = logger.WithContextAttrs(ctx, map[string]any{logger.LogAttr_Feat: "myfeat"})
		stdout, _ := captureCtxOutput(func() {
			logger.VerboseCtx(ctx, "accumulated")
		})
		require.Contains(stdout, "vapp=myapp")
		require.Contains(stdout, "feat=myfeat")
	})

	t.Run("same key is overwritten", func(t *testing.T) {
		require := require.New(t)
		ctx := logger.WithContextAttrs(context.Background(), map[string]any{logger.LogAttr_VApp: "first"})
		ctx = logger.WithContextAttrs(ctx, map[string]any{logger.LogAttr_VApp: "second"})
		stdout, _ := captureCtxOutput(func() {
			logger.VerboseCtx(ctx, "overwrite")
		})
		require.Contains(stdout, "vapp=second")
		require.NotContains(stdout, "vapp=first")
	})
}

func Test_CtxFuncs_StandardAttrs(t *testing.T) {
	require := require.New(t)
	defer logger.SetLogLevelWithRestore(logger.LogLevelTrace)()

	ctx := logger.WithContextAttrs(context.Background(), map[string]any{
		logger.LogAttr_ReqID:     42,
		logger.LogAttr_WSID:      100,
		logger.LogAttr_Extension: "c.sys.UploadBLOBHelper",
	})

	stdout, _ := captureCtxOutput(func() {
		logger.InfoCtx(ctx, "standard attrs")
	})
	require.Contains(stdout, "reqid=42")
	require.Contains(stdout, "wsid=100")
	require.Contains(stdout, "extension=c.sys.UploadBLOBHelper")
}

func Test_CtxFuncs_SLogLevels(t *testing.T) {
	testCases := []struct {
		name       string
		level      logger.TLogLevel
		logFn      func(context.Context, ...interface{})
		msg        string
		wantLevel  string
		wantStdErr bool
	}{
		{name: "ErrorCtx", level: logger.LogLevelError, logFn: logger.ErrorCtx, msg: "error msg", wantLevel: "ERROR", wantStdErr: true},
		{name: "WarningCtx", level: logger.LogLevelWarning, logFn: logger.WarningCtx, msg: "warning msg", wantLevel: "WARN"},
		{name: "InfoCtx", level: logger.LogLevelInfo, logFn: logger.InfoCtx, msg: "info msg", wantLevel: "INFO"},
		{name: "VerboseCtx", level: logger.LogLevelVerbose, logFn: logger.VerboseCtx, msg: "verbose msg", wantLevel: "VERBOSE"},
		{name: "TraceCtx", level: logger.LogLevelTrace, logFn: logger.TraceCtx, msg: "trace msg", wantLevel: "TRACE"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require := require.New(t)
			defer logger.SetLogLevelWithRestore(tc.level)()

			stdout, stderr := captureCtxOutput(func() {
				tc.logFn(context.Background(), tc.msg)
			})

			check := func(shouldBeEmpty, shouldBeNotEmpty string) {
				require.Empty(shouldBeEmpty)
				require.Contains(shouldBeNotEmpty, "level="+tc.wantLevel)
				require.Contains(shouldBeNotEmpty, "msg=\""+tc.msg+"\"")
			}

			if tc.wantStdErr {
				check(stdout, stderr)
			} else {
				check(stderr, stdout)
			}
		})
	}
}

func Test_CtxFuncs_LevelFiltering(t *testing.T) {
	ctx := context.Background()

	t.Run("VerboseCtx suppressed at Info level", func(t *testing.T) {
		require := require.New(t)
		defer logger.SetLogLevelWithRestore(logger.LogLevelInfo)()
		stdout, stderr, err := testingu.CaptureStdoutStderr(func() error {
			logger.VerboseCtx(ctx, "should not appear")
			return nil
		})
		require.NoError(err)
		require.Empty(stdout)
		require.Empty(stderr)
	})

	t.Run("TraceCtx suppressed at Info level", func(t *testing.T) {
		require := require.New(t)
		logger.SetLogLevel(logger.LogLevelInfo)
		defer logger.SetLogLevel(logger.LogLevelInfo)
		stdout, stderr, err := testingu.CaptureStdoutStderr(func() error {
			logger.TraceCtx(ctx, "should not appear")
			return nil
		})
		require.NoError(err)
		require.Empty(stdout)
		require.Empty(stderr)
	})

	t.Run("ErrorCtx goes to stderr", func(t *testing.T) {
		require := require.New(t)
		logger.SetLogLevel(logger.LogLevelError)
		defer logger.SetLogLevel(logger.LogLevelInfo)
		ctx2 := logger.WithContextAttrs(ctx, map[string]any{"k": "v"})
		stdout, stderr := captureCtxOutput(func() {
			logger.ErrorCtx(ctx2, "boom")
		})
		require.Empty(stdout)
		require.Contains(stderr, "boom")
		require.Contains(stderr, "k=v")
	})

	t.Run("WarningCtx visible at Warning level", func(t *testing.T) {
		require := require.New(t)
		logger.SetLogLevel(logger.LogLevelWarning)
		defer logger.SetLogLevel(logger.LogLevelInfo)
		stdout, _ := captureCtxOutput(func() {
			logger.WarningCtx(ctx, "warn msg")
		})
		require.Contains(stdout, "warn msg")
	})
}

func Test_CtxFuncs_EmptyContext(t *testing.T) {
	require := require.New(t)
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()

	stdout, _ := captureCtxOutput(func() {
		logger.VerboseCtx(context.Background(), "no attrs")
	})
	require.Contains(stdout, "no attrs")
}

func TestMultithread(t *testing.T) {
	toLog := []string{}
	for i := 0; i < 100; i++ {
		toLog = append(toLog, strings.Repeat(strconv.Itoa(i), 10))
	}

	wg := sync.WaitGroup{}
	wg.Add(1000)

	stdout, _ := captureCtxOutput(func() {
		for i := 0; i < 1000; i++ {
			go func() {
				for i := 0; i < 100; i++ {
					logger.Info(toLog[i])
				}
				wg.Done()
			}()
		}
		wg.Wait()
	})

	logged := strings.Split(stdout, "\n")
outer:
	for _, loggedActual := range logged {
		if len(loggedActual) == 0 {
			continue
		}
		for _, loggedExpected := range toLog {
			if strings.Contains(loggedActual, loggedExpected) {
				continue outer
			}
		}
		t.Fatal(loggedActual)
	}
}
