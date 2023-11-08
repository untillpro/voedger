/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts/internal/pool"
	"github.com/voedger/voedger/pkg/istructs"
)

type app struct {
	name istructs.AppQName
	// no locks need. Owned appPartitions structure will locks access to this structure
	parts map[istructs.PartitionID]*partition
}

func newApplication(name istructs.AppQName) *app {
	return &app{
		name:  name,
		parts: map[istructs.PartitionID]*partition{},
	}
}

type partition struct {
	app        *app
	appDef     appdef.IAppDef
	appStructs istructs.IAppStructs
	id         istructs.PartitionID
	pool       [ProcKind_Count]*pool.Pool[IProc]
}

func newPartition(app *app, appDef appdef.IAppDef, appStructs istructs.IAppStructs, id istructs.PartitionID, processors [ProcKind_Count][]IProc) *partition {
	part := &partition{
		app:        app,
		appDef:     appDef,
		appStructs: appStructs,
		id:         id,
	}
	for k, p := range processors {
		part.pool[k] = pool.New[IProc](p)
	}
	return part
}

func (p *partition) borrow(proc ProcKind) (*partitionRT, IProc, error) {
	b := newPartitionRT(p, proc)

	if err := b.init(); err != nil {
		return nil, nil, err
	}

	return b, b.borrowed, nil
}

type partitionRT struct {
	part       *partition
	appDef     appdef.IAppDef
	appStructs istructs.IAppStructs
	proc       ProcKind
	borrowed   IProc
}

func newPartitionRT(part *partition, proc ProcKind) *partitionRT {
	return &partitionRT{
		part:       part,
		appDef:     part.appDef,
		appStructs: part.appStructs,
		proc:       proc}
}

func (rt *partitionRT) App() istructs.AppQName           { return rt.part.app.name }
func (rt *partitionRT) AppStructs() istructs.IAppStructs { return rt.appStructs }
func (rt *partitionRT) ID() istructs.PartitionID         { return rt.part.id }

func (rt *partitionRT) Release() {
	if b := rt.borrowed; b != nil {
		rt.borrowed = nil
		rt.part.pool[rt.proc].Release(b)
	}
}

// Initialize partition RT structures for use
func (rt *partitionRT) init() error {
	p, err := rt.part.pool[rt.proc].Borrow()
	if err != nil {
		return fmt.Errorf(errNotEnoughProcessor, rt.proc.TrimString(), err)
	}
	rt.borrowed = p
	return nil
}
