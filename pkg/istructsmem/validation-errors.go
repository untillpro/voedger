/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package istructsmem

import "fmt"

// validate error codes, see ValidateError.Code()
const (
	ECode_UnknownError = iota

	ECode_EmptyTypeName
	ECode_InvalidTypeName
	ECode_InvalidTypeKind

	ECode_EmptyData

	ECode_InvalidRecordID
	ECode_InvalidRefRecordID

	ECode_EEmptyCUDs

	ECode_InvalidChildName
	ECode_InvalidOccursMin
	ECode_InvalidOccursMax

	ECode_TooManyCreates
	ECode_TooManyUpdates
	ECode_TooManyChildren
)

type validateErrorType struct {
	error
	code int
}

func (e validateErrorType) Code() int {
	return e.code
}

func (e validateErrorType) Unwrap() error {
	return e.error
}

func validateError(code int, err error) ValidateError {
	e := validateErrorType{
		error: fmt.Errorf("%w; validate error code: %d", err, code),
		code:  code,
	}
	return e
}

func validateErrorf(code int, format string, a ...interface{}) ValidateError {
	return validateError(code, fmt.Errorf(format, a...))
}

const (
	// These errors are possible while checking type and content of the event arguments and CUDs
	errEventArgUseWrongType         = "%v argument uses wrong type «%v», expected «%v»: %w"
	errEventUnloggedArgUseWrongType = "%v unlogged argument uses wrong type «%v», expected «%v»: %w"
	errUnknownContainerName         = "%v child[%d] has unknown container name «%s»: %w"
	errWrongContainerType           = "%v child[%d] %v has wrong type name, expected «%v»: %w"
	errWrongParentID                = "%v child[%d] %v has wrong parent id «%d», expected «%d»: %w"
	errInvalidTypeKindInCUD         = "%v CUD.%s() [record ID «%d»] %v has invalid type kind: %w"
)
