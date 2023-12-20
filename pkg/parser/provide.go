/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */
package parser

import (
	"errors"

	"github.com/voedger/voedger/pkg/appdef"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

// ParseFile parses content of the single file, creates FileSchemaAST and returns pointer to it.
// Performs syntax analysis
func ParseFile(fileName, content string) (*FileSchemaAST, error) {
	ast, err := parseImpl(fileName, content)
	if err != nil {
		return nil, err
	}
	return &FileSchemaAST{
		FileName: fileName,
		Ast:      ast,
	}, nil
}

// BuildPackageSchema merges File Schema ASTs into Package Schema AST.
// Performs package-level semantic analysis
func BuildPackageSchema(qualifiedPackageName string, asts []*FileSchemaAST) (*PackageSchemaAST, error) {
	return buildPackageSchemaImpl(qualifiedPackageName, asts)
}

// ParsePackageDir is a helper which parses all SQL schemas from specified FS and returns Package Schema.
func ParsePackageDir(qualifiedPackageName string, fs IReadFS, subDir string) (ast *PackageSchemaAST, err error) {
	ast, _, err = ParsePackageDirCollectingFiles(qualifiedPackageName, fs, subDir)
	return
}

// ParsePackageDirCollectingFiles is a helper which parses all SQL schemas from specified FS
// Returns package schema and list of schema file names which were parsed
func ParsePackageDirCollectingFiles(qualifiedPackageName string, fs IReadFS, subDir string) (*PackageSchemaAST, []string, error) {
	asts, errs := parseFSImpl(fs, subDir)
	fileNames := make([]string, len(asts))
	for i, fileAst := range asts {
		fileNames[i] = fileAst.FileName
	}
	packageAst, packageBuildErr := BuildPackageSchema(qualifiedPackageName, asts)
	if packageBuildErr != nil {
		errs = append(errs, coreutils.SplitErrors(packageBuildErr)...)
	}
	return packageAst, fileNames, errors.Join(errs...)
}

// Application-level semantic analysis (e.g. cross-package references)
func BuildAppSchema(packages []*PackageSchemaAST) (*AppSchemaAST, error) {
	return buildAppSchemaImpl(packages)
}

func BuildAppDefs(appSchema *AppSchemaAST, builder appdef.IAppDefBuilder) error {
	return buildAppDefs(appSchema, builder)
}

func BuildAppDefFromFS(qualifiedPackageName string, fs IReadFS, subDir string) (appdef.IAppDef, error) {
	packageAst, err := ParsePackageDir(qualifiedPackageName, fs, subDir)
	if err != nil {
		return nil, err
	}
	appSchema, err := BuildAppSchema([]*PackageSchemaAST{packageAst})
	if err != nil {
		return nil, err
	}

	adb := appdef.New()
	err = BuildAppDefs(appSchema, adb)
	if err != nil {
		return nil, err
	}

	return adb.Build()
}
