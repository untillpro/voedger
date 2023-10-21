/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package descr

import (
	"github.com/voedger/voedger/pkg/appdef"
)

type Structure struct {
	Comment     string `json:",omitempty"`
	Name        appdef.QName
	Kind        appdef.TypeKind
	Fields      []*Field     `json:",omitempty"`
	Containers  []*Container `json:",omitempty"`
	Uniques     []*Unique    `json:",omitempty"`
	UniqueField string       `json:",omitempty"`
	Singleton   bool         `json:",omitempty"`
}

type Field struct {
	Comment    string `json:",omitempty"`
	Name       string
	DataType   *Data         `json:",omitempty"`
	Data       *appdef.QName `json:",omitempty"`
	Required   bool          `json:",omitempty"`
	Verifiable bool          `json:",omitempty"`
	Refs       []string      `json:",omitempty"`
}

type Container struct {
	Comment   string `json:",omitempty"`
	Name      string
	Type      appdef.QName
	MinOccurs appdef.Occurs
	MaxOccurs appdef.Occurs
}

type Unique struct {
	Comment string `json:",omitempty"`
	Name    string
	Fields  []string
}
