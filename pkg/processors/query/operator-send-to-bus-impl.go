/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package queryprocessor

import (
	"context"
	"net/http"
	"time"

	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/pipeline"
)

type SendToBusOperator struct {
	pipeline.AsyncNOOP
	responseWriter bus.IResponseWriter
	responder      bus.IResponder
	metrics        IMetrics
	errCh          chan<- error
}

func (o *SendToBusOperator) DoAsync(_ context.Context, work pipeline.IWorkpiece) (outWork pipeline.IWorkpiece, err error) {
	begin := time.Now()
	defer func() {
		o.metrics.Increase(Metric_ExecSendSeconds, time.Since(begin).Seconds())
	}()
	if o.responseWriter == nil {
		o.responseWriter = o.responder.StreamJSON(http.StatusOK)
	}
	return work, o.responseWriter.Write(work.(rowsWorkpiece).OutputRow().Values())
}

func (o *SendToBusOperator) OnError(_ context.Context, err error) {
	select {
	case o.errCh <- err:
	default:
	}
}
