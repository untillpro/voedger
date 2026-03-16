/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package coreutils

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/voedger/voedger/pkg/istructs"
)

type EmbedFS interface {
	Open(name string) (fs.File, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	ReadFile(name string) ([]byte, error)
}

type PathReader struct {
	rootPath string
}

func NewPathReader(rootPath string) *PathReader {
	return &PathReader{
		rootPath: rootPath,
	}
}

func (r *PathReader) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(r.rootPath, name))
}

func (r *PathReader) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(filepath.Join(r.rootPath, name))
}

func (r *PathReader) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(r.rootPath, name))
}

type IErrUnwrapper interface {
	Unwrap() []error
}

type CUDs []CUD

func (c CUDs) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal([]CUD(c))
	if err != nil {
		return nil, err
	}
	const cuds = `{"cuds":`
	out := make([]byte, 0, len(cuds)+len(b)+1)
	out = append(out, cuds...)
	out = append(out, b...)
	out = append(out, '}')
	return out, nil
}

func (c CUDs) ToJSON() string {
	bb, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(bb)
}

type CUD struct {
	ID     istructs.RecordID      `json:"sys.ID,omitempty"`
	Fields map[string]interface{} `json:"fields"`
}
