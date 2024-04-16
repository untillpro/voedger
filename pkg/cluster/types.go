/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package cluster

import coreutils "github.com/voedger/voedger/pkg/utils"

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
	NumParts       coreutils.NumAppPartitions
	EnginePoolSize [ProcessorKind_Count]int
}

func PoolSize(c, q, p int) [ProcessorKind_Count]int { return [ProcessorKind_Count]int{c, q, p} }
