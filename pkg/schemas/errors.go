/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package schemas

import (
	"errors"
)

var ErrNameMissed = errors.New("name is missed")

var ErrInvalidName = errors.New("name not valid")

var ErrNameUniqueViolation = errors.New("duplicate name")

var ErrNameNotFound = errors.New("name not found")

var ErrInvalidQNameStringRepresentation = errors.New("invalid string representation of qualified name")

var ErrInvalidSchemaKind = errors.New("invalid schema kind")

var ErrWrongSchemaStruct = errors.New("wrong schema structure")

var ErrInvalidDataKind = errors.New("invalid data kind")

var ErrInvalidOccurs = errors.New("invalid occurs value")

var ErrFieldsMissed = errors.New("fields missed")
