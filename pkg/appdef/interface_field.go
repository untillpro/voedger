/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Field Verification kind
type VerificationKind uint8

//go:generate stringer -type=VerificationKind -output=stringer_verificationkind.go

const (
	VerificationKind_EMail VerificationKind = iota
	VerificationKind_Phone
	VerificationKind_FakeLast
)

var VerificationKind_Any = []VerificationKind{VerificationKind_EMail, VerificationKind_Phone}

// Final types with fields are:
//	- TypeKind_GDoc and TypeKind_GRecord,
//	- TypeKind_CDoc and TypeKind_CRecord,
//	- TypeKind_ODoc and TypeKind_CRecord,
//	- TypeKind_WDoc and TypeKind_WRecord,
//	- TypeKind_Object and TypeKind_Element,
//	- TypeKind_ViewRecord
type IFields interface {
	// Finds field by name.
	//
	// Returns nil if not found.
	Field(name string) IField

	// Returns fields count
	FieldCount() int

	// All fields in add order.
	Fields() []IField

	// Finds reference field by name.
	//
	// Returns nil if field is not found, or field found but it is not a reference field
	RefField(name string) IRefField

	// All reference fields. System field (sys.ParentID) is also included
	RefFields() []IRefField

	// Returns user fields count. System fields (sys.QName, sys.ID, …) do not count
	UserFieldCount() int
}

type IFieldsBuilder interface {
	IFields

	// Adds field specified name and kind.
	//
	// # Panics:
	//   - if name is empty,
	//   - if name is invalid,
	//   - if field with name is already exists,
	//   - if specified data kind is not allowed by structured type kind,
	//	 - if constraints are not compatible with specified data type.
	AddField(name string, kind DataKind, required bool, constraints ...IConstraint) IFieldsBuilder

	// Adds field with specified data type.
	//
	// If constraints specified, then new anonymous data type inherits from specified
	// type will be created and this new type will be used as field data type.
	//
	// # Panics:
	//   - if field name is empty,
	//   - if field name is invalid,
	//   - if field with name is already exists,
	//   - if specified data type is not found,
	//   - if specified data kind is not allowed by structured type kind,
	//	 - if constraints are not compatible with specified data type.
	AddDataField(name string, data QName, required bool, constraints ...IConstraint) IFieldsBuilder

	// Adds reference field specified name and target refs.
	//
	// # Panics:
	//   - if name is empty,
	//   - if name is invalid,
	//   - if field with name is already exists.
	AddRefField(name string, required bool, ref ...QName) IFieldsBuilder

	// Sets fields comment.
	// Useful for reference or verified fields, what Add×××Field has not comments
	// argument.
	//
	// # Panics:
	//   - if field not found.
	SetFieldComment(name string, comment ...string) IFieldsBuilder

	// Sets verification kind for specified field.
	//
	// If not verification kinds are specified then it means that field is not verifiable.
	//
	// # Panics:
	//   - if field not found.
	SetFieldVerify(name string, vk ...VerificationKind) IFieldsBuilder
}

// Describe single field.
type IField interface {
	IComment

	// Returns field name
	Name() string

	// Returns data type
	Data() IData

	// Returns data kind for field
	DataKind() DataKind

	// Returns is field required
	Required() bool

	// Returns is field verifiable
	Verifiable() bool

	// Returns is field verifiable by specified verification kind
	VerificationKind(VerificationKind) bool

	// Returns is field has fixed width data kind
	IsFixedWidth() bool

	// Returns is field system
	IsSys() bool

	// All field constraints.
	//
	// Result contains throughout the data types hierarchy, include all ancestors recursively.
	// If any constraint (for example `MinLen`) is specified by the ancestor, but redefined in the descendant,
	// then the constraint from the descendant will include in result.
	Constraints() map[ConstraintKind]IConstraint
}

// Reference field. Describe field with DataKind_RecordID.
//
// Use Refs() to obtain list of target references.
type IRefField interface {
	IField

	// Returns list of target references
	Refs() QNames

	// Returns, is the link available
	Ref(QName) bool
}
