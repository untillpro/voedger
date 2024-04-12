/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

// Application definition is a set of types, views, commands, queries and workspaces.
type IAppDef interface {
	IWithComment
	IWithPackages
	IWithTypes
	IWithDataTypes
	IWithGDocs
	IWithCDocs
	IWithWDocs
	IWithSingletons

	// Return ODoc by name.
	//
	// Returns nil if not found.
	ODoc(name QName) IODoc

	// Return ORecord by name.
	//
	// Returns nil if not found.
	ORecord(name QName) IORecord

	// Return Object by name.
	//
	// Returns nil if not found.
	Object(name QName) IObject

	// Return record by name.
	//
	// Returns nil if not found.
	Record(QName) IRecord

	// Enumerates all application records, e.g. documents and contained records
	//
	// Records are enumerated in alphabetical order by QName
	Records(func(IRecord))

	// Enumerates all application structures
	//
	// Structures are enumerated in alphabetical order by QName
	Structures(func(IStructure))

	// Return View by name.
	//
	// Returns nil if not found.
	View(name QName) IView

	// Returns Command by name.
	//
	// Returns nil if not found.
	Command(QName) ICommand

	// Returns Query by name.
	//
	// Returns nil if not found.
	Query(QName) IQuery

	// Return projector by name.
	//
	// Returns nil if not found.
	Projector(QName) IProjector

	// Enumerates all application projectors.
	//
	// Projectors are enumerated in alphabetical order by QName.
	Projectors(func(IProjector))

	// Return extension by name.
	//
	// Returns nil if not found.
	Extension(QName) IExtension

	// Enumerates all application extensions (commands, queries and extensions)
	//
	// Extensions are enumerated in alphabetical order by QName
	Extensions(func(IExtension))

	// Returns workspace by name.
	//
	// Returns nil if not found.
	Workspace(QName) IWorkspace

	// Returns workspace by descriptor.
	//
	// Returns nil if not found.
	WorkspaceByDescriptor(QName) IWorkspace
}

type IAppDefBuilder interface {
	ICommentBuilder
	IPackagesBuilder
	IDataTypesBuilder
	IGDocsBuilder
	ICDocsBuilder
	IWDocsBuilder

	// Adds new ODoc type with specified name.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddODoc(name QName) IODocBuilder

	// Adds new ORecord type with specified name.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddORecord(name QName) IORecordBuilder

	// Adds new Object type with specified name.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddObject(name QName) IObjectBuilder

	// Adds new types for view.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddView(QName) IViewBuilder

	// Adds new command.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddCommand(QName) ICommandBuilder

	// Adds new query.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddQuery(QName) IQueryBuilder

	// Adds new projector.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddProjector(QName) IProjectorBuilder

	// Adds new workspace.
	//
	// # Panics:
	//   - if name is empty (appdef.NullQName),
	//   - if name is invalid,
	//   - if type with name already exists.
	AddWorkspace(QName) IWorkspaceBuilder

	// Returns application definition while building.
	//
	// Can be called before or after all entities added.
	// Does not validate application definition, some types may be invalid.
	AppDef() IAppDef

	// Builds application definition.
	//
	// Validates and returns builded application type or error.
	// Must be called after all entities added.
	Build() (IAppDef, error)
}
