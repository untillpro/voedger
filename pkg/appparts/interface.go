/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appparts

import (
	"context"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/istructs"
)

// Application partitions manager.
//
// @ConcurrentAccess
type IAppPartitions interface {
	// Adds new application or update existing.
	//
	// partsCount - total partitions count for the application.
	//
	// If application with the same name exists, then its definition will be updated.
	DeployApp(name istructs.AppQName, def appdef.IAppDef, partsCount istructs.NumAppPartitions, numEngines [cluster.ProcessorKind_Count]int)

	// Deploys new partitions for specified application or update existing.
	//
	// If partition with the same app and id already exists, it will be updated.
	//
	// # Panics:
	// 	- if application not exists
	DeployAppPartitions(appName istructs.AppQName, partIDs []istructs.PartitionID)

	// Returns application definition.
	//
	// Returns nil and error if app not exists.
	AppDef(istructs.AppQName) (appdef.IAppDef, error)

	// Returns _total_ application partitions count.
	//
	// This is a configuration value for the application, independent of how many sections are currently deployed.
	//
	// Returns 0 and error if app not exists.
	AppPartsCount(istructs.AppQName) (istructs.NumAppPartitions, error)

	// Returns partition ID for specified workspace
	//
	// Returns error if app not exists.
	AppWorkspacePartitionID(istructs.AppQName, istructs.WSID) (istructs.PartitionID, error)

	// Borrows and returns a partition.
	//
	// If partition not exist, returns error.
	Borrow(istructs.AppQName, istructs.PartitionID, cluster.ProcessorKind) (IAppPartition, error)

	// Waits for partition to be available and borrows it.
	//
	// If partition not exist, returns error.
	WaitForBorrow(context.Context, istructs.AppQName, istructs.PartitionID, cluster.ProcessorKind) (IAppPartition, error)
}

// Application partition.
type IAppPartition interface {
	App() istructs.AppQName
	ID() istructs.PartitionID

	AppStructs() istructs.IAppStructs

	// Releases borrowed partition
	Release()

	DoSyncActualizer(ctx context.Context, work interface{}) (err error)

	// Invoke extension engine.
	Invoke(ctx context.Context, name appdef.QName, state istructs.IState, intents istructs.IIntents) error
}
