/*
  - Copyright (c) 2024-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/
package schedulers

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/goutils/logger"
	retrier "github.com/voedger/voedger/pkg/goutils/retry"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/state/stateprovide"
)

type scheduler struct {
	name   string
	conf   SchedulerConfig
	job    appdef.QName
	logCtx context.Context
	// init:
	jobInErrAddr *imetrics.MetricValue
	schedule     cron.Schedule
	// run:
	ctx          context.Context
	projErrState int32 // 0 - no error, 1 - error
	appParts     appparts.IAppPartitions
	retrierCfg   retrier.Config
}

func (a *scheduler) Prepare() {
	if a.conf.IntentsLimit == 0 {
		a.conf.IntentsLimit = defaultIntentsLimit
	}

	a.retrierCfg.OnError = func(_ int, _ time.Duration, opErr error) (retry bool, abortErr error) {
		a.finit() // even execute if a.init has failed
		if errors.Is(opErr, appparts.ErrNotFound) {
			logger.ErrorCtx(a.logCtx, "job.error", fmt.Sprintf("appparts %s, will try again", opErr.Error()))
			return true, nil
		}
		logger.ErrorCtx(a.logCtx, "job.error", opErr)
		return false, opErr
	}
	a.name = fmt.Sprintf("%v [idx: %d, id: %d]", a.job, a.conf.AppWSIdx, a.conf.Workspace)
}

func (a *scheduler) Run(ctx context.Context) {
	a.ctx = ctx
	err := retrier.RetryNoResult(ctx, a.retrierCfg, a.init)
	if err != nil {
		// context.Canceled is only possible here
		// err is logged already by retrier OnError()
		return
	}
	a.keepRunning()
	a.finit()
}

func (a *scheduler) runJob() {
	var err error
	var borrowedPartition appparts.IAppPartition
	defer func() {
		if borrowedPartition != nil {
			borrowedPartition.Release()
		}
		if err != nil {
			logger.ErrorCtx(a.logCtx, "job.error", err)
			if atomic.CompareAndSwapInt32(&a.projErrState, 0, 1) {
				if a.jobInErrAddr != nil {
					a.jobInErrAddr.Increase(1)
				}
			}
		}
	}()
	borrowedPartition, err = a.appParts.WaitForBorrow(a.ctx, a.conf.AppQName, a.conf.Partition, appparts.ProcessorKind_Scheduler)
	if err != nil {
		return
	}
	state := stateprovide.ProvideSchedulerStateFactory()(
		a.ctx,
		func() istructs.IAppStructs { return borrowedPartition.AppStructs() },
		func() istructs.WSID { return a.conf.Workspace },
		func(view appdef.QName, wsid istructs.WSID, offset istructs.Offset) {
			a.conf.Broker.Update(in10n.ProjectionKey{
				App:        a.conf.AppQName,
				Projection: view,
				WS:         wsid,
			}, offset)
		},
		a.conf.SecretReader,
		a.conf.Tokens,
		a.conf.Federation,
		func() int64 { return a.conf.Time.Now().Unix() },
		a.conf.IntentsLimit,
		a.conf.stateOpts,
		a.conf.EmailSender,
		a.conf.HTTPClient,
	)

	if err = borrowedPartition.Invoke(a.ctx, a.job, state, state); err != nil {
		return
	}
	logger.VerboseCtx(a.logCtx, "job.success")
	err = state.ApplyIntents()
	if err != nil {
		return
	}
	if err == nil && a.jobInErrAddr != nil {
		if atomic.CompareAndSwapInt32(&a.projErrState, 1, 0) {
			a.jobInErrAddr.Increase(-1)
		}
	}
}

func (a *scheduler) init() (err error) {

	appDef, err := a.appParts.AppDef(a.conf.AppQName)
	if err != nil {
		return err
	}
	jobType := appdef.Job(appDef.Type, a.job)
	if jobType == nil {
		return fmt.Errorf("job %s is not defined in AppDef", a.job)
	}

	if a.conf.Metrics != nil {
		a.jobInErrAddr = a.conf.Metrics.AppMetricAddr(JobsInError, string(a.conf.VvmName), a.conf.AppQName)
	}

	parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	a.schedule, err = parser.Parse(jobType.CronSchedule())
	if err != nil {
		return fmt.Errorf("failed to parse cron schedule: %w", err)
	}
	return nil
}

func (a *scheduler) keepRunning() {
	now := a.conf.Time.Now()
	nextTime := a.schedule.Next(now)
	for a.ctx.Err() == nil {
		logger.VerboseCtx(a.logCtx, "job.schedule", "now=", now, ",next=", nextTime)
		timerChan := a.conf.Time.NewTimerChan(nextTime.Sub(now))
		select {
		case <-a.ctx.Done():
			return
		case now = <-timerChan:
			logger.VerboseCtx(a.logCtx, "job.wake-up", now)
			a.runJob()
			nextTime = a.schedule.Next(now)
		}
	}
}

func (a *scheduler) finit() {
	if a.jobInErrAddr != nil {
		if atomic.CompareAndSwapInt32(&a.projErrState, 1, 0) {
			a.jobInErrAddr.Increase(-1)
		}
	}
}
