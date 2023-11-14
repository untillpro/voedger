/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
*
* @author Michael Saigachenko
*/

package collection

import (
	"github.com/voedger/voedger/pkg/appdef"
)

// ///////////////////////////////////
//
//	VIEW: sys.Collection
const (
	Field_PartKey          = "PartKey"
	Field_DocQName         = "DocQName"
	field_DocID            = "DocID"
	field_ElementID        = "ElementID"
	Field_Record           = "Record"
	PartitionKeyCollection = 1 // Always put the BO in the fixed partition
)

var (
	QNameViewCollection      = appdef.NewQName("sys", "CollectionView")
	QNameProjectorCollection = appdef.NewQName("sys", "ProjectorCollection")
)

// ///////////////////////////////////
//
//	FUNC: sys.Collection
const (
	field_Schema = "Schema"
	field_ID     = "ID"
)

var qNameQueryCollection = appdef.NewQName(appdef.SysPackage, "Collection")

// ///////////////////////////////////
//
//	FUNC: air.state
const (
	field_State = "State"
	field_After = "After"
)

var qNameQueryState = appdef.NewQName(appdef.SysPackage, "State")

// ///////////////////////////////////
//
//	FUNC: air.cdoc
const (
	field_xrefs = "xrefs"
)

var (
	qNameGetCDocFunc = appdef.NewQName(appdef.SysPackage, "GetCDoc")
)

const DEC = 10
