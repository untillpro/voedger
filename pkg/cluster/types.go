/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package cluster

// ProcessorKind is a enumeration of processors.
type ProcessorKind uint8

//go:generate stringer -type=ProcessorKind

const (
	ProcessorKind_Command ProcessorKind = iota
	ProcessorKind_Query
	ProcessorKind_Projector

	ProcessorKind_Count
)

type AppDeploymentDescriptor struct {
	NumParts int

	EnginePoolSize [ProcessorKind_Count]int
}
