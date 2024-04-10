/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdefcompat"
	"github.com/voedger/voedger/pkg/compile"
	"github.com/voedger/voedger/pkg/parser"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func newCompatCmd() *cobra.Command {
	params := vpmParams{}
	cmd := &cobra.Command{
		Use:   "compat [baseline-folder]",
		Short: "check backward compatibility",
		Args:  showHelpIfLackOfArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			params, err = prepareParams(params, args)
			if err != nil {
				return err
			}
			ignores, err := readIgnoreFile(params.IgnoreFile)
			if err != nil {
				return err
			}
			compileRes, err := compile.Compile(params.Dir)
			if err != nil {
				return err
			}
			return compat(compileRes, params, ignores)
		},
	}
	initGlobalFlags(cmd, &params)
	cmd.Flags().StringVarP(&params.IgnoreFile, "ignore", "", "", "path to yaml file which contains list of errors to be ignored")
	return cmd
}

// compat checks compatibility of schemas in dir versus baseline schemas in target dir
func compat(compileRes *compile.Result, params vpmParams, ignores [][]string) error {
	baselineDir := params.TargetDir
	var errs []error
	baselineAppDef, err := appDefFromBaselineDir(baselineDir)
	if err != nil {
		errs = append(errs, coreutils.SplitErrors(err)...)
	}

	if baselineAppDef != nil && compileRes.AppDef != nil {
		compatErrs := appdefcompat.CheckBackwardCompatibility(baselineAppDef, compileRes.AppDef)
		compatErrs = appdefcompat.IgnoreCompatibilityErrors(compatErrs, ignores)
		errObjs := make([]error, len(compatErrs.Errors))
		for i, err := range compatErrs.Errors {
			errObjs[i] = err
		}
		errs = append(errs, errObjs...)
	}
	return errors.Join(errs...)
}

// readIgnoreFile reads yaml file and returns list of errors to be ignored
func readIgnoreFile(ignoreFilePath string) ([][]string, error) {
	if ignoreFilePath != "" {
		content, err := os.ReadFile(ignoreFilePath)
		if err != nil {
			return nil, err
		}

		var ignoreInfoObj ignoreInfo
		if err := yaml.Unmarshal(content, &ignoreInfoObj); err != nil {
			return nil, err
		}
		return splitIgnorePaths(ignoreInfoObj.Ignore), nil
	}
	return nil, nil
}

// appDefFromBaselineDir builds app def from baseline dir
func appDefFromBaselineDir(baselineDir string) (appdef.IAppDef, error) {
	var errs []error

	pkgDirPath := filepath.Join(baselineDir, pkgDirName)
	pkgDirPathExists, err := exists(pkgDirPath)
	if err != nil {
		// notest
		return nil, err
	}
	if !pkgDirPathExists {
		return nil, fmt.Errorf("baseline directory does not contain %s subdirectory", pkgDirName)
	}
	baselineJsonFilePath := filepath.Join(baselineDir, baselineInfoFileName)
	baselineJsonFilePathExists, err := exists(baselineJsonFilePath)
	if err != nil {
		// notest
		return nil, err
	}
	if !baselineJsonFilePathExists {
		return nil, fmt.Errorf("baseline directory does not contain %s file", baselineInfoFileName)
	}

	// gather schema files from baseline dir
	var schemaFiles []string
	if err := filepath.Walk(pkgDirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".vsql" || filepath.Ext(path) == ".sql") {
			schemaFiles = append(schemaFiles, path)
		}
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	// form package files structure
	pkgFiles := make(packageFiles)
	for _, schemaFile := range schemaFiles {
		dir := filepath.Dir(schemaFile)
		qpn, err := filepath.Rel(pkgDirPath, dir)
		if err != nil {
			return nil, err
		}
		qpn = strings.ReplaceAll(qpn, "\\", "/")
		pkgFiles[qpn] = append(pkgFiles[qpn], schemaFile)
	}

	// build package ASTs from schema files
	packageASTs := make([]*parser.PackageSchemaAST, 0)
	for qpn, files := range pkgFiles {
		// build file ASTs
		var fileASTs []*parser.FileSchemaAST
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				errs = append(errs, err)
			}
			fileName := filepath.Base(file)

			fileAST, err := parser.ParseFile(fileName, string(content))
			if err != nil {
				errs = append(errs, err)
			}
			fileASTs = append(fileASTs, fileAST)
		}

		// build package AST
		packageAST, err := parser.BuildPackageSchema(qpn, fileASTs)
		if err != nil {
			errs = append(errs, err)
		}
		// add package AST to list
		packageASTs = append(packageASTs, packageAST)
	}

	// build app AST
	appAST, err := parser.BuildAppSchema(packageASTs)
	if err != nil {
		errs = append(errs, err)
	}
	// build app def from app AST
	if appAST != nil {
		builder := appdef.New()
		if err := parser.BuildAppDefs(appAST, builder); err != nil {
			errs = append(errs, err)
		}
		appDef, err := builder.Build()
		if err != nil {
			errs = append(errs, err)
		}
		return appDef, errors.Join(errs...)
	}
	return nil, errors.Join(errs...)
}

// splitIgnorePaths splits list of ignore paths into list of path parts
func splitIgnorePaths(ignores []string) (res [][]string) {
	res = make([][]string, len(ignores))
	for i, ignore := range ignores {
		res[i] = strings.Split(ignore, "/")
	}
	return
}

func showHelpIfLackOfArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return cmd.Help()
		}
		return nil
	}
}
