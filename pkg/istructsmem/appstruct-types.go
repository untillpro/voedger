/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package istructsmem

import (
	"errors"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/iterate"
	"github.com/voedger/voedger/pkg/irates"
	istorage "github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem/internal/containers"
	"github.com/voedger/voedger/pkg/istructsmem/internal/dynobuf"
	"github.com/voedger/voedger/pkg/istructsmem/internal/qnames"
	"github.com/voedger/voedger/pkg/istructsmem/internal/singletons"
	"github.com/voedger/voedger/pkg/istructsmem/internal/vers"
)

// AppConfigsType: map of applications configurators
type AppConfigsType map[appdef.AppQName]*AppConfigType

// AddAppConfig: adds new config for specified application or replaces if exists
func (cfgs *AppConfigsType) AddAppConfig(name appdef.AppQName, id istructs.ClusterAppID, def appdef.IAppDef, wsCount istructs.NumAppWorkspaces) *AppConfigType {
	c := newAppConfig(name, id, def, wsCount)

	(*cfgs)[name] = c
	return c
}

// AddBuiltInAppConfig: adds new config for specified builtin application or replaces if exists
func (cfgs *AppConfigsType) AddBuiltInAppConfig(appName appdef.AppQName, appDef appdef.IAppDefBuilder) *AppConfigType {
	c := newBuiltInAppConfig(appName, appDef)

	(*cfgs)[appName] = c
	return c
}

// GetConfig: gets config for specified application
func (cfgs *AppConfigsType) GetConfig(appName appdef.AppQName) *AppConfigType {
	c, ok := (*cfgs)[appName]
	if !ok {
		panic(fmt.Errorf("unable return configuration for unknown application «%v»: %w", appName, istructs.ErrAppNotFound))
	}
	return c
}

// AppConfigType: configuration for application workflow
type AppConfigType struct {
	Name         appdef.AppQName
	ClusterAppID istructs.ClusterAppID

	appDefBuilder appdef.IAppDefBuilder
	AppDef        appdef.IAppDef
	Resources     Resources

	// Application configuration parameters
	Params AppConfigParams

	dynoSchemes *dynobuf.DynoBufSchemes

	storage            istorage.IAppStorage // will be initialized on prepare()
	versions           *vers.Versions
	qNames             *qnames.QNames
	cNames             *containers.Containers
	singletons         *singletons.Singletons
	prepared           bool
	app                *appStructsType
	FunctionRateLimits functionRateLimits
	syncProjectors     istructs.Projectors
	asyncProjectors    istructs.Projectors
	cudValidators      []istructs.CUDValidator
	eventValidators    []istructs.EventValidator
	numAppWorkspaces   istructs.NumAppWorkspaces
}

func newAppConfig(name appdef.AppQName, id istructs.ClusterAppID, def appdef.IAppDef, wsCount istructs.NumAppWorkspaces) *AppConfigType {
	cfg := AppConfigType{
		Name:             name,
		ClusterAppID:     id,
		Params:           makeAppConfigParams(),
		syncProjectors:   make(istructs.Projectors),
		asyncProjectors:  make(istructs.Projectors),
		numAppWorkspaces: wsCount,
	}

	cfg.AppDef = def
	cfg.Resources = makeResources()

	cfg.dynoSchemes = dynobuf.New()

	cfg.versions = vers.New()
	cfg.qNames = qnames.New()
	cfg.cNames = containers.New()
	cfg.singletons = singletons.New()

	cfg.FunctionRateLimits = functionRateLimits{
		limits: map[appdef.QName]map[istructs.RateLimitKind]istructs.RateLimit{},
	}
	return &cfg
}

func newBuiltInAppConfig(appName appdef.AppQName, appDef appdef.IAppDefBuilder) *AppConfigType {
	id, ok := istructs.ClusterApps[appName]
	if !ok {
		panic(fmt.Errorf("unable construct configuration for unknown application «%v»: %w", appName, istructs.ErrAppNotFound))
	}

	def, err := appDef.Build()
	if err != nil {
		panic(fmt.Errorf("%v: unable build application: %w", appName, err))
	}

	cfg := newAppConfig(appName, id, def, 0)
	cfg.appDefBuilder = appDef

	return cfg
}

// prepare: prepares application configuration to use. It creates config globals and must be called from thread-safe code
func (cfg *AppConfigType) prepare(buckets irates.IBuckets, appStorage istorage.IAppStorage) error {
	// if cfg.QNameID == istructs.NullClusterAppID {…} — unnecessary check. QNameIDmust be checked before prepare()

	if cfg.prepared {
		return nil
	}

	if cfg.appDefBuilder != nil {
		// BuiltIn application, appDefBuilder can be changed after add config
		app, err := cfg.appDefBuilder.Build()
		if err != nil {
			return fmt.Errorf("%v: unable rebuild changed application: %w", cfg.Name, err)
		}
		cfg.AppDef = app
	}

	cfg.dynoSchemes.Prepare(cfg.AppDef)

	// prepare IAppStorage
	cfg.storage = appStorage

	// prepare system views versions
	if err := cfg.versions.Prepare(cfg.storage); err != nil {
		return err
	}

	// prepare QNames
	if err := cfg.qNames.Prepare(cfg.storage, cfg.versions, cfg.AppDef); err != nil {
		return err
	}

	// prepare container names
	if err := cfg.cNames.Prepare(cfg.storage, cfg.versions, cfg.AppDef); err != nil {
		return err
	}

	// prepare singleton CDocs
	if err := cfg.singletons.Prepare(cfg.storage, cfg.versions, cfg.AppDef); err != nil {
		return err
	}

	// prepare functions rate limiter
	cfg.FunctionRateLimits.prepare(buckets)

	if err := cfg.validateResources(); err != nil {
		return err
	}

	if cfg.numAppWorkspaces <= 0 {
		return fmt.Errorf("%s: %w", cfg.Name, ErrNumAppWorkspacesNotSet)
	}

	cfg.prepared = true
	return nil
}

func (cfg *AppConfigType) validateResources() (err error) {

	cfg.AppDef.Extensions(func(ext appdef.IExtension) {
		if ext.Engine() == appdef.ExtensionEngineKind_BuiltIn {
			// Only builtin extensions should be validated by cfg.Resources
			name := ext.QName()
			switch ext.Kind() {
			case appdef.TypeKind_Query, appdef.TypeKind_Command:
				if cfg.Resources.QueryResource(name).QName() == appdef.NullQName {
					err = errors.Join(err,
						fmt.Errorf("%v: exec is not defined: %w", ext, ErrNameNotFound))
				}
			case appdef.TypeKind_Projector:
				prj := ext.(appdef.IProjector)
				_, syncFound := cfg.syncProjectors[name]
				_, asyncFound := cfg.asyncProjectors[name]
				if !syncFound && !asyncFound {
					err = errors.Join(err,
						fmt.Errorf("%v: exec is not defined in Resources", prj))
				} else if syncFound && asyncFound {
					err = errors.Join(err,
						fmt.Errorf("%v: exec is defined twice in Resources (both sync & async)", prj))
				} else if prj.Sync() && asyncFound {
					err = errors.Join(err,
						fmt.Errorf("%v: exec is defined in Resources as async, but sync expected", prj))
				} else if !prj.Sync() && syncFound {
					err = errors.Join(err,
						fmt.Errorf("%v: exec is defined in Resources as sync, but async expected", prj))
				}
			}
		}
	})

	if err != nil {
		return err
	}
	err = iterate.ForEachError(cfg.Resources.Resources, func(qName appdef.QName) error {
		if cfg.AppDef.Type(qName).Kind() == appdef.TypeKind_null {
			return fmt.Errorf("exec of func %s is defined but the func is not defined in SQL", qName)
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, prj := range cfg.syncProjectors {
		if cfg.AppDef.TypeByName(prj.Name) == nil {
			return fmt.Errorf("exec of sync projector %s is defined but the projector is not defined in SQL", prj.Name)
		}
	}
	for _, prj := range cfg.asyncProjectors {
		if cfg.AppDef.TypeByName(prj.Name) == nil {
			return fmt.Errorf("exec of async projector %s is defined but the projector is not defined in SQL", prj.Name)
		}
	}
	return nil
}

func (cfg *AppConfigType) AddSyncProjectors(pp ...istructs.Projector) {
	for _, p := range pp {
		cfg.syncProjectors[p.Name] = p
	}
}

func (cfg *AppConfigType) AddAsyncProjectors(pp ...istructs.Projector) {
	for _, p := range pp {
		cfg.asyncProjectors[p.Name] = p
	}
}

func (cfg *AppConfigType) AddCUDValidators(cudValidators ...istructs.CUDValidator) {
	cfg.cudValidators = append(cfg.cudValidators, cudValidators...)
}

func (cfg *AppConfigType) AddEventValidators(eventValidators ...istructs.EventValidator) {
	cfg.eventValidators = append(cfg.eventValidators, eventValidators...)
}

func (cfg *AppConfigType) AsyncProjectors() istructs.Projectors {
	return cfg.asyncProjectors
}

// Returns is application configuration prepared
func (cfg *AppConfigType) Prepared() bool {
	return cfg.prepared
}

func (cfg *AppConfigType) SyncProjectors() istructs.Projectors {
	return cfg.syncProjectors
}

// need to build view.sys.NextBaseWSID and view.sys.projectionOffsets
// could be called on application build stage only
//
// Should be used for built-in applications only.
func (cfg *AppConfigType) AppDefBuilder() appdef.IAppDefBuilder {
	if cfg.prepared {
		panic("IAppStructsProvider.AppStructs() is called already for the app -> IAppDef is built already -> wrong to work with IAppDefBuilder")
	}
	return cfg.appDefBuilder
}

func (cfg *AppConfigType) NumAppWorkspaces() istructs.NumAppWorkspaces {
	return cfg.numAppWorkspaces
}

// must be called after creating the AppConfigType because app will provide the deployment descriptor with the actual NumAppWorkspaces after willing the AppConfigType
// so fisrt create AppConfigType, use it on app provide, then set the actual NumAppWorkspaces
func (cfg *AppConfigType) SetNumAppWorkspaces(naw istructs.NumAppWorkspaces) {
	if cfg.prepared {
		panic("must not set NumAppWorkspaces after first IAppStructsProvider.AppStructs() call because the app is considered working")
	}
	cfg.numAppWorkspaces = naw
}

// Application configuration parameters
type AppConfigParams struct {
	// PLog events cache size.
	//
	// Default value is DefaultPLogEventCacheSize (10’000 events).
	// Zero (0) means that cache will not be used
	PLogEventCacheSize int
}

func makeAppConfigParams() AppConfigParams {
	return AppConfigParams{
		PLogEventCacheSize: DefaultPLogEventCacheSize, // 10’000
	}
}
