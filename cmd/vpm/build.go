/*
 * Copyright (c) 2024-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/spf13/cobra"
	"github.com/voedger/voedger/pkg/compile"
	"github.com/voedger/voedger/pkg/goutils/exec"
	"github.com/voedger/voedger/pkg/goutils/logger"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func newBuildCmd(params *vpmParams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build [-C] [-o <archive-name>]",
		Short: "build",
		RunE: func(cmd *cobra.Command, args []string) error {
			exists, err := checkPackageGenFileExists(params.Dir)
			if err != nil {
				return err
			}
			if !exists {
				return errors.New("packages_gen.go not found. Run 'vpm init'")
			}

			compileRes, err := compile.CompileNoDummyApp(params.Dir)
			if err := checkAppSchemaNotFoundErr(err); err != nil {
				return err
			}
			if err := checkCompileResult(compileRes); err != nil {
				return err
			}
			return build(compileRes, params)
		},
	}
	cmd.SilenceErrors = true
	cmd.Flags().StringVarP(&params.Dir, "change-dir", "C", "", "Change to dir before running the command. Any files named on the command line are interpreted after changing directories. If used, this flag must be the first one in the command line.")
	cmd.Flags().StringVarP(&params.Output, "output", "o", "", "output archive name")
	return cmd
}

func checkAppSchemaNotFoundErr(err error) error {
	if err != nil {
		logger.Error(err)
		if errors.Is(err, compile.ErrAppSchemaNotFound) {
			return errors.New("failed to build, app schema not found")
		}
	}
	return nil
}

func checkCompileResult(compileRes *compile.Result) error {
	switch {
	case compileRes == nil:
		return errors.New("failed to compile, check schemas")
	case len(compileRes.NotFoundDeps) > 0:
		return errors.New("failed to compile, missing dependencies. Run 'vpm tidy'")
	default:
		return nil
	}
}

func build(compileRes *compile.Result, params *vpmParams) error {
	// temp directory to save the build info: vsql files, wasm files
	tempBuildInfoDir := filepath.Join(os.TempDir(), uuid.New().String(), buildDirName)
	if err := os.MkdirAll(tempBuildInfoDir, coreutils.FileMode_rwxrwxrwx); err != nil {
		return err
	}
	// create temp build info directory along with vsql and wasm files
	if err := buildDir(compileRes.PkgFiles, tempBuildInfoDir); err != nil {
		return err
	}
	// set the path to the output archive, e.g. app.var
	archiveName := params.Output
	if archiveName == "" {
		archiveName = filepath.Base(params.Dir)
	}
	if !strings.HasSuffix(archiveName, ".var") {
		archiveName += ".var"
	}
	archivePath := filepath.Join(params.Dir, archiveName)

	// zip build info directory along with vsql and wasm files
	return coreutils.Zip(archivePath, tempBuildInfoDir)
}

// buildDir creates a directory structure with vsql and wasm files
func buildDir(pkgFiles packageFiles, buildDirPath string) error {
	for qpn, files := range pkgFiles {
		pkgBuildDir := filepath.Join(buildDirPath, qpn)
		if err := os.MkdirAll(pkgBuildDir, coreutils.FileMode_rwxrwxrwx); err != nil {
			return err
		}

		for _, file := range files {
			// copy vsql files
			base := filepath.Base(file)
			fileNameExtensionless := base[:len(base)-len(filepath.Ext(base))]

			if err := coreutils.CopyFile(file, pkgBuildDir); err != nil {
				return fmt.Errorf(errFmtCopyFile, file, err)
			}

			// building wasm files: if wasm directory exists, build wasm file and copy it to the temp build directory
			fileDir := filepath.Dir(file)
			wasmDirPath := filepath.Join(fileDir, wasmDirName)
			exists, err := coreutils.Exists(wasmDirPath)
			if err != nil {
				return err
			}
			if exists {
				appName := filepath.Base(fileDir)
				wasmFilePath, err := execTinyGoBuild(wasmDirPath, appName)
				if err != nil {
					return err
				}
				if err := coreutils.CopyFile(wasmFilePath, pkgBuildDir); err != nil {
					return fmt.Errorf(errFmtCopyFile, wasmFilePath, err)
				}
				// remove the wasm file after copying it to the build directory
				if err := os.Remove(wasmFilePath); err != nil {
					return err
				}
			}

		}
	}
	return nil
}

// execTinyGoBuild builds the project using tinygo and returns the path to the resulting wasm file
func execTinyGoBuild(dir, appName string) (wasmFilePath string, err error) {
	var stdout io.Writer
	if logger.IsVerbose() {
		stdout = os.Stdout
	}

	wasmFileName := appName + ".wasm"
	if err := new(exec.PipedExec).Command("tinygo", "build", "--no-debug", "-o", wasmFileName, "-scheduler=none", "-opt=2", "-gc=leaking", "-target=wasi", ".").WorkingDir(dir).Run(stdout, os.Stderr); err != nil {
		return "", err
	}
	return filepath.Join(dir, wasmFileName), nil
}
