/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package dm

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/untillpro/goutils/logger"
	"golang.org/x/mod/modfile"
)

type goImpl struct {
	cachePath     string
	goModFilePath string
	modFile       *modfile.File
}

func (g *goImpl) LocalPath(depURL string) (localDepPath string, err error) {
	if logger.IsVerbose() {
		logger.Verbose(fmt.Sprintf("locating dependency %s", depURL))
	}
	pkgURL, subDir, version, ok := g.parseDepURL(depURL)
	if !ok {
		return "", fmt.Errorf("unknown dependency %s", depURL)
	}
	if version == "" {
		localDepPath = path.Join(path.Dir(g.goModFilePath), subDir)
		return
	}
	localDepPath = path.Join(g.cachePath, fmt.Sprintf("%s@%s", pkgURL, version), subDir)
	if _, err := os.Stat(localDepPath); os.IsNotExist(err) {
		if err := downloadDependencies(g.goModFilePath); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(localDepPath); os.IsNotExist(err) {
		return "", err
	}
	return localDepPath, nil
}

// parseDepURL slices depURL into pkgURL, subDir and version.
// Empty version means depURL belongs to local project
func (g *goImpl) parseDepURL(depURL string) (pkgURL, subDir, version string, ok bool) {
	subDir, ok = matchDepPath(depURL, g.modFile.Module.Mod.Path)
	if ok {
		pkgURL = g.modFile.Module.Mod.Path
		return
	}
	for _, r := range g.modFile.Require {
		subDir, ok = matchDepPath(depURL, r.Mod.Path)
		if ok {
			pkgURL = r.Mod.Path
			version = r.Mod.Version
			return
		}
	}
	return
}

func parseGoModFile(goModPath string) (*modfile.File, error) {
	if logger.IsVerbose() {
		logger.Verbose(fmt.Sprintf("parsing %s", goModPath))
	}
	// TODO: checkout behaviour of modfile if we got replace in go.mod
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s file - %v", goModFile, err)
	}

	modFile, err := modfile.ParseLax(goModPath, content, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s file - %v", goModFile, err)
	}
	return modFile, nil
}

func matchDepPath(depURL, depPath string) (subDir string, ok bool) {
	ok = true
	if strings.HasPrefix(depURL, depPath) && depURL[len(depPath)] == '/' {
		subDir = depURL[len(depPath)+1:]
		return
	}
	if depURL == depPath {
		return
	}
	ok = false
	return
}

func downloadDependencies(goModFilePath string) error {
	if logger.IsVerbose() {
		logger.Verbose("downloading dependencies")
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		if logger.IsVerbose() {
			logger.Verbose(fmt.Sprintf("changing working directory back to %s", wd))
		}
		_ = os.Chdir(wd)
	}()
	if logger.IsVerbose() {
		logger.Verbose(fmt.Sprintf("working directory is %s", wd))
	}

	if err := os.Chdir(path.Dir(goModFilePath)); err != nil {
		return err
	}
	if logger.IsVerbose() {
		logger.Verbose(fmt.Sprintf("changing working directory to %s", path.Dir(goModFilePath)))
	}
	return execGoCmd("mod", "download").Run()
}

func checkGoInstalled() error {
	if logger.IsVerbose() {
		logger.Verbose("checking out for installed go")
	}
	// Check if the "go" executable is in the PATH
	if _, err := exec.LookPath("go"); err != nil {
		// Provide a more informative error message
		return fmt.Errorf("go is not installed or not in the PATH. please install Go https://golang.org/doc/install. Error - %w", err)
	}

	// Optionally, you can check the Go version to ensure it meets your application's requirements
	goVersionOutput, versionErr := execGoCmd("version").Output()
	if versionErr != nil {
		return fmt.Errorf("unable to determine Go version. Error - %v", versionErr)
	}

	// Extract the version information from the output
	versionLine := strings.Split(string(goVersionOutput), " ")[2] // Assuming the version is the third element
	goVersion := strings.TrimSpace(strings.TrimSuffix(versionLine, "\n"))
	goVersion = strings.TrimPrefix(goVersion, "go")
	if goVersion == "" {
		return fmt.Errorf("failed to extract go version from 'go version' output")
	}
	return nil
}

func getCachePath() (string, error) {
	if logger.IsVerbose() {
		logger.Verbose("searching for cache of the packages")
	}
	goPath, ok := os.LookupEnv("GOPATH")
	if !ok {
		return "", fmt.Errorf("GOPATH env var is not defined")
	}
	return path.Join(goPath, "pkg", "mod"), nil
}

func getGoModFile() (*modfile.File, string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}
	var previousDir string
	for currentDir != previousDir {
		if logger.IsVerbose() {
			logger.Verbose(fmt.Sprintf("searching for %s in %s", goModFile, currentDir))
		}
		goModPath := filepath.Join(currentDir, goModFile)
		if _, err := os.Stat(goModPath); err == nil {
			modFile, err := parseGoModFile(goModPath)
			if err != nil {
				return nil, "", err
			}
			if logger.IsVerbose() {
				logger.Verbose(fmt.Sprintf("%s is located at %s", goModFile, currentDir))
			}
			return modFile, goModPath, nil
		}
		previousDir = currentDir
		currentDir = filepath.Dir(currentDir)
	}
	return nil, "", fmt.Errorf("dependency file %s not found", goModFile)
}

func execGoCmd(args ...string) *exec.Cmd {
	if logger.IsVerbose() {
		logger.Verbose(fmt.Sprintf("running: go %s", strings.Join(args, " ")))
	}
	return exec.Command("go", args...)
}
