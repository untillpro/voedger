/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package main

import (
	"errors"
)

const (
	wasmDirName          = "wasm"
	buildDirName         = "build"
	ormDirName           = "orm"
	pkgDirName           = "pkg"
	baselineInfoFileName = "baseline.json"
	timestampFormat      = "Mon, 02 Jan 2006 15:04:05.000 GMT"
	errFmtCopyFile       = "'%s': failed to copy - %w"
)

const (
	gitignoreFileContent = `*
`
	unknownType                  = "Unknown"
	errInGeneratingOrmFileFormat = "error occurred while generating %s: %w"
)

const (
	goModFileName                 = "go.mod"
	goSumFileName                 = "go.sum"
	packagesGenFileName           = "packages_gen.go"
	minimalRequiredGoVersion      = "1.18"
	unsupportedGoVersionErrFormat = "vpm: unsupported go version %s, minimal required go version is " + minimalRequiredGoVersion
	goModContentTemplate          = `module %s

go %s
`
	defaultOrmFilesHeaderComment = "// Code generated by vpm. DO NOT EDIT."
	packagesGenContentTemplate   = defaultOrmFilesHeaderComment + `

package %s

import (
	%s
	_ "github.com/voedger/voedger/pkg/sys"
)

func init() {
	return
}
`
)

var (
	errGoModFileNotFound = errors.New("go.mod file not found. Run 'vpm init'")
)

var sysFields = []string{"sys.ID", "sys.IsActive"}
