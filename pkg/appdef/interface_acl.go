/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Enumeration of ACL operation kinds.
type OperationKind uint8

//go:generate stringer -type=OperationKind -output=stringer_operationkind.go

const (
	OperationKind_null OperationKind = iota

	// # Insert records or view records.
	// 	- Operation applicable on records, view records.
	// 	- Fields are not applicable.
	OperationKind_Insert

	// # Update records or view records.
	// 	- Operation applicable on records, view records.
	// 	- Fields are applicable and specify fields of records or view records that can be updated.
	OperationKind_Update

	// # Select records or view records.
	// 	- Operation applicable on records, view records.
	// 	- Fields are applicable and specify fields of records or view records that can be selected.
	OperationKind_Select

	// # Execute functions.
	// 	- Operation applicable on commands, queries.
	// 	- Fields are not applicable.
	OperationKind_Execute

	// # Inherit ACL records from other roles.
	// 	- Operation applicable on roles only.
	// 	- Fields are not applicable.
	OperationKind_Inherits

	OperationKind_count
)

// Enumeration of ACL operation policy.
type PolicyKind uint8

//go:generate stringer -type=PolicyKind -output=stringer_policykind.go

const (
	PolicyKind_null PolicyKind = iota

	PolicyKind_Allow

	PolicyKind_Deny

	PolicyKind_count
)

type IACLFilter interface {
	IFilter

	// Returns fields (of records or views) then insert, update or select operation is described.
	// TODO: should return iter.Seq[FieldName]
	Fields() []FieldName
}

// Represents a ACL rule record (specific rights or permissions) to be granted to role or revoked from.
type IACLRule interface {
	IWithComments

	// Returns operations that was granted or revoked.
	Ops() []OperationKind

	// Returns operations are granted or revoked.
	Policy() PolicyKind

	// Returns filter of types on which rule is applicable.
	Filter() IACLFilter

	// Returns the role to which the operations was granted or revoked.
	Principal() IRole

	// Returns workspace where the rule is defined.
	Workspace() IWorkspace
}

// IWithACL is an interface for entities that have ACL.
type IWithACL interface {
	// Enumerates all ACL rules.
	//
	// Rules are enumerated in the order they are added.
	//
	// TODO: should return iter.Seq[IACLRule]
	ACL(func(IACLRule) bool)
}

type IACLBuilder interface {
	// Grants operations on filtered types to role.
	//
	// # Panics:
	//   - if ops is empty,
	//	 - if ops contains incompatible operations (e.g. INSERT with EXECUTE),
	//	 - if filtered type is not compatible with operations,
	//	 - if fields contains unknown names,
	//   - if role is unknown.
	Grant(ops []OperationKind, flt IFilter, fields []FieldName, toRole QName, comment ...string) IACLBuilder

	// Grants all available operations on filtered types to role.
	//
	// If the types are records or view records, then insert, update, and select are granted.
	//
	// If the types are commands or queries, their execution is granted.
	//
	// If the types are roles, then all operations from these roles are granted to specified role.
	//
	// No mixed types are allowed.
	GrantAll(flt IFilter, toRole QName, comment ...string) IACLBuilder

	// Revokes operations on filtered types from role.
	//
	// Revoke inherited roles is not supported
	Revoke(ops []OperationKind, resources IFilter, fields []FieldName, fromRole QName, comment ...string) IACLBuilder

	// Remove all available operations on filtered types from role.
	//
	// Revoke inherited roles is not supported
	RevokeAll(flt IFilter, fromRole QName, comment ...string) IACLBuilder
}
