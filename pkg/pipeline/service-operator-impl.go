// Copyright (c) 2021-present Voedger Authors.
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package pipeline

import (
	"context"
)

type serviceOperator struct {
	serviceDone chan struct{}
	iService    IService
	isStarted   bool
}

func (so *serviceOperator) Close() {
	if !so.isStarted {
		return
	}
	so.iService.Stop()
	<-so.serviceDone
}

func (so *serviceOperator) DoSync(ctx context.Context, work interface{}) (err error) {
	if err = so.iService.Prepare(work); err != nil {
		return err
	}
	so.isStarted = true
	so.serviceDone = make(chan struct{})
	go func() {
		so.iService.Run(ctx)
		close(so.serviceDone)
	}()
	return
}

func ServiceOperator(service IService) ISyncOperator {
	if service == nil {
		panic("service logic must not be nil")
	}
	return &serviceOperator{
		iService: service,
	}
}
