/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package router

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
)

func reply_v2(requestCtx context.Context, w http.ResponseWriter, responseCh <-chan any, responseErr *error, onSendFailed func(), respMeta bus.ResponseMeta) {
	sendSuccess := true
	defer func() {
		if requestCtx.Err() != nil {
			if onRequestCtxClosed != nil {
				onRequestCtxClosed()
			}
			log.Println("client disconnected during sections sending")
			return
		}
		if !sendSuccess {
			logger.ErrorCtx(requestCtx, "routing.response.error", "failed to write response")
			onSendFailed()
			for range responseCh {
			}
		}
	}()

	// ApiArray and no elems -> {"results":[]}

	if respMeta.Mode() == bus.RespondMode_StreamJSON {
		if sendSuccess = writeResponse(w, `{"results":[`); !sendSuccess {
			return
		}
	}
	elemsCount := 0
	for elem := range responseCh {
		if requestCtx.Err() != nil {
			// possible: ctx is done but on select {sections<-section, <-ctx.Done()} write to sections channel is triggered.
			// ctx.Done() must have the priority
			return
		}
		toSend := ""

		if respMeta.Mode() == bus.RespondMode_StreamJSON {
			if elemsCount > 0 {
				if sendSuccess = writeResponse(w, ","); !sendSuccess {
					return
				}
			}
			toSendBytes, err := json.Marshal(&elem)
			if err != nil {
				panic(err)
			}

			toSend = string(toSendBytes)
		} else {
			switch typed := elem.(type) {
			case nil:
				// avoiding error:
				// failed to write response: http: request method or response status code does not allow body
				if respMeta.StatusCode != http.StatusNoContent {
					toSend = "{}"
				}
			case string:
				toSend = typed
			case []byte:
				toSend = string(typed)
			case coreutils.SysError:
				toSend = typed.ToJSON_APIV2()
			default:
				elemBytes, err := json.Marshal(elem)
				if err != nil {
					// notest
					panic(err)
				}
				toSend = string(elemBytes)
			}
		}

		if sendSuccess = writeResponse(w, toSend); !sendSuccess {
			return
		}

		elemsCount++
	}

	if respMeta.Mode() == bus.RespondMode_StreamJSON {
		if sendSuccess = writeResponse(w, "]"); !sendSuccess {
			return
		}
	}

	if *responseErr != nil {
		// actual for ApiArray mode only
		if sendSuccess = writeResponse(w, ","); !sendSuccess {
			return
		}
		var sysError coreutils.SysError
		if errors.As(*responseErr, &sysError) {
			jsonErr := sysError.ToJSON_APIV2()
			sendSuccess = writeResponse(w, `"error":`+jsonErr)
		} else {
			if sendSuccess = writeResponse(w, `"error":`); sendSuccess {
				sendSuccess = writeCommonError_V2(w, (*responseErr), http.StatusInternalServerError)
			}
		}
	}

	if sendSuccess && respMeta.Mode() == bus.RespondMode_StreamJSON {
		sendSuccess = writeResponse(w, "}")
	}
}
