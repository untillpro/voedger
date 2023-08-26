/*
* Copyright (c) 2023-present unTill Pro, Ltd.
* @author Michael Saigachenko
 */

package parser

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
)

const (
	nameCDOC      = "CDoc"
	nameODOC      = "ODoc"
	nameWDOC      = "WDoc"
	nameSingleton = "Singleton"
	nameCRecord   = "CRecord"
	nameORecord   = "ORecord"
	nameWRecord   = "WRecord"
)

const rootWorkspaceName = "Workspace"

const maxNestedTableContainerOccurrences = 100 // FIXME: 100 container occurrences

var canNotReferenceTo = map[appdef.DefKind][]appdef.DefKind{
	appdef.DefKind_ODoc:    {},
	appdef.DefKind_ORecord: {},
	appdef.DefKind_WDoc:    {appdef.DefKind_ODoc, appdef.DefKind_ORecord},
	appdef.DefKind_WRecord: {appdef.DefKind_ODoc, appdef.DefKind_ORecord},
	appdef.DefKind_CDoc:    {appdef.DefKind_WDoc, appdef.DefKind_WRecord, appdef.DefKind_ODoc, appdef.DefKind_ORecord},
	appdef.DefKind_CRecord: {appdef.DefKind_WDoc, appdef.DefKind_WRecord, appdef.DefKind_ODoc, appdef.DefKind_ORecord},
}

func defaultDescriptorName(wsName string) Ident {
	return Ident(fmt.Sprintf("%sDescriptor", wsName))
}
