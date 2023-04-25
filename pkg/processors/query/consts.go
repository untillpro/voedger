/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package queryprocessor

import "github.com/voedger/voedger/pkg/schemas"

const (
	filterKind_Eq    = "eq"
	filterKind_NotEq = "notEq"
	filterKind_Gt    = "gt"
	filterKind_Lt    = "lt"
	filterKind_And   = "and"
	filterKind_Or    = "or"
)

const (
	minNormalFloat64     = 0x1.0p-1022
	rootDocument         = ""
	Field_JSONSchemaBody = "Body"
)

var (
	qNamePosDepartment = schemas.NewQName("pos", "Department")
	qNameXLowerCase    = schemas.NewQName("x", "lower-case")
)
