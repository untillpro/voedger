/*
  - Copyright (c) 2023-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/
package iextenginebuiltin

import (
	"context"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iextengine"
)

type extensionEngineFactory struct {
	funcs          iextengine.BuiltInExtFuncs
	statelessFuncs iextengine.BuiltInAppExtFuncs
}

type extensionEngine struct {
	app            appdef.AppQName
	funcs          iextengine.BuiltInExtFuncs
	statelessFuncs iextengine.BuiltInAppExtFuncs
}

func (e extensionEngine) SetLimits(limits iextengine.ExtensionLimits) {}

func (e extensionEngine) Invoke(ctx context.Context, extName appdef.FullQName, io iextengine.IExtensionIO) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("extension panic: %v", r)
		}
	}()
	if f, ok := e.statelessFuncs[extName]; ok {
		return f(ctx, io)
	}
	if appFuncs, ok := e.funcs[e.app]; ok {
		if f, ok := appFuncs[extName]; ok {
			return f(ctx, io)
		}
	}
	return undefinedExtension(e.app, extName.String())
}

func (e extensionEngine) Close(ctx context.Context) {}

func (f extensionEngineFactory) New(_ context.Context, app appdef.AppQName, _ []iextengine.ExtensionPackage, _ *iextengine.ExtEngineConfig, numEngines int) (result []iextengine.IExtensionEngine, err error) {
	result = make([]iextengine.IExtensionEngine, numEngines)
	for i := 0; i < numEngines; i++ {
		result[i] = &extensionEngine{app, f.funcs, f.statelessFuncs}
	}
	return
}
