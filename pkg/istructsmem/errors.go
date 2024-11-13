/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package istructsmem

import (
	"errors"
	"fmt"
	"strings"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

// Enriches error with additional information
//
// arg is any value to be added to the error message, and args are additional values to be added to the error message.
//
// If arg is a string contains `%` and args is not empty, then arg is treated as a format string
func enrichError(err error, arg any, args ...any) error {
	var enrich string
	if msg, ok := arg.(string); ok && len(args) > 0 && strings.Contains(msg, "%") {
		enrich = fmt.Sprintf(msg, args...)
	} else {
		enrich = fmt.Sprint(arg)
		for i := range args {
			enrich += " " + fmt.Sprint(args[i])
		}
	}
	return fmt.Errorf("%w: %s", err, enrich)
}

// TODO: use enrichError() for all errors
// eliminate all calls fmt.Errorf("… %w …", …) with err×××Wrap constants

var ErrorEventNotValidError = errors.New("event is not valid")

func ErrorEventNotValid(arg any, args ...any) error {
	return enrichError(ErrorEventNotValidError, arg, args...)
}

var ErrNameMissedError = errors.New("name is empty")

func ErrNameMissed(arg any, args ...any) error {
	return enrichError(ErrNameMissedError, arg, args...)
}

var ErrOutOfBoundsError = errors.New("out of bounds")

func ErrOutOfBounds(arg any, args ...any) error {
	return enrichError(ErrOutOfBoundsError, arg, args...)
}

var ErrWrongTypeError = errors.New("wrong type")

func ErrWrongType(arg any, args ...any) error {
	return enrichError(ErrWrongTypeError, arg, args...)
}

var ErrNameNotFoundError = errors.New("name not found")

func ErrNameNotFound(arg any, args ...any) error {
	return enrichError(ErrNameNotFoundError, arg, args...)
}

func ErrFieldNotFound(name string, typ interface{}) error {
	return enrichError(ErrNameNotFoundError, "field «%s» in %v", name, typ)
}

func ErrTypedFieldNotFound(t, f string, typ interface{}) error {
	return enrichError(ErrNameNotFoundError, "%s-field «%s» in %v", t, f, typ)
}

func ErrContainerNotFound(name string, typ interface{}) error {
	return enrichError(ErrNameNotFoundError, "container «%s» in %v", name, typ)
}

// name should  be string or any Stringer interface (e.g. QName)
func ErrTypeNotFound(name interface{}) error {
	return enrichError(ErrNameNotFoundError, "type «%v»", name)
}

// name should  be string or any Stringer interface (e.g. QName)
func ErrViewNotFound(name interface{}) error {
	return enrichError(ErrNameNotFoundError, "view «%v»", name)
}

var ErrInvalidNameError = errors.New("name not valid")

func ErrInvalidName(arg any, args ...any) error {
	return enrichError(ErrInvalidNameError, arg, args...)
}

var ErrIDNotFoundError = errors.New("ID not found")

func ErrIDNotFound(arg any, args ...any) error {
	return enrichError(ErrIDNotFoundError, arg, args...)
}

func ErrRefIDNotFound(t interface{}, f string, id istructs.RecordID) error {
	return ErrIDNotFound("%v field «%s» refers to unknown ID «%d»", t, f, id)
}

var ErrRecordNotFound = errors.New("record cannot be found")

var ErrMinOccursViolationError = errors.New("minimum occurs violated")

func ErrMinOccursViolated(t interface{}, n string, o, minO appdef.Occurs) error {
	return enrichError(ErrMinOccursViolationError, "%v container «%s» has not enough occurrences (%d, minimum %d)", t, n, o, minO)
}

var ErrMaxOccursViolationError = errors.New("maximum occurs violated")

func ErrMaxOccursViolated(t interface{}, n string, o, maxO appdef.Occurs) error {
	return enrichError(ErrMaxOccursViolationError, "%v container «%s» has too many occurrences (%d, maximum %d)", t, n, o, maxO)
}

var ErrFieldIsEmptyError = errors.New("field is empty")

// name should  be string or any Stringer interface (e.g. IField)
func ErrFieldIsEmpty(name interface{}) error {
	return enrichError(ErrFieldIsEmptyError, "%v", name)
}

func ErrFieldMissed(t, name interface{}) error {
	return enrichError(ErrFieldIsEmptyError, "%v %v", t, name)
}

var ErrInvalidVerificationKindError = errors.New("invalid verification kind")

func ErrInvalidVerificationKind(t, f interface{}, k appdef.VerificationKind) error {
	return enrichError(ErrInvalidVerificationKindError, "%s for %v «%v»", k.TrimString(), t, f)
}

var ErrCUDsMissedError = errors.New("CUDs are missed")

// event should be string or any Stringer interface (e.g. IEvent)
func ErrCUDsMissed(event interface{}) error {
	return enrichError(ErrCUDsMissedError, "%v", event)
}

var ErrRawRecordIDRequiredError = errors.New("raw record ID required")

func ErrRawRecordIDRequired(row, fld interface{}, id istructs.RecordID) error {
	return enrichError(ErrRawRecordIDRequiredError, "%v %v: id «%d» is not raw", row, fld, id)
}

var ErrUnexpectedRawRecordIDError = errors.New("unexpected raw record ID")

func ErrUnexpectedRawRecordID(rec, fld interface{}, id istructs.RecordID) error {
	return enrichError(ErrUnexpectedRawRecordIDError, "%v %v: id «%d» should not be raw", rec, fld, id)
}

var ErrRecordIDUniqueViolationError = errors.New("record ID duplicates")

func ErrRecordIDUniqueViolation(id istructs.RecordID, rec, dupe interface{}) error {
	return enrichError(ErrRecordIDUniqueViolationError, "id «%d» used by %v and %v", id, rec, dupe)
}

// name should  be string or any Stringer interface (e.g. QName)
func ErrSingletonViolation(name interface{}) error {
	return enrichError(ErrRecordIDUniqueViolationError, "singleton «%v» violation", name)
}

var ErrWrongRecordIDError = errors.New("wrong record ID")

func ErrWrongRecordID(arg any, args ...any) error {
	return enrichError(ErrWrongRecordIDError, arg, args...)
}

func ErrWrongRecordIDTarget(t, f interface{}, id istructs.RecordID, target interface{}) error {
	return enrichError(ErrWrongRecordIDError, "%v %v refers to record ID «%d» that has wrong target «%s»", t, f, id, target)
}

var ErrUnableToUpdateSystemFieldError = errors.New("unable to update system field")

func ErrUnableToUpdateSystemField(t, f interface{}) error {
	return enrichError(ErrUnableToUpdateSystemFieldError, "%v %v", t, f)
}

var ErrAbstractTypeError = errors.New("abstract type")

func ErrAbstractType(arg any, args ...any) error {
	return enrichError(ErrAbstractTypeError, arg, args...)
}

var ErrUnexpectedTypeError = errors.New("unexpected type")

func ErrUnexpectedType(arg any, args ...any) error {
	return enrichError(ErrUnexpectedTypeError, arg, args...)
}

var ErrUnknownCodecError = errors.New("unknown codec")

func ErrUnknownCodec(arg any, args ...any) error {
	return enrichError(ErrUnknownCodecError, arg, args...)
}

var ErrMaxGetBatchRecordCountExceeds = errors.New("the maximum count of records to batch is exceeded")

var ErrWrongFieldTypeError = errors.New("wrong field type")

func ErrWrongFieldType(arg any, args ...any) error {
	return enrichError(ErrWrongFieldTypeError, arg, args...)
}

var ErrTypeChanged = errors.New("type has been changed")

var ErrDataConstraintViolation = errors.New("data constraint violation")

var ErrNumAppWorkspacesNotSet = errors.New("NumAppWorkspaces is not set")

var ErrCorruptedData = errors.New("corrupted data")

const (
	errWrongFieldValue        = "field «%v» value should be %s, but got %T"
	errFieldValueTypeConvert  = "field «%s» value type «%T» can not to be converted to «%s»"
	errFieldMustBeVerified    = "field «%s» must be verified, token expected, but value «%T» passed"
	errFieldValueTypeMismatch = "value type «%s» is not applicable for %v"
)

const errNumberFieldWrongValueWrap = "field «%s» value %s can not to be converted to «%s»: %w"

const errCantGetFieldQNameIDWrap = "QName field «%s» can not get ID for value «%v»: %w"

const errMustValidatedBeforeStore = "%v must be validated before store: %w"

const errViewNotFoundWrap = "view «%v» not found: %w"

const errFieldDataConstraintViolatedFmt = "%v data constraint «%v» violated: %w"

// ValidateError: an interface for describing errors that occurred during validation
//   - methods:
//     — Code(): returns error code, see ECode_××× constants
type ValidateError interface {
	error
	Code() int
}
