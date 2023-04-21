/*
* Copyright (c) 2022-present unTill Pro, Ltd.
* @author Maxim Geraskin
 */

package iauthnz

import "github.com/voedger/voedger/pkg/schemas"

type OperationKindType byte

const (
	OperationKind_NULL OperationKindType = iota
	OperationKind_INSERT
	OperationKind_UPDATE
	OperationKind_SELECT
	OperationKind_EXECUTE
)

type AuthzRequest struct {
	OperationKind OperationKindType

	Resource schemas.QName

	Fields []string
}
