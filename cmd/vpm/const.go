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
	sysContent = `// Code generated by vpm. DO NOT EDIT.

package orm

import "github.com/voedger/voedger/pkg/exttinygo"

type QName = string
type Ref int64
func (r Ref) ID() ID { return ID(r) }
type ID int64
type Bytes []byte

const (
	FieldNameEventUnloggedArgumentObject 	= "UnloggedArgumentObject"
	FieldNameEventArgumentObject 			= "ArgumentObject"
	FieldNameSysID               			= "sys.ID"
	FieldName_IsSingleton					= "IsSingleton"
	FieldName_ID                            = "ID"
)

type Type struct {
	qname QName
}

func (t *Type) QName() QName {
	return t.qname
}

type Event struct{}
type Value_CommandContext struct{ tv exttinygo.TValue }

func CommandContext() Value_CommandContext {
	kb := exttinygo.KeyBuilder(exttinygo.StorageCommandContext, exttinygo.NullEntity)
	return Value_CommandContext{tv: exttinygo.MustGetValue(kb)}
}
`
	unknownType                  = "Unknown"
	defaultOrmFilesHeaderComment = "// Code generated by vpm. DO NOT EDIT."
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
	packagesGenContentTemplate = defaultOrmFilesHeaderComment + `

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
	errGoModFileNotFound = errors.New("go.mod file not found")
)

var sysFields = []string{"sys.ID", "sys.IsActive"}
