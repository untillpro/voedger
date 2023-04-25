/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Maxim Geraskin
 */

package sqlschema

import fs "io/fs"

// provide.go
type IReadFS interface {
	fs.ReadDirFS
	fs.ReadFileFS
}

// TODO: in: FS, moduleCache [string]FS, out: (Schema, DependencySchemas [string]Schema, error)

//
type FSParser func(fs IReadFS, subDir string) (*SchemaAST, error)

// input:
//   modulesCache: key: module
// output:
//   schema - parsed package AST
//   deps - dependencies. Key is a fully-qualified package name
// type FSParser func(fs IReadFS, subDir string, modulesCache map[string]IReadFS) (schema *SchemaAST, deps map[string]*SchemaAST, err error)

type StringParser func(fileName string, content string) (*SchemaAST, error)


/*

provide.go


// Syntax analysis
ParseFile(qualifiedPackageName string, string fileName, string content) (FileSchemaAST, error)

// Pakage-level semantic analysis
MergeFileSchemaASTs([]FileSchemaAST) (PackageSchemaAST, error)

// Application-level semantic analysis (e.g. cross-package references)
MergePackageSchemas([]PackageSchema) (schemas.AppSchema, error)

*/