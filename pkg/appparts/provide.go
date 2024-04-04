/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
)

type SyncActualizerFactory = func(istructs.IAppStructs, istructs.PartitionID) pipeline.ISyncOperator

// New only for tests where sync actualizer is not used
func New(structs istructs.IAppStructsProvider) (ap IAppPartitions, cleanup func(), err error) {
	return NewWithActualizerWithExtEnginesFactories(
		structs,
		func(istructs.IAppStructs, istructs.PartitionID) pipeline.ISyncOperator {
			return &pipeline.NOOP{}
		},
		iextengine.ExtensionEngineFactories{
			appdef.ExtensionEngineKind_BuiltIn: iextengine.NullExtensionEngineFactory,
			appdef.ExtensionEngineKind_WASM:    iextengine.NullExtensionEngineFactory,
		},
	)
}

func NewWithActualizerWithExtEnginesFactories(structs istructs.IAppStructsProvider, actualizer SyncActualizerFactory,
	eef iextengine.ExtensionEngineFactories) (ap IAppPartitions, cleanup func(), err error) {
	return newAppPartitions(structs, actualizer, eef)
}
