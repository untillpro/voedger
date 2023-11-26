/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package istructsmem

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
)

// https://github.com/voedger/voedger/issues/673
// TODO: remove it after switching to func declaration in sql ony
func ReplaceCommandDefinitions(cmd istructs.ICommandFunction, params, unlogged, result appdef.QName) {
	cf := cmd.(*commandFunction)
	cf.pars = params
	cf.unlPars = unlogged
	cf.res = func(pa istructs.PrepareArgs) appdef.QName { return result }
}

// https://github.com/voedger/voedger/issues/673
// TODO: remove it after switching to func declaration in sql ony
func ReplaceQueryDefinitions(query istructs.IQueryFunction, pars appdef.QName, result appdef.QName) {
	qf := query.(*queryFunction)
	qf.pars = pars
	qf.res = func(pa istructs.PrepareArgs) appdef.QName { return result }
}
