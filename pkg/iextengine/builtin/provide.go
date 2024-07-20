/*
  - Copyright (c) 2023-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/
package iextenginebuiltin

import "github.com/voedger/voedger/pkg/iextengine"

func ProvideExtensionEngineFactory(funcs iextengine.BuiltInExtFuncs, statelessFuncs iextengine.BuiltInAppExtFuncs) iextengine.IExtensionEngineFactory {
	return extensionEngineFactory{funcs, statelessFuncs}
}
