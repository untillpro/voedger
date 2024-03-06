/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"errors"
	"fmt"
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/istructs"
)

type apps struct {
	structs    istructs.IAppStructsProvider
	actualizer SyncActualizerFactory
	apps       map[istructs.AppQName]*app
	mx         sync.RWMutex
}

func newAppPartitions(structs istructs.IAppStructsProvider, actualizer SyncActualizerFactory) (ap IAppPartitions, cleanup func(), err error) {
	a := &apps{
		structs:    structs,
		actualizer: actualizer,
		apps:       map[istructs.AppQName]*app{},
		mx:         sync.RWMutex{},
	}
	return a, func() {}, err
}

func (aps *apps) DeployApp(name istructs.AppQName, def appdef.IAppDef, partsCount int, engines [cluster.ProcessorKind_Count]int) {
	aps.mx.Lock()
	defer aps.mx.Unlock()

	if _, ok := aps.apps[name]; ok {
		panic(fmt.Errorf(errAppCannotToBeRedeployed, name, errors.ErrUnsupported))
	}

	a := newApplication(aps, name, partsCount)
	aps.apps[name] = a

	appStructs, err := aps.structs.AppStructsByDef(name, def)
	if err != nil {
		panic(err)
	}

	a.deploy(def, appStructs, engines)
}

func (aps *apps) DeployAppPartitions(appName istructs.AppQName, partIDs []istructs.PartitionID) {
	aps.mx.Lock()
	defer aps.mx.Unlock()

	a, ok := aps.apps[appName]
	if !ok {
		panic(fmt.Errorf(errAppNotFound, appName, ErrNotFound))
	}

	for _, id := range partIDs {
		p := newPartition(a, id)
		a.parts[id] = p
	}
}

func (aps *apps) AppDef(appName istructs.AppQName) (appdef.IAppDef, error) {
	aps.mx.Lock()
	defer aps.mx.Unlock()

	app, ok := aps.apps[appName]
	if !ok {
		return nil, fmt.Errorf(errAppNotFound, appName, ErrNotFound)
	}
	return app.def, nil
}

// Returns _total_ application partitions count.
//
// This is a configuration value for the application, independent of how many sections are currently deployed.
func (aps *apps) AppPartsCount(appName istructs.AppQName) (int, error) {
	aps.mx.Lock()
	defer aps.mx.Unlock()

	app, ok := aps.apps[appName]
	if !ok {
		return 0, fmt.Errorf(errAppNotFound, appName, ErrNotFound)
	}
	return app.partsCount, nil
}

func (aps *apps) Borrow(appName istructs.AppQName, partID istructs.PartitionID, proc cluster.ProcessorKind) (IAppPartition, error) {
	aps.mx.RLock()
	defer aps.mx.RUnlock()

	app, ok := aps.apps[appName]
	if !ok {
		return nil, fmt.Errorf(errAppNotFound, appName, ErrNotFound)
	}

	part, ok := app.parts[partID]
	if !ok {
		return nil, fmt.Errorf(errPartitionNotFound, appName, partID, ErrNotFound)
	}

	borrowed, err := part.borrow(proc)
	if err != nil {
		return nil, err
	}

	return borrowed, nil
}
