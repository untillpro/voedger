/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package queryprocessor

import (
	"context"
	"math"
	"time"

	pipeline "github.com/heeus/core-pipeline"
)

type CounterOperator struct {
	pipeline.AsyncNOOP
	startFrom int64
	count     int64
	counter   int64
	limiter   int64
	metrics   IMetrics
}

func newCounterOperator(startFrom, count int64, metrics IMetrics) pipeline.IAsyncOperator {
	if count == 0 {
		count = math.MaxInt
	}
	return &CounterOperator{
		startFrom: startFrom,
		count:     count,
		metrics:   metrics,
	}
}

func (o *CounterOperator) DoAsync(_ context.Context, work pipeline.IWorkpiece) (outWork pipeline.IWorkpiece, err error) {
	begin := time.Now()
	defer func() {
		o.metrics.Increase(execCountSeconds, time.Since(begin).Seconds())
	}()
	if o.counter >= o.startFrom && o.limiter < o.count {
		outWork = work
		o.limiter += 1
	}
	o.counter += 1
	if outWork == nil {
		work.Release()
	}
	return
}
