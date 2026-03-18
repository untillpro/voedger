/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Alisher Nurmanov
 */
package ielections

import (
	"context"
	"sync"
	"time"

	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/sys"
)

// [~server.design.orch/elections~impl]

// AcquireLeadership returns nil if leadership is *not* acquired (e.g., error in storage,
// already local leader, or elections cleaned up), otherwise returns a *non-nil* context.
func (e *elections[K, V]) AcquireLeadership(key K, val V, leadershipDurationSeconds LeadershipDurationSeconds) context.Context {
	logCtx := leadershipLogCtx(key)

	if e.isFinalized.Load() {
		logger.WarningCtx(logCtx, "leadership.acquire.finalized", "elections cleaned up; cannot acquire leadership")
		return nil
	}

	inserted, err := e.storage.InsertIfNotExist(key, val, int(leadershipDurationSeconds))
	if err != nil {
		// notest
		logger.ErrorCtx(logCtx, "leadership.acquire.error", "InsertIfNotExist failed:", err)
		return nil
	}
	if !inserted {
		logger.InfoCtx(logCtx, "leadership.acquire.other", "leadership already acquired by someone else")
		return nil
	}

	logger.InfoCtx(logCtx, "leadership.acquire.success", "success")

	logCtxWithCancel, cancel := context.WithCancel(logCtx)
	li := &leaderInfo[K, V]{
		val:    val,
		ctx:    logCtxWithCancel,
		cancel: cancel,
	}
	e.leadership.Store(key, li)

	li.wg.Add(1)
	maintainLeadershipStarted := sync.WaitGroup{}
	maintainLeadershipStarted.Add(1)
	go e.maintainLeadership(key, val, leadershipDurationSeconds, li, &maintainLeadershipStarted)
	maintainLeadershipStarted.Wait()
	return logCtxWithCancel
}

func (e *elections[K, V]) maintainLeadership(key K, val V, leadershipDurationSeconds LeadershipDurationSeconds, li *leaderInfo[K, V], maintainLeadershipStarted *sync.WaitGroup) {
	defer li.wg.Done()

	tickerInterval := time.Duration(leadershipDurationSeconds) * time.Second / renewalsPerLeadershipDur
	ticker := e.clock.NewTimerChan(tickerInterval)
	maintainLeadershipStarted.Done()
	tickerCounter := int64(0)

	for li.ctx.Err() == nil {
		select {
		case <-li.ctx.Done():
			return
		case <-ticker:
			ticker = e.clock.NewTimerChan(tickerInterval)
			tickerCounter = bumpTickerCounter(tickerCounter, li.ctx, tickerInterval)
			if !e.renewWithRetry(li.ctx, key, val, leadershipDurationSeconds, li, tickerInterval) {
				return
			}
		}
	}
}

func (e *elections[K, V]) renewWithRetry(ctx context.Context, key K, val V, leadershipDurationSeconds LeadershipDurationSeconds, li *leaderInfo[K, V], retryDuration time.Duration) (renewed bool) {
	deadline := e.clock.NewTimerChan(retryDuration)
	for li.ctx.Err() == nil {
		ok, err := e.storage.CompareAndSwap(key, val, val, int(leadershipDurationSeconds))
		if err == nil {
			if ok {
				return true
			}
			logger.ErrorCtx(ctx, "leadership.maintain.stolen", "compareAndSwap !ok => release")
			e.releaseLeadership(ctx, key)
			return false
		}
		logger.ErrorCtx(ctx, "leadership.maintain.stgerror", "compareAndSwap error:", err)
		select {
		case <-li.ctx.Done():
			return false
		case <-deadline:
			logger.ErrorCtx(ctx, "leadership.maintain.release", "retry deadline reached, releasing. Last error:", err)
			e.releaseLeadership(ctx, key)
			return false
		case <-e.clock.NewTimerChan(time.Second):
		}
	}
	return false
}

func bumpTickerCounter(tickerCounter int64, ctx context.Context, tickerInterval time.Duration) (tickerCounterPlus1 int64) {
	tickerCounterPlus1 = tickerCounter + 1
	if tickerCounter < 10 {
		logger.VerboseCtx(ctx, "leadership.maintain.10", "renewing leadership")
	} else if tickerCounter%200 == 0 {
		// notest
		logger.VerboseCtx(ctx, "leadership.maintain.200", "still leader for", tickerInterval*time.Duration(tickerCounter))
	}
	return tickerCounterPlus1
}

func (e *elections[K, V]) ReleaseLeadership(key K) {
	li := e.releaseLeadership(leadershipLogCtx(key), key)
	if li == nil {
		return
	}

	li.wg.Wait()
}

func (e *elections[K, V]) releaseLeadership(ctx context.Context, key K) *leaderInfo[K, V] {
	liIntf, found := e.leadership.LoadAndDelete(key)
	if !found {
		logger.WarningCtx(ctx, "leadership.release.notleader", "we're not the leader")
		return nil
	}

	li := liIntf.(*leaderInfo[K, V])
	if _, err := e.storage.CompareAndDelete(key, li.val); err != nil {
		// notest
		logger.ErrorCtx(li.ctx, "leadership.release.error", "CompareAndDelete failed:", err)
	}

	li.cancel()

	logger.InfoCtx(li.ctx, "leadership.released", "")
	return li
}

// cleanup disallows new acquisitions, releases all ongoing leadership, and waits for them to finish.
func (e *elections[K, V]) cleanup() {
	e.isFinalized.Store(true)

	e.leadership.Range(func(key, liIntf any) bool {
		li := liIntf.(*leaderInfo[K, V])
		if _, err := e.storage.CompareAndDelete(key.(K), li.val); err != nil {
			// notest
			logger.ErrorCtx(li.ctx, "leadership.release.error", "CompareAndDelete failed:", err)
		}
		li.cancel()
		li.wg.Wait()
		e.leadership.Delete(key)
		return true
	})
}

func leadershipLogCtx[K any](key K) context.Context {
	return logger.WithContextAttrs(context.Background(), map[string]any{
		logger.LogAttr_VApp:      sys.VApp_SysVoedger,
		logger.LogAttr_Extension: "sys._Leadership",
		"key":                    key,
	})
}
