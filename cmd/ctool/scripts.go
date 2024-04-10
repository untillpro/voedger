/*
* Copyright (c) 2023-present Sigma-Soft, Ltd.
* @author Dmitry Molchanovsky
 */
package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/untillpro/goutils/exec"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"golang.org/x/crypto/ssh/terminal"
)

//go:embed scripts/drafts/*
var scriptsFS embed.FS

var scriptsTempDir string

var indicator []string

type scriptExecuterType struct {
	outputPrefix string
	sshKeyPath   string
}

func selectIndicator() []string {
	indicators1 := []string{"|", "/", "-", "\\"}
	indicators2 := []string{"◐", "◓", "◑", "◒"}
	indicators3 := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

	indicators := [][]string{indicators1, indicators2, indicators3}
	// nolint
	randomIndex := rand.Intn(len(indicators))
	return indicators[randomIndex]
}

func showProgress(done chan bool) {

	if len(indicator) == 0 {
		indicator = selectIndicator()
	}

	i := 0
	for {
		select {
		case <-done:
			fmt.Print("\r")
			return
		default:
			if !verbose() {
				fmt.Printf(green("\r%s\r"), indicator[i])
			}
			i = (i + 1) % len(indicator)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func verbose() bool {
	if dryRun {
		return true
	}
	b, err := rootCmd.Flags().GetBool("verbose")
	return err == nil && b
}

func (se *scriptExecuterType) run(scriptName string, args ...string) error {

	var pExec *exec.PipedExec

	// nolint
	os.Chdir(scriptsTempDir)

	args = append([]string{scriptName}, args...)
	pExec = new(exec.PipedExec).Command("bash", args...)

	var stdoutWriter io.Writer
	var stderrWriter io.Writer
	if logFile != nil {
		if verbose() {
			stdoutWriter = io.MultiWriter(os.Stdout, logFile)
			stderrWriter = io.MultiWriter(os.Stderr, logFile)
		} else {
			stdoutWriter = logFile
			stderrWriter = logFile
		}
	} else {
		if verbose() {
			stdoutWriter = os.Stdout
			stderrWriter = os.Stderr
		} else {
			stdoutWriter = nil
			stderrWriter = nil
		}
	}

	done := make(chan bool)
	go showProgress(done)
	defer func() { done <- true }()

	var err error
	if len(se.outputPrefix) > 0 {
		sedArg := fmt.Sprintf("s/^/[%s]: /", se.outputPrefix)
		err = pExec.
			Command("sed", sedArg).
			Run(stdoutWriter, stderrWriter)
	} else {
		err = pExec.
			Run(stdoutWriter, stderrWriter)
	}

	if err != nil && verbose() {
		loggerError(fmt.Errorf("the error of the script %s: %w", scriptName, err).Error())
	}
	return err
}

func newScriptExecuter(sshKey string, outputPrefix string) *scriptExecuterType {
	return &scriptExecuterType{sshKeyPath: sshKey, outputPrefix: outputPrefix}
}

// nolint
func getEnvValue1(key string) string {
	value, _ := os.LookupEnv(key)
	return value
}

func scriptExists(scriptFileName string) (bool, error) {
	if scriptsTempDir == "" {
		return false, nil
	}

	exists, err := coreutils.Exists(filepath.Join(scriptsTempDir, scriptFileName))
	if err != nil {
		// notest
		return false, err
	}
	return exists, nil
}

func prepareScripts(scriptFileNames ...string) error {

	// nolint
	os.Chdir(scriptsTempDir)

	err := createScriptsTempDir()
	if err != nil {
		return err
	}

	// If scriptfilenames is empty, then we will copy all scripts from scriptsfs
	if len(scriptFileNames) == 0 {
		err = extractAllScripts()
		if err != nil {
			loggerError(err.Error())
			return err
		}
		return nil
	}

	for _, fileName := range scriptFileNames {

		exists, err := scriptExists(fileName)
		if err != nil {
			// notest
			return err
		}
		if exists {
			continue
		}

		file, err := scriptsFS.Open("./scripts/drafts/" + fileName)
		if err != nil {
			return err
		}
		defer file.Close()

		destFileName := filepath.Join(scriptsTempDir, fileName)

		dir := filepath.Dir(destFileName)

		// nolint
		err = os.MkdirAll(dir, rwxrwxrwx) // os.ModePerm)
		if err != nil {
			return err
		}

		newFile, err := os.Create(destFileName)
		if err != nil {
			return err
		}

		defer newFile.Close()
		if err = os.Chmod(destFileName, rwxrwxrwx); err != nil {
			return err
		}

		if _, err = io.Copy(newFile, file); err != nil {
			return err
		}

	}

	return nil
}

// save all the embedded scripts into the temporary folder
func extractAllScripts() error {
	return fs.WalkDir(scriptsFS, "scripts/drafts", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			content, err := fs.ReadFile(scriptsFS, path)
			if err != nil {
				return err
			}
			destPath := filepath.Join(scriptsTempDir, strings.TrimPrefix(path, "scripts/drafts"))
			err = os.MkdirAll(filepath.Dir(destPath), rwxrwxrwx)
			if err != nil {
				return err
			}
			err = os.WriteFile(destPath, content, rwxrwxrwx)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// nolint
func inputPassword(pass *string) error {

	bytePassword, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	if err == nil {
		*pass = string(bytePassword)
		return nil
	}
	return err
}

// nolint
func prepareScriptFromTemplate(scriptFileName string, data interface{}) error {

	err := createScriptsTempDir()
	if err != nil {
		return err
	}

	tmpl, err := template.ParseFS(scriptsFS, filepath.Join(embedScriptsDir, scriptFileName))
	if err != nil {
		return err
	}

	destFilename := filepath.Join(scriptsTempDir, scriptFileName)
	destFile, err := os.Create(destFilename)
	if err != nil {
		return err
	}
	defer destFile.Close()

	err = destFile.Chmod(rw_rw_rw_)
	if err != nil {
		return err
	}

	err = tmpl.Execute(destFile, data)
	if err != nil {
		return err
	}

	return nil
}
