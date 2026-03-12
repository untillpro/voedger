/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package actualizers

import (
	"context"
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
)

// Run is a function that runs actualizer for the specified projector.
type Run func(context.Context, appdef.AppQName, istructs.PartitionID, appdef.QName)

// PartitionActualizers manages actualizers deployment for the specified application partition.
type PartitionActualizers struct {
	app  appdef.AppQName
	part istructs.PartitionID
	rt   sync.Map // appdef.QName -> *runtime
	rtWG sync.WaitGroup
}

func New(app appdef.AppQName, part istructs.PartitionID) *PartitionActualizers {
	return &PartitionActualizers{
		app:  app,
		part: part,
		rt:   sync.Map{},
		rtWG: sync.WaitGroup{},
	}
}

// Deploys partition actualizers: stops actualizers for removed projectors and
// starts actualizers for new projectors using the specified run function.
func (pa *PartitionActualizers) Deploy(vvmCtx context.Context, appDef appdef.IAppDef, run Run) {
	pa.stopOlds(appDef)
	pa.startNews(vvmCtx, appDef, run)
}

// Returns all deployed actualizers
func (pa *PartitionActualizers) Enum() appdef.QNames {
	names := appdef.QNames{}
	for n := range pa.rt.Range {
		names.Add(n.(appdef.QName))
	}
	return names
}

// Wait waits for all actualizers to finish.
//
// The context should be stopped before calling this method. Here we just wait for actualizers to finish.
func (pa *PartitionActualizers) Wait() {
	pa.rtWG.Wait()
}

// async start actualizer
func (pa *PartitionActualizers) start(vvmCtx context.Context, name appdef.QName, run Run) {
	ctx, cancel := context.WithCancel(vvmCtx)
	rt := newRuntime(cancel)

	pa.rt.Store(name, rt)

	pa.rtWG.Add(1)

	go func() {
		defer func() {
			pa.rt.Delete(name)
			close(rt.done) // actualizer finished
			pa.rtWG.Done()
		}()

		run(ctx, pa.app, pa.part, name)
	}()
}

// async start new actualizers
func (pa *PartitionActualizers) startNews(vvmCtx context.Context, appDef appdef.IAppDef, run Run) {
	for prj := range appdef.Projectors(appDef.Types()) {
		if !prj.Sync() {
			name := prj.QName()
			if _, exists := pa.rt.Load(name); !exists {
				logCtx := logger.WithContextAttrs(vvmCtx, map[string]any{
					logger.LogAttr_VApp:      pa.app,
					logger.LogAttr_Extension: name,
				})
				pa.start(logCtx, name, run) // actualizer will be started in a separated goroutine there
			}
		}
	}
}

// async stop old actualizers
func (pa *PartitionActualizers) stopOlds(appDef appdef.IAppDef) {
	wg := sync.WaitGroup{}
	for name, rt := range pa.rt.Range {
		name := name.(appdef.QName)
		rt := rt.(*runtime)
		// TODO: compare if projector properties changed (events, sync/async, etc.)
		if appdef.Projector(appDef.Type, name) == nil {
			wg.Add(1)
			go func() {
				rt.cancel()
				<-rt.done
				wg.Done()
			}()
		}
	}
	// wrong to watch over vvmCtx. See https://github.com/voedger/voedger/issues/3971
	wg.Wait()
}

type runtime struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func newRuntime(cancel context.CancelFunc) *runtime {
	return &runtime{
		cancel: cancel,
		done:   make(chan struct{}),
	}
}
