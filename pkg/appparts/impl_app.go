/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts/internal/actualizer"
	"github.com/voedger/voedger/pkg/appparts/internal/pool"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
)

type engines map[appdef.ExtensionEngineKind]iextengine.IExtensionEngine

type appVersion struct {
	mx      sync.RWMutex
	def     appdef.IAppDef
	structs istructs.IAppStructs
	pools   [ProcessorKind_Count]*pool.Pool[engines]
}

func (av *appVersion) appDef() appdef.IAppDef {
	av.mx.RLock()
	defer av.mx.RUnlock()
	return av.def
}

func (av *appVersion) appStructs() istructs.IAppStructs {
	av.mx.RLock()
	defer av.mx.RUnlock()
	return av.structs
}

// returns AppDef, AppStructs and engines pool for the specified processor kind
func (av *appVersion) snapshot(proc ProcessorKind) (appdef.IAppDef, istructs.IAppStructs, *pool.Pool[engines]) {
	av.mx.RLock()
	defer av.mx.RUnlock()
	return av.def, av.structs, av.pools[proc]
}

func (av *appVersion) upgrade(
	def appdef.IAppDef,
	structs istructs.IAppStructs,
	pools [ProcessorKind_Count]*pool.Pool[engines],
) {
	av.mx.Lock()
	defer av.mx.Unlock()

	av.def = def
	av.structs = structs
	av.pools = pools
}

type appRT struct {
	mx             sync.RWMutex
	apps           *apps
	name           appdef.AppQName
	partsCount     istructs.NumAppPartitions
	lastestVersion appVersion
	parts          map[istructs.PartitionID]*appPartitionRT
}

func newApplication(apps *apps, name appdef.AppQName, partsCount istructs.NumAppPartitions) *appRT {
	return &appRT{
		mx:             sync.RWMutex{},
		apps:           apps,
		name:           name,
		partsCount:     partsCount,
		lastestVersion: appVersion{},
		parts:          map[istructs.PartitionID]*appPartitionRT{},
	}
}

// extModuleURLs is important for non-builtin (non-native) apps
// extModuleURLs: packagePath->packageURL
func (a *appRT) deploy(def appdef.IAppDef, extModuleURLs map[string]*url.URL, structs istructs.IAppStructs, numEnginesPerEngineKind [ProcessorKind_Count]int) {
	eef := a.apps.extEngineFactories

	enginesPathsModules := map[appdef.ExtensionEngineKind]map[string]*iextengine.ExtensionModule{}
	def.Extensions(func(ext appdef.IExtension) {
		extEngineKind := ext.Engine()
		path := ext.App().PackageFullPath(ext.QName().Pkg())
		pathsModules, ok := enginesPathsModules[extEngineKind]
		if !ok {
			// initialize any engine mentioned in the schema
			pathsModules = map[string]*iextengine.ExtensionModule{}
			enginesPathsModules[extEngineKind] = pathsModules
		}
		if extEngineKind != appdef.ExtensionEngineKind_WASM {
			return
		}
		extModule, ok := pathsModules[path]
		if !ok {
			moduleURL, ok := extModuleURLs[path]
			if !ok {
				panic(fmt.Sprintf("module path %s is missing among extension modules URLs", path))
			}
			extModule = &iextengine.ExtensionModule{
				Path:      path,
				ModuleUrl: moduleURL,
			}
			pathsModules[path] = extModule
		}
		extModule.ExtensionNames = append(extModule.ExtensionNames, ext.QName().Entity())
	})
	extModules := map[appdef.ExtensionEngineKind][]iextengine.ExtensionModule{}
	for extEngineKind, pathsModules := range enginesPathsModules {
		extModules[extEngineKind] = nil // initialize any engine mentioned in the schema
		for _, extModule := range pathsModules {
			extModules[extEngineKind] = append(extModules[extEngineKind], *extModule)
		}
	}

	pools := [ProcessorKind_Count]*pool.Pool[engines]{}
	// processorKind here is one of ProcessorKind_Command, ProcessorKind_Query, ProcessorKind_Actualizer
	for processorKind, processorsCountPerKind := range numEnginesPerEngineKind {
		ee := make([]engines, processorsCountPerKind)
		for extEngineKind, extensionModules := range extModules {
			extensionEngineFactory, ok := eef[extEngineKind]
			if !ok {
				panic(fmt.Errorf("no extension engine factory for engine %s met among def of %s", extEngineKind.String(), a.name))
			}
			extEngines, err := extensionEngineFactory.New(a.apps.vvmCtx, a.name, extensionModules, &iextengine.DefaultExtEngineConfig, processorsCountPerKind)
			if err != nil {
				panic(err)
			}
			for i := 0; i < processorsCountPerKind; i++ {
				if ee[i] == nil {
					ee[i] = map[appdef.ExtensionEngineKind]iextengine.IExtensionEngine{}
				}
				ee[i][extEngineKind] = extEngines[i]
			}
		}
		pools[processorKind] = pool.New(ee)
	}

	a.lastestVersion.upgrade(def, structs, pools)
}

type appPartitionRT struct {
	app            *appRT
	id             istructs.PartitionID
	syncActualizer pipeline.ISyncOperator
	actualizers    *actualizer.Actualizers
}

func newAppPartitionRT(app *appRT, id istructs.PartitionID) *appPartitionRT {
	part := &appPartitionRT{
		app:            app,
		id:             id,
		syncActualizer: app.apps.syncActualizerFactory(app.lastestVersion.appStructs(), id),
		actualizers:    actualizer.New(app.name, id),
	}
	return part
}

func (p *appPartitionRT) borrow(proc ProcessorKind) (*borrowedPartition, error) {
	b := newBorrowedPartition(p)

	if err := b.borrow(proc); err != nil {
		return nil, err
	}

	return b, nil
}

// # Implements IAppPartition
type borrowedPartition struct {
	part       *appPartitionRT
	appDef     appdef.IAppDef
	appStructs istructs.IAppStructs
	kind       ProcessorKind
	pool       *pool.Pool[engines] // pool of borrowed engines
	engines    engines             // borrowed engines
}

var borrowedPartitionsPool = sync.Pool{
	New: func() interface{} {
		return &borrowedPartition{}
	},
}

func newBorrowedPartition(part *appPartitionRT) *borrowedPartition {
	bp := borrowedPartitionsPool.Get().(*borrowedPartition)
	bp.part = part
	return bp
}

// # IAppPartition.App
func (bp *borrowedPartition) App() appdef.AppQName { return bp.part.app.name }

// # IAppPartition.AppStructs
func (bp *borrowedPartition) AppStructs() istructs.IAppStructs { return bp.appStructs }

// # IAppPartition.DoSyncActualizer
func (bp *borrowedPartition) DoSyncActualizer(ctx context.Context, work pipeline.IWorkpiece) error {
	return bp.part.syncActualizer.DoSync(ctx, work)
}

// # IAppPartition.ID
func (bp *borrowedPartition) ID() istructs.PartitionID { return bp.part.id }

// # IAppPartition.Invoke
func (bp *borrowedPartition) Invoke(ctx context.Context, name appdef.QName, state istructs.IState, intents istructs.IIntents) error {
	e := bp.appDef.Extension(name)
	if e == nil {
		return errUndefinedExtension(name)
	}

	if compat, err := bp.kind.compatibleWithExtension(e); !compat {
		return fmt.Errorf("%s: %w", bp, err)
	}

	extName := bp.appDef.FullQName(name)
	if extName == appdef.NullFullQName {
		return errCantObtainFullQName(name)
	}
	io := iextengine.NewExtensionIO(bp.appDef, state, intents)

	extEngine, ok := bp.engines[e.Engine()]
	if !ok {
		return fmt.Errorf("no extension engine for extension kind %s", e.Engine().String())
	}

	return extEngine.Invoke(ctx, extName, io)
}

func (bp *borrowedPartition) String() string {
	return fmt.Sprintf("borrowedPartition{app=%s, part=%d, kind=%s}", bp.part.app.name, bp.part.id, bp.kind)
}

// # IAppPartition.Release
func (bp *borrowedPartition) Release() {
	bp.part = nil
	bp.appDef = nil
	bp.appStructs = nil
	if pool := bp.pool; pool != nil {
		bp.pool = nil
		if engine := bp.engines; engine != nil {
			bp.engines = nil
			pool.Release(engine)
		}
	}
	borrowedPartitionsPool.Put(bp)
}

func (bp *borrowedPartition) borrow(proc ProcessorKind) (err error) {
	bp.kind = proc
	bp.appDef, bp.appStructs, bp.pool = bp.part.app.lastestVersion.snapshot(proc)
	bp.engines, err = bp.pool.Borrow()
	if err != nil {
		return errNotAvailableEngines[proc]
	}
	return nil
}
