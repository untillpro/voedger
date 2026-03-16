/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package logger

import (
	"log/slog"
	"os"
)

const (
	logCtxSkipFrames = 3
)

const (
	LogAttr_VApp      = "vapp"
	LogAttr_Feat      = "feat"
	LogAttr_ReqID     = "reqid"
	LogAttr_WSID      = "wsid"
	LogAttr_Extension = "extension"
)

var (
	// ctxHandlerOpts disables handler-level filtering (isEnabled() already gates all calls).
	ctxHandlerOpts = &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	slogOut = slog.New(slog.NewTextHandler(os.Stdout, ctxHandlerOpts))
	slogErr = slog.New(slog.NewTextHandler(os.Stderr, ctxHandlerOpts))
)
