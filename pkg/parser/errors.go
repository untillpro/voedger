/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */
package parser

import (
	"errors"
	"fmt"

	"github.com/alecthomas/participle/v2/lexer"

	"github.com/voedger/voedger/pkg/appdef"
)

var ErrDirContainsNoSchemaFiles = errors.New("no schema files in directory")
var ErrNoQualifiedName = errors.New("empty qualified name")
var ErrEmptyFileAstList = errors.New("empty file ast list")
var ErrFunctionParamsIncorrect = errors.New("function parameters do not match")
var ErrFunctionResultIncorrect = errors.New("function result do not match")
var ErrPrimaryKeyRedefined = errors.New("redefinition of primary key")
var ErrApplicationRedefined = errors.New("redefinition of application")
var ErrApplicationNotDefined = errors.New("application not defined")
var ErrPrimaryKeyNotDefined = errors.New("primary key not defined")
var ErrUndefinedTableKind = errors.New("undefined table kind")
var ErrNestedTableIncorrectKind = errors.New("incorrect nested table kind")
var ErrBaseTableMustBeAbstract = errors.New("base table must be abstract")
var ErrBaseWorkspaceMustBeAbstract = errors.New("base workspace must be abstract")
var ErrAbstractWorkspaceDescriptor = errors.New("abstract workspace cannot have a descriptor")
var ErrNestedTablesNotSupportedInTypes = errors.New("nested tables not supported in types")
var ErrSysWorkspaceNotFound = errors.New("sys.Workspace type not found")
var ErrInheritanceFromSysWorkspaceNotAllowed = errors.New("explicit inheritance from sys.Workspace not allowed")

var ErrMustBeNotNull = errors.New("field has to be NOT NULL")
var ErrCircularReferenceInInherits = errors.New("circular reference in INHERITS")
var ErrRegexpCheckOnlyForVarcharField = errors.New("regexp CHECK only available for varchar field")
var ErrMaxFieldLengthTooLarge = fmt.Errorf("maximum field length is %d", appdef.MaxFieldLength)
var ErrOnlyInsertForOdocOrORecord = errors.New("only INSERT allowed for ODoc or ORecord")
var ErrPackageWithSameNameAlreadyIncludedInApp = errors.New("package with the same name already included in application")
var ErrStorageDeclaredOnlyInSys = errors.New("storages are only declared in sys package")
var ErrPkgFolderNotFound = errors.New("pkg folder not found")

func ErrAppDoesNotDefineUseOfPackage(name string) error {
	return fmt.Errorf("application does not define use of package %s", name)
}

func ErrUndefinedCommand(name DefQName) error {
	return fmt.Errorf("undefined command: %s", name.String())
}

func ErrUndefinedQuery(name DefQName) error {
	return fmt.Errorf("undefined query: %s", name.String())
}

func ErrUndefinedRate(name DefQName) error {
	return fmt.Errorf("undefined rate: %s", name)
}

func ErrUndefinedWorkspace(name DefQName) error {
	return fmt.Errorf("undefined workspace: %s", name.String())
}

func ErrUndefinedTag(name DefQName) error {
	return fmt.Errorf("undefined tag: %s", name.String())
}

func ErrUndefinedRole(name DefQName) error {
	return fmt.Errorf("undefined role: %s", name.String())
}

func ErrUndefinedTypeOrOdoc(name DefQName) error {
	return fmt.Errorf("undefined type or ODoc: %s", name.String())
}

func ErrUndefinedTypeOrTable(name DefQName) error {
	return fmt.Errorf("undefined type or table: %s", name.String())
}

func ErrUndefinedDataTypeOrTable(name DefQName) error {
	return fmt.Errorf("undefined data type or table: %s", name.String())
}

func ErrUndefinedType(name DefQName) error {
	return fmt.Errorf("undefined type: %s", name.String())
}

func ErrUndefinedTable(name DefQName) error {
	return fmt.Errorf("undefined table: %s", name.String())
}

func ErrCheckRegexpErr(e error) error {
	return fmt.Errorf("CHECK regexp error:  %w", e)
}

// Golang: could not import github.com/alecthomas/participle/v2/asd (no required module provides package "github.com/alecthomas/participle/v2/asd")
func ErrCouldNotImport(pkgName string) error {
	return fmt.Errorf("could not import %s", pkgName)
}

func ErrUnexpectedRootTableKind(kind int) error {
	return fmt.Errorf("unexpected root table kind %d", kind)
}

func ErrReferenceToAbstractTable(tblName string) error {
	return fmt.Errorf("reference to abstract table %s", tblName)
}

func ErrNestedAbstractTable(tblName string) error {
	return fmt.Errorf("nested abstract table %s", tblName)
}

func ErrUseOfAbstractTable(tblName string) error {
	return fmt.Errorf("use of abstract table %s", tblName)
}

func ErrUseOfAbstractWorkspace(wsName string) error {
	return fmt.Errorf("use of abstract workspace %s", wsName)
}

func ErrWorkspaceIsNotAlterable(wsName string) error {
	return fmt.Errorf("workspace %s is not alterable", wsName)
}

func ErrAbstractTableNotAlowedInProjectors(tblName string) error {
	return fmt.Errorf("projector refers to abstract table %s", tblName)
}

func ErrProjectorDoesNotDeclareViewIntent(projectorName, viewName string) error {
	return fmt.Errorf("projector %s does not declare intent for view %s", projectorName, viewName)
}

func ErrUndefined(name string) error {
	return fmt.Errorf("%s undefined", name)
}

func ErrUndefinedField(name string) error {
	return fmt.Errorf("undefined field %s", name)
}

func ErrFieldAlreadyInUnique(name string) error {
	return fmt.Errorf("field %s already in unique constraint", name)
}

func ErrTypeNotSupported(name string) error {
	return fmt.Errorf("%s type not supported", name)
}

func ErrStorageRequiresEntity(name string) error {
	return fmt.Errorf("storage %s requires entity", name)
}

func ErrStorageNotInProjectorState(name string) error {
	return fmt.Errorf("storage %s is not available in the state of projectors", name)
}

func ErrStorageNotInProjectorIntents(name string) error {
	return fmt.Errorf("storage %s is not available in the intents of projectors", name)
}

func ErrRedefined(name string) error {
	return fmt.Errorf("redefinition of %s", name)
}

func ErrPackageRedeclared(name string) error {
	return fmt.Errorf("package %s redeclared", name)
}

func ErrViewFieldVarchar(name string) error {
	return fmt.Errorf("varchar field %s not supported in partition key", name)
}

func ErrViewFieldBytes(name string) error {
	return fmt.Errorf("bytes field %s not supported in partition key", name)
}

func ErrVarcharFieldInCC(name string) error {
	return fmt.Errorf("varchar field %s can only be the last one in clustering key", name)
}

func ErrBytesFieldInCC(name string) error {
	return fmt.Errorf("bytes field %s can only be the last one in clustering key", name)
}

func errorAt(err error, pos *lexer.Position) error {
	return fmt.Errorf("%s: %w", pos.String(), err)
}
