/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package blobprocessor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/testingu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
)

type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (sb *syncBuf) Write(p []byte) (int, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *syncBuf) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}

type mockBlobStorage struct {
	iblobstorage.IBLOBStorage
}

func (m *mockBlobStorage) WriteBLOB(_ context.Context, _ iblobstorage.PersistentBLOBKeyType, _ iblobstorage.DescrType, _ io.Reader, _ iblobstorage.WLimiterType) (uint64, error) {
	return 100, nil
}

func okCmdSender() bus.IRequestSender {
	return bus.NewIRequestSender(testingu.MockTime, func(_ context.Context, _ bus.Request, responder bus.IResponder) {
		_ = responder.Respond(bus.ResponseMeta{StatusCode: http.StatusOK}, `{"NewIDs":{"1":42}}`)
	})
}

func newWriteBW(requestSender bus.IRequestSender) *blobWorkpiece {
	ownerQName := appdef.NewQName("test", "Owner")
	msg := &implIBLOBMessage_Write{
		implIBLOBMessage_base: implIBLOBMessage_base{
			appQName:      istructs.AppQName_test1_app1,
			wsid:          1,
			requestCtx:    context.Background(),
			header:        map[string]string{},
			requestSender: requestSender,
		},
		ownerRecord:      ownerQName,
		ownerRecordField: "BlobField",
	}
	bw := &blobWorkpiece{
		blobMessage:      msg,
		blobMessageWrite: msg,
	}
	bw.logCtx = logger.WithContextAttrs(msg.requestCtx, map[string]any{
		attrOwnerQName: ownerQName.String(),
		attrOwnerField: "BlobField",
	})
	return bw
}

func TestBlobWriteLogging(t *testing.T) {
	defer logger.SetLogLevelWithRestore(logger.LogLevelVerbose)()
	var buf syncBuf
	logger.SetCtxWriters(&buf, &buf)
	defer logger.SetCtxWriters(os.Stdout, os.Stderr)

	t.Run("bp.meta on write success", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newWriteBW(nil)
		bw.blobName = []string{"testfile.txt"}
		bw.blobContentType = []string{"text/plain"}
		bw.descr.Name = "testfile.txt"
		bw.descr.ContentType = "text/plain"

		require.NoError(validateQueryParams(context.Background(), bw))

		out := buf.String()
		require.Contains(out, "stage=bp.meta")
		require.Contains(out, "name=")
		require.Contains(out, "contenttype=")
		require.Contains(out, "ownerqname=test.Owner")
		require.Contains(out, "ownerfield=BlobField")
	})

	t.Run("bp.register.success on write success", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newWriteBW(okCmdSender())
		bw.registerFuncName = appdef.NewQName("sys", "RegisterPersistentBLOB")
		bw.registerFuncBody = `{"args":{}}`

		require.NoError(registerBLOB(context.Background(), bw))

		out := buf.String()
		require.Contains(out, "stage=bp.register.success")
		require.Contains(out, "blobid=")
		require.Contains(out, "ownerqname=test.Owner")
		require.Contains(out, "ownerfield=BlobField")
	})

	t.Run("bp.write.success on write success", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newWriteBW(nil)
		bw.blobKey = &iblobstorage.PersistentBLOBKeyType{
			ClusterAppID: istructs.ClusterAppID_sys_blobber,
			WSID:         1,
			BlobID:       42,
		}
		bw.logCtx = logger.WithContextAttrs(bw.logCtx, map[string]any{attrBlobID: "42"})

		writeFn := provideWriteBLOB(&mockBlobStorage{}, func() iblobstorage.WLimiterType { return nil })
		require.NoError(writeFn(context.Background(), bw))

		out := buf.String()
		require.Contains(out, "stage=bp.write.success")
		require.Contains(out, "blobid=42")
		require.Contains(out, "ownerqname=test.Owner")
		require.Contains(out, "ownerfield=BlobField")
	})

	t.Run("bp.setcompleted.success on write success", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newWriteBW(okCmdSender())
		bw.newBLOBID = 42
		bw.logCtx = logger.WithContextAttrs(bw.logCtx, map[string]any{attrBlobID: "42"})

		require.NoError(setBLOBStatusCompleted(context.Background(), bw))

		out := buf.String()
		require.Contains(out, "stage=bp.setcompleted.success")
		require.Contains(out, "blobid=42")
		require.Contains(out, "ownerqname=test.Owner")
		require.Contains(out, "ownerfield=BlobField")
	})

	t.Run("bp.error on write error", func(t *testing.T) {
		require := require.New(t)
		buf = syncBuf{}
		bw := newWriteBW(nil)

		s := &sendWriteResult{}
		testErr := coreutils.WrapSysError(errors.New("test write failure"), http.StatusInternalServerError)
		_ = s.OnErr(testErr, bw, nil)

		out := buf.String()
		require.Contains(out, "stage=bp.error")
		require.Contains(out, "test write failure")
	})
}
