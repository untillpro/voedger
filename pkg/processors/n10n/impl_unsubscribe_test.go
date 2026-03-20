/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package n10n

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
)

func TestUnsubscribeLogging(t *testing.T) {
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()
	var buf syncBuf
	logger.SetCtxWriters(&buf, &buf)
	defer logger.SetCtxWriters(os.Stdout, os.Stderr)

	require := require.New(t)
	buf.Reset()

	projKey := in10n.ProjectionKey{
		App:        appdef.NewAppQName("test", "app"),
		Projection: appdef.NewQName("test", "View"),
		WS:         istructs.WSID(42),
	}
	wp := newN10nWP("chan-unsub-1", projKey)

	require.NoError(logUnsubscribeSuccess(context.Background(), wp))

	out := buf.String()
	require.Contains(out, "stage=n10n.unsubscribe.success")
	require.Contains(out, "projectionkey=")
	require.Contains(out, "channelid=chan-unsub-1")
}
