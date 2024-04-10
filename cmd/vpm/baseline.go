/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/untillpro/goutils/exec"
	"github.com/untillpro/goutils/logger"

	"github.com/voedger/voedger/pkg/compile"
)

func newBaselineCmd() *cobra.Command {
	params := vpmParams{}
	cmd := &cobra.Command{
		Use:   "baseline [baseline-folder]",
		Short: "create baseline schemas",
		Args:  showHelpIfLackOfArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			params, err = prepareParams(params, args)
			if err != nil {
				return err
			}
			compileRes, err := compile.Compile(params.Dir)
			if err != nil {
				return err
			}
			return baseline(compileRes, params.Dir, params.TargetDir)
		},
	}
	initGlobalFlags(cmd, &params)
	return cmd
}

// baseline creates baseline schemas in target dir
func baseline(compileRes *compile.Result, dir, targetDir string) error {
	if err := createBaselineDir(targetDir); err != nil {
		return err
	}

	pkgDir := filepath.Join(targetDir, pkgDirName)
	if err := saveBaselineSchemas(compileRes.PkgFiles, pkgDir); err != nil {
		return err
	}

	if err := saveBaselineInfo(compileRes, dir, targetDir); err != nil {
		return err
	}
	return nil
}

func saveBaselineInfo(compileRes *compile.Result, dir, baselineDir string) error {
	var gitCommitHash string
	sb := new(strings.Builder)
	if err := new(exec.PipedExec).Command("git", "rev-parse", "HEAD").WorkingDir(dir).Run(sb, nil); err == nil {
		gitCommitHash = strings.TrimSpace(sb.String())
	}

	baselineInfoObj := baselineInfo{
		BaselinePackageUrl: compileRes.ModulePath,
		Timestamp:          time.Now().In(time.FixedZone("GMT", 0)).Format(timestampFormat),
		GitCommitHash:      gitCommitHash,
	}

	content, err := json.MarshalIndent(baselineInfoObj, "", "  ")
	if err != nil {
		return err
	}

	baselineInfoFilePath := filepath.Join(baselineDir, baselineInfoFileName)
	if err := os.WriteFile(baselineInfoFilePath, content, defaultPermissions); err != nil {
		return err
	}
	if logger.IsVerbose() {
		logger.Verbose("create baseline info file: %s", baselineInfoFilePath)
	}
	return nil
}

func saveBaselineSchemas(pkgFiles packageFiles, baselineDir string) error {
	for qpn, files := range pkgFiles {
		packageDir := filepath.Join(baselineDir, qpn)
		if err := os.MkdirAll(packageDir, defaultPermissions); err != nil {
			return err
		}
		for _, file := range files {
			base := filepath.Base(file)
			fileNameExtensionless := base[:len(base)-len(filepath.Ext(base))]
			filePath := filepath.Join(packageDir, fileNameExtensionless+".vsql")

			fileContent, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			if err := os.WriteFile(filePath, fileContent, defaultPermissions); err != nil {
				return err
			}
			if logger.IsVerbose() {
				logger.Verbose("create baseline file: %s", filePath)
			}
		}
	}
	return nil
}

func createBaselineDir(dir string) error {
	exists, err := exists(dir)
	if err != nil {
		// notest
		return err
	}
	if exists {
		return fmt.Errorf("baseline directory already exists: %s", dir)
	}
	pkgDir := filepath.Join(dir, pkgDirName)
	return os.MkdirAll(pkgDir, defaultPermissions)
}
