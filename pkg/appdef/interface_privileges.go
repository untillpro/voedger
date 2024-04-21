/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Enumeration of privileges.
type PrivilegeKind int8

//go:generate stringer -type=PrivilegeKind -output=stringer_privilegekind.go

const (
	PrivilegeKind_null PrivilegeKind = iota

	// # Privilege to insert records or view records.
	// 	- Privilege applicable on records, view records or workspaces.
	// 	- Then applied to workspaces, it means insert on all tables and views of the workspace.
	// 	- Fields are not applicable.
	PrivilegeKind_Insert

	// # Privilege to update records or view records.
	// 	- Privilege applicable on records, view records or workspaces.
	// 	- Then applied to workspaces, it means update on all tables and views of the workspace.
	// 	- Fields are applicable and specify fields of records or view records that can be updated.
	PrivilegeKind_Update

	// # Privilege to select records or view records.
	// 	- Privilege applicable on records, view records or workspaces.
	// 	- Then applied to workspaces, it means select on all tables and views of the workspace.
	// 	- Fields are applicable and specify fields of records or view records that can be selected.
	PrivilegeKind_Select

	// # Privilege to execute functions.
	// 	- Privilege applicable on commands, queries or workspaces.
	// 	- Then applied to workspaces, it means execute on all queries and commands of the workspace.
	// 	- Fields are not applicable.
	PrivilegeKind_Execute

	// # Privilege to inherit privileges from other roles.
	// 	- Privilege applicable on roles only.
	// 	- Fields are not applicable.
	PrivilegeKind_Inherits
)

// Represents a privilege (specific rights or permissions) to be granted to role or revoked from.
type IPrivilege interface {
	IWithComments

	// Returns privilege kind
	Kind() PrivilegeKind

	// Returns is privilege has been granted. The opposite of `IsRevoked()`
	IsGranted() bool

	// Returns is privilege has been revoked. The opposite of `IsGranted()`
	IsRevoked() bool

	// Returns objects names on which privilege was applied.
	//
	// # For PrivilegeKind_Insert, GrantKind_Update and GrantKind_Select:
	//	- records or view records names or
	//	- workspaces names.
	//
	// # For PrivilegeKind_Execute:
	//	- commands & queries names or
	//	- workspaces names.
	//
	// # For PrivilegeKind_Inherits:
	//	- roles names.
	On() QNames

	// Returns fields (of objects) which was granted or revoked.
	//
	// For PrivilegeKind_Update and PrivilegeKind_Select returns field names of records or view records.
	Fields() []FieldName

	// Returns the role to which the privilege was granted or revoked.
	Role() IRole
}

// IWithPrivileges is an interface for entities that have grants.
type IWithPrivileges interface {
	// Enumerates all privileges.
	//
	// Privileges are enumerated in alphabetical order of roles, and within each role in the order they are added.
	Privileges(func(IPrivilege))

	// Enumerates all privileges with specified kind.
	PrivilegesByKind(PrivilegeKind, func(IPrivilege))

	// Returns all privileges for entity with specified QName.
	PrivilegesFor(QName) []IPrivilege
}

type IPrivilegesBuilder interface {
	// Grants new privilege with specified kind on specified objects to specified role.
	//
	// # Panics:
	//   - if kind is PrivilegeKind_null,
	//	 - if objects are empty,
	//	 - if objects contains unknown names,
	//	 - if fields contains unknown names,
	//   - if role is unknown.
	Grant(kind PrivilegeKind, on []QName, fields []FieldName, toRole QName, comment ...string) IPrivilegesBuilder

	// Grants all available privileges on specified objects to specified role.
	//
	// If the objects are records or view records, then insert, update, and select privileges are granted.
	//
	// If the objects are commands or queries, their execution is granted.
	//
	// If the objects are workspaces, then:
	//	- insert, update and select records and view records of these workspaces are granted,
	//	- execution of commands & queries from these workspaces is granted.
	//
	// If the objects are roles, then all privileges from these roles are granted to specified role.
	GrantAll(on []QName, toRole QName, comment ...string) IPrivilegesBuilder

	// Revokes privilege with specified kind on specified objects from specified role.
	//
	// # Panics:
	//   - if kind is PrivilegeKind_null,
	//	 - if objects are empty,
	//	 - if objects contains unknown names,
	//	 - if fields contains unknown names,
	//   - if role is unknown.
	Revoke(kind PrivilegeKind, on []QName, fields []FieldName, fromRole QName, comment ...string) IPrivilegesBuilder
}
