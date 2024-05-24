/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package engines

import (
	"context"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/state"
)

// provides all built-in extension functions for specified application config
//
// # Panics:
//   - if any extension implementation not found
//   - if any extension package full path is unknown
func provideAppsBuiltInExtFuncs(cfgs istructsmem.AppConfigsType) iextengine.BuiltInExtFuncs {
	funcs := make(iextengine.BuiltInExtFuncs)

	for app, cfg := range cfgs {
		appFuncs := make(iextengine.BuiltInAppExtFuncs)
		cfg.AppDef.Extensions(
			func(ext appdef.IExtension) {
				if ext.Engine() != appdef.ExtensionEngineKind_BuiltIn {
					return
				}
				name := ext.QName()

				var fn iextengine.BuiltInExtFunc

				switch ext.Kind() {
				case appdef.TypeKind_Command:
					if cmd, ok := cfg.Resources.QueryResource(name).(istructs.ICommandFunction); ok {
						fn = func(_ context.Context, io iextengine.IExtensionIO) error {
							kb, err := io.KeyBuilder(state.CommandContext, appdef.NullQName)
							if err != nil {
								return err
							}
							cmdCtx, err := io.MustExist(kb)
							if err != nil {
								return err
							}

							if cmdCtx == nil {
								return fmt.Errorf("command context not found")
							}

							execArgs := istructs.ExecCommandArgs{
								CommandPrepareArgs: io.CommandPrepareArgs(),
								State:              io,
								Intents:            io,
							}
							return cmd.Exec(execArgs)
						}
					}
				case appdef.TypeKind_Query:
					if query, ok := cfg.Resources.QueryResource(name).(istructs.IQueryFunction); ok {
						fn = func(ctx context.Context, io iextengine.IExtensionIO) error {
							return query.Exec(
								ctx,
								istructs.ExecQueryArgs{State: io},
								// TODO: add query result handler
								func(istructs.IObject) error { return nil },
							)
						}
					}
				case appdef.TypeKind_Projector:
					var prj istructs.Projector
					if ext.(appdef.IProjector).Sync() {
						prj = cfg.SyncProjectors()[name]
					} else {
						prj = cfg.AsyncProjectors()[name]
					}
					if prj.Name != appdef.NullQName {
						fn = func(_ context.Context, io iextengine.IExtensionIO) error {
							return prj.Func(io.PLogEvent(), io, io)
						}
					}
				}

				if fn == nil {
					panic(fmt.Errorf("application «%v»: %v implementation not found", app, ext))
				}

				extName := cfg.AppDef.FullQName(name)
				if extName == appdef.NullFullQName {
					panic(fmt.Errorf("application «%v»: package «%s» full path is unknown", app, name.Pkg()))
				}

				appFuncs[extName] = fn
			})

		funcs[app] = appFuncs
	}
	return funcs
}
