/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// SetCtxWriters replaces the writers used by all *Ctx logging functions.
// Must be used in tests only: create an os.Pipe(), call SetCtxWriters, run the code, read the pipe.
func SetCtxWriters(out, err io.Writer) {
	slogOut = slog.New(slog.NewTextHandler(out, ctxHandlerOpts))
	slogErr = slog.New(slog.NewTextHandler(err, ctxHandlerOpts))
}

// WithContextAttrs returns a new context with the given attributes added. Later calls shadow earlier ones for the same key.
func WithContextAttrs(ctx context.Context, attrs map[string]any) context.Context {
	prev, _ := ctx.Value(ctxKey{}).(*logAttrs)
	return context.WithValue(ctx, ctxKey{}, &logAttrs{attrs: attrs, parent: prev})
}

// Context-aware logging functions. Each reads slog.Attr values stored in ctx
// (via WithContextAttrs) and appends them to the log record.

func VerboseCtx(ctx context.Context, stage string, args ...interface{}) {
	logCtx(ctx, LogLevelVerbose, logCtxSkipFrames, stage, args...)
}

func ErrorCtx(ctx context.Context, stage string, args ...interface{}) {
	logCtx(ctx, LogLevelError, logCtxSkipFrames, stage, args...)
}

func InfoCtx(ctx context.Context, stage string, args ...interface{}) {
	logCtx(ctx, LogLevelInfo, logCtxSkipFrames, stage, args...)
}

func WarningCtx(ctx context.Context, stage string, args ...interface{}) {
	logCtx(ctx, LogLevelWarning, logCtxSkipFrames, stage, args...)
}

func TraceCtx(ctx context.Context, stage string, args ...interface{}) {
	logCtx(ctx, LogLevelTrace, logCtxSkipFrames, stage, args...)
}

// skipStackFrames is relative to the caller
func LogCtx(ctx context.Context, skipStackFrames int, level TLogLevel, stage string, args ...interface{}) {
	logCtx(ctx, level, skipStackFrames+logCtxSkipFrames, stage, args...)
}

// logCtx is the shared implementation for all *Ctx functions.
func logCtx(ctx context.Context, level TLogLevel, skipStackFrames int, stage string, args ...interface{}) {
	if !isEnabled(level) {
		return
	}
	log := slogOut
	if level == LogLevelError {
		log = slogErr
	}
	attrsFromCtx := sLogAttrsFromCtx(ctx)
	fn, line := getFuncName(skipStackFrames)
	attrs := make([]any, 0, len(attrsFromCtx)+2)
	attrs = append(attrs, slog.Attr{Key: "src", Value: slog.StringValue(fmt.Sprintf("%s:%d", fn, line))})
	if stage != "" {
		attrs = append(attrs, slog.Attr{Key: LogAttr_Stage, Value: slog.StringValue(stage)})
	}
	attrs = append(attrs, attrsFromCtx...)
	slogLevel := loggerLevelToSLogLevel(level)
	log.Log(ctx, slogLevel, fmt.Sprint(args...), attrs...)
}

func loggerLevelToSLogLevel(level TLogLevel) slog.Level {
	switch level {
	case LogLevelError:
		return slog.LevelError
	case LogLevelWarning:
		return slog.LevelWarn
	case LogLevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelDebug
	}
}

func sLogAttrsFromCtx(ctx context.Context) []any {
	node, _ := ctx.Value(ctxKey{}).(*logAttrs)
	seen := map[string]bool{}
	attrs := []any{}
	for node != nil {
		for k := range node.attrs {
			if seen[k] {
				continue
			}
			seen[k] = true
			attrs = append(attrs, slog.Any(k, node.attrs[k]))
		}
		node = node.parent
	}
	return attrs
}
