/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package projectors

import (
	"context"
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

type (
	// actualizers is a set of actualizers for application partitions.
	//
	// # Implements:
	//	- IActualizersService:
	//	   + pipeline.IService
	//	   + appparts.IActualizerFactory
	actualizers struct {
		cfg  BasicAsyncActualizerConfig
		wait sync.WaitGroup
	}
)

func newActualizers(cfg BasicAsyncActualizerConfig) *actualizers {
	return &actualizers{
		cfg:  cfg,
		wait: sync.WaitGroup{},
	}
}

// Creates and runs new actualizer for specified partition.
//
// # apparts.IActualizerRunner.NewAndRun
func (a *actualizers) NewAndRun(ctx context.Context, app appdef.AppQName, part istructs.PartitionID, prj appdef.QName) {
	act := &asyncActualizer{
		projector: prj,
		conf: AsyncActualizerConf{
			BasicAsyncActualizerConfig: a.cfg,
			AppQName:                   app,
			Partition:                  part,
		},
	}
	act.conf.Ctx = ctx
	act.Prepare()

	a.wait.Add(1)
	act.Run(ctx)
	a.wait.Done()
}

// # pipeline.IService.Prepare
func (*actualizers) Prepare(interface{}) error { return nil }

// # pipeline.IService.Run
func (*actualizers) Run(context.Context) {
	panic("not implemented")
}

// # pipeline.IServiceEx.RunEx
func (a *actualizers) RunEx(ctx context.Context, started func()) {
	a.cfg.Ctx = ctx
	started()
}

func (a *actualizers) Stop() {
	// Cancellation has already been sent to the context by caller.
	// Here we are just waiting while all async actualizers are stopped
	a.wait.Wait()
}
