/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package cluster

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

// ProcessorKind is a enumeration of processors.
type ProcessorKind uint8

//go:generate stringer -type=ProcessorKind

const (
	ProcessorKind_Command ProcessorKind = iota
	ProcessorKind_Query
	ProcessorKind_Actualizer

	ProcessorKind_Count
)

type AppDeploymentDescriptor struct {
	// Number of partitions. Partitions IDs will be generated from 0 to NumParts-1
	//
	// NumParts should contain _total_ number of partitions, not only to deploy.
	NumParts istructs.NumAppPartitions

	// EnginePoolSize pools size for each processor kind
	EnginePoolSize [ProcessorKind_Count]int

	// total numer of AppWorkspaces
	NumAppWorkspaces istructs.NumAppWorkspaces
}

func PoolSize(c, q, p int) [ProcessorKind_Count]int { return [ProcessorKind_Count]int{c, q, p} }

// Describes built-in application.
type BuiltInApp struct {
	AppDeploymentDescriptor

	Name istructs.AppQName

	// Application definition will use to generate AppStructs
	Def appdef.IAppDef
}
