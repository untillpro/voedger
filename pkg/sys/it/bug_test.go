/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sys_it

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/sys"
	it "github.com/voedger/voedger/pkg/vit"
)

type rr struct {
	istructs.NullObject
	res string
}

func (r *rr) AsString(string) string {
	return r.res
}

func TestBug_QueryProcessorMustStopOnClientDisconnect(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	require := require.New(t)
	goOn := make(chan interface{})
	it.MockQryExec = func(input string, _ istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
		rr := &rr{res: input}
		require.NoError(callback(rr))
		<-goOn // what for http client to receive the first element and disconnect
		// now wait for error context.Cancelled. It will be occurred immediately because an async pipeline works within queryprocessor
		for err == nil {
			err = callback(rr)
		}
		require.Equal(context.Canceled, err)
		defer func() { goOn <- nil }() // signal that context.Canceled error is caught
		return err
	}
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	// sned POST request
	body := `{"args": {"Input": "world"},"elements": [{"fields": ["Res"]}]}`
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	vit.PostWS(ws, "q.app1pkg.MockQry", body, coreutils.WithResponseHandler(func(httpResp *http.Response) {
		// read out the first part of the respoce (the serer will not send the next one before writing something in goOn)
		entireResp := []byte{}
		var err error
		n := 0
		for string(entireResp) != `{"sections":[{"type":"","elements":[[[["world"]]]` {
			if n == 0 && errors.Is(err, io.EOF) {
				t.Fatal()
			}
			buf := make([]byte, 512)
			n, err = httpResp.Body.Read(buf)
			entireResp = append(entireResp, buf[:n]...)
		}

		// break the connection during request handling
		httpResp.Request.Body.Close()
		httpResp.Body.Close()
		goOn <- nil // the func will start to send the second part. That will be failed because the request context is closed
	}))

	<-goOn // wait for error check
	// expecting that there are no additional errors: nothing hung, queryprocessor is done, router does not try to write to a closed connection etc
}

func Test409OnRepeatedlyUsedRawIDsInResultCUDs_(t *testing.T) {
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()
	it.MockCmdExec = func(_ string, args istructs.ExecCommandArgs) error {
		// the same rawID 2 times -> 500 internal server error
		kb, err := args.State.KeyBuilder(sys.Storage_Record, it.QNameApp1_CDocCategory)
		if err != nil {
			return err
		}
		sv, err := args.Intents.NewValue(kb)
		if err != nil {
			return err
		}
		sv.PutRecordID(appdef.SystemField_ID, 1)

		kb, err = args.State.KeyBuilder(sys.Storage_Record, it.QNameApp1_CDocCategory)
		if err != nil {
			return err
		}
		sv, err = args.Intents.NewValue(kb)
		if err != nil {
			return err
		}
		sv.PutRecordID(appdef.SystemField_ID, 1)
		return nil
	}
	ws := vit.WS(istructs.AppQName_test1_app1, "test_ws")
	vit.PostWS(ws, "c.app1pkg.MockCmd", `{"args":{"Input":"Str"}}`, coreutils.Expect409()).Println()
}
