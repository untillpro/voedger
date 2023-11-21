/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package descr

import "github.com/voedger/voedger/pkg/appdef"

type Extensions struct {
	Commands map[appdef.QName]*CommandFunction `json:",omitempty"`
	Queries  map[appdef.QName]*QueryFunction   `json:",omitempty"`
}

type Extension struct {
	Type
	Name   string
	Engine string
}

type Function struct {
	Extension
	Arg    *appdef.QName `json:",omitempty"`
	Result *appdef.QName `json:",omitempty"`
}

type CommandFunction struct {
	Function
	UnloggedArg *appdef.QName `json:",omitempty"`
}

type QueryFunction struct {
	Function
}

type Projector struct {
	Extension
}
