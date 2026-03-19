/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package blobprocessor

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
)

type mockReadBlobStorage struct {
	iblobstorage.IBLOBStorage
	readErr error
}

func (m *mockReadBlobStorage) ReadBLOB(_ context.Context, _ iblobstorage.IBLOBKey, _ func(iblobstorage.BLOBState) error, _ io.Writer, _ iblobstorage.RLimiterType) error {
	return m.readErr
}

func newReadBW() *blobWorkpiece {
	ownerQName := appdef.NewQName("test", "Owner")
	msg := &implIBLOBMessage_Read{
		implIBLOBMessage_base: implIBLOBMessage_base{
			appQName:       istructs.AppQName_test1_app1,
			wsid:           1,
			requestCtx:     context.Background(),
			header:         map[string]string{},
			errorResponder: func(_ coreutils.SysError) {},
		},
		ownerRecord:           ownerQName,
		ownerRecordField:      "BlobField",
		ownerID:               100,
		existingBLOBIDOrSUUID: "42",
	}
	bw := &blobWorkpiece{
		blobMessage:     msg,
		blobMessageRead: msg,
	}
	bw.logCtx = logger.WithContextAttrs(msg.requestCtx, map[string]any{
		attrOwnerQName: ownerQName.String(),
		attrOwnerField: "BlobField",
		attrOwnerID:    uint64(100),
		attrBlobID:     "42",
	})
	bw.blobKey = &iblobstorage.PersistentBLOBKeyType{
		ClusterAppID: istructs.ClusterAppID_sys_blobber,
		WSID:         1,
		BlobID:       42,
	}
	bw.writer = io.Discard
	return bw
}

func TestBlobReadLogging(t *testing.T) {
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()
	var buf syncBuf
	logger.SetCtxWriters(&buf, &buf)
	defer logger.SetCtxWriters(os.Stdout, os.Stderr)

	t.Run("bp.success on read success", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newReadBW()

		readFn := provideReadBLOB(&mockReadBlobStorage{})
		require.NoError(readFn(context.Background(), bw))

		out := buf.String()
		require.Contains(out, "stage=bp.success")
		require.Contains(out, "blobid=42")
		require.Contains(out, "ownerqname=test.Owner")
		require.Contains(out, "ownerfield=BlobField")
		require.Contains(out, "ownerid=100")
	})

	t.Run("bp.error on read error", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newReadBW()

		c := &catchReadError{}
		_ = c.OnErr(errors.New("test read failure"), bw, nil)
		_ = c.DoSync(context.Background(), bw)

		out := buf.String()
		require.Contains(out, "stage=bp.error")
		require.Contains(out, "test read failure")
		require.Contains(out, "blobid=42")
	})
}

