/*
 * Copyright (c) 2026-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package sqlquery

import (
	"context"
	"errors"
	"io"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/istructs"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
)

func TestExecuteBlobReadStopsImmediatelyWithoutContent(t *testing.T) {
	require := require.New(t)
	var handler blobprocessor.IRequestHandler = &blobReadHandlerStub{}
	var sender bus.IRequestSender

	info, text, err := executeBlobRead(
		context.Background(),
		"Blob",
		false,
		0,
		&handler,
		&sender,
		istructs.AppQName_test1_app1,
		1,
		appdef.QName{},
		1,
		map[string]string{},
	)

	require.NoError(err)
	require.Nil(text)
	stub := handler.(*blobReadHandlerStub)
	require.Equal(1, stub.limiterCalls)
	require.Zero(stub.writeCalls)
	require.Empty(stub.written)
	infoMap := info.(map[string]interface{})
	require.Equal("blob", infoMap["name"])
	require.Equal(httpu.ContentType_TextPlain, infoMap["mimetype"])
	require.EqualValues(5, infoMap["size"])
}

type blobReadHandlerStub struct {
	limiterCalls int
	writeCalls   int
	written      []byte
}

func (s *blobReadHandlerStub) HandleRead(_ appdef.AppQName, _ istructs.WSID, _ map[string]string, _ context.Context,
	_ func(headersKeyValue ...string) io.Writer, _ blobprocessor.ErrorResponder, _ string, _ bus.IRequestSender, _ iblobstorage.RLimiterType) bool {
	panic("unexpected call")
}

func (s *blobReadHandlerStub) HandleWrite(_ appdef.AppQName, _ istructs.WSID, _ map[string]string, _ context.Context,
	_ url.Values, _ func(headersKeyValue ...string) io.Writer, _ io.ReadCloser, _ blobprocessor.ErrorResponder, _ bus.IRequestSender) bool {
	panic("unexpected call")
}

func (s *blobReadHandlerStub) HandleWrite_V2(_ appdef.AppQName, _ istructs.WSID, _ map[string]string, _ context.Context,
	_ func(headersKeyValue ...string) io.Writer, _ io.ReadCloser, _ blobprocessor.ErrorResponder, _ bus.IRequestSender, _ appdef.QName, _ string) bool {
	panic("unexpected call")
}

func (s *blobReadHandlerStub) HandleWriteTemp_V2(_ appdef.AppQName, _ istructs.WSID, _ map[string]string, _ context.Context,
	_ func(headersKeyValue ...string) io.Writer, _ io.ReadCloser, _ blobprocessor.ErrorResponder, _ bus.IRequestSender) bool {
	panic("unexpected call")
}

func (s *blobReadHandlerStub) HandleRead_V2(_ appdef.AppQName, _ istructs.WSID, _ map[string]string, _ context.Context,
	okResponseIniter func(headersKeyValue ...string) io.Writer, _ blobprocessor.ErrorResponder,
	_ appdef.QName, _ string, _ istructs.RecordID, _ bus.IRequestSender, rLimiter iblobstorage.RLimiterType) bool {
	writer := okResponseIniter(
		coreutils.BlobName, "blob",
		httpu.ContentType, httpu.ContentType_TextPlain,
		httpu.ContentLength, "5",
	)
	if rLimiter != nil {
		s.limiterCalls++
		if err := rLimiter(5); err != nil {
			if errors.Is(err, iblobstorage.ErrReadLimitReached) {
				return true
			}
			panic(err)
		}
	}
	_, _ = writer.Write([]byte("hello"))
	s.writeCalls++
	s.written = append(s.written, []byte("hello")...)
	return true
}

func (s *blobReadHandlerStub) HandleReadTemp_V2(_ appdef.AppQName, _ istructs.WSID, _ map[string]string, _ context.Context,
	_ func(headersKeyValue ...string) io.Writer, _ blobprocessor.ErrorResponder, _ bus.IRequestSender, _ iblobstorage.SUUID, _ iblobstorage.RLimiterType) bool {
	panic("unexpected call")
}
