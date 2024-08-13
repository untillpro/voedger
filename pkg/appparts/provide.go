/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"context"

	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
)

type SyncActualizerFactory = func(istructs.IAppStructs, istructs.PartitionID) pipeline.ISyncOperator

// New only for tests where sync actualizer is not used
func New(structs istructs.IAppStructsProvider) (ap IAppPartitions, cleanup func(), err error) {
	return New2(
		context.Background(),
		structs,
		NullSyncActualizerFactory,
		NullProcessorRunner,
		NullProcessorRunner,
		NullExtensionEngineFactories,
	)
}

// New2 creates new app partitions.
//
// # Parameters:
//
//	vvmCtx - VVM context. Used to run async actualizers
//	structs - application structures provider
//	syncAct - sync actualizer factory, old actualizers style, should be used with builtin applications only
//	asyncActualizersRunner - async actualizers runner
//	jobSchedulerRunner - job scheduler runner
//	eef - extension engine factories
func New2(
	vvmCtx context.Context,
	structs istructs.IAppStructsProvider,
	syncAct SyncActualizerFactory,
	asyncActualizersRunner IProcessorRunner,
	jobSchedulerRunner IProcessorRunner,
	eef iextengine.ExtensionEngineFactories,
) (ap IAppPartitions, cleanup func(), err error) {
	return newAppPartitions(vvmCtx, structs, syncAct, asyncActualizersRunner, jobSchedulerRunner, eef)
}
