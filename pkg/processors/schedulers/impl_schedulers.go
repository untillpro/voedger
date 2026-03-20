/*
  - Copyright (c) 2024-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/
package schedulers

import (
	"context"
	"fmt"
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/goutils/logger"
	retrier "github.com/voedger/voedger/pkg/goutils/retry"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/istructs"
)

/*
implements:

	appparts.ISchedulerRunner
*/
type schedulers struct {
	cfg      BasicSchedulerConfig
	wait     sync.WaitGroup
	appParts appparts.IAppPartitions

	// Need to fine schedulers control in tests
	// In tests this time differs from testingu.MockTime and controlled via ISchedulerRunner.SchedulersTime()
	time timeu.ITime
}

func newSchedulers(cfg BasicSchedulerConfig) *schedulers {
	s := &schedulers{
		cfg:  cfg,
		time: cfg.Time,
	}
	// If cfg.Time supports NewIsolatedTime (e.g., testingu.MockTime in tests),
	// use it to create an isolated time for schedulers
	if itp, ok := cfg.Time.(interface{ NewIsolatedTime() timeu.ITime }); ok {
		s.time = itp.NewIsolatedTime()
		s.cfg.Time = s.time
	}
	return s
}

// Creates and runs new actualizer for specified partition.
//
// # apparts.IActualizerRunner.NewAndRun
func (a *schedulers) NewAndRun(ctx context.Context, app appdef.AppQName, partition istructs.PartitionID, appWSIdx istructs.AppWorkspaceNumber, wsid istructs.WSID, job appdef.QName) {
	logCtx := logger.WithContextAttrs(ctx, map[string]any{
		logger.LogAttr_VApp:      app,
		logger.LogAttr_Extension: fmt.Sprintf("job.%s", job),
		logger.LogAttr_WSID:      wsid,
	})
	act := &scheduler{
		job:    job,
		logCtx: logCtx,
		conf: SchedulerConfig{
			BasicSchedulerConfig: a.cfg,
			AppQName:             app,
			Partition:            partition,
			Workspace:            wsid,
			AppWSIdx:             appWSIdx,
		},
		appParts:   a.appParts,
		retrierCfg: retrier.NewConfig(schedulerRetryDelay, schedulerRetryDelay),
	}
	act.Prepare()

	a.wait.Add(1)
	act.Run(ctx)
	a.wait.Done()
}

func (a *schedulers) SetAppPartitions(ap appparts.IAppPartitions) {
	a.appParts = ap
}

// SchedulersTime returns the isolated time used by schedulers.
// In tests with MockTime, this is an isolated time instance that can be
// advanced independently from the global MockTime.
func (a *schedulers) SchedulersTime() timeu.ITime {
	return a.time
}
