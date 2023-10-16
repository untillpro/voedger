/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package builtin

import (
	"embed"

	"github.com/voedger/voedger/pkg/appdef"
)

// Deprecated: use c.sys.CUD instead. Kept to not to break existing events only
var QNameCommandInit = appdef.NewQName(appdef.SysPackage, "Init")

//go:embed schema.sql
var schemaBuiltinFS embed.FS

const (
	field_ExistingQName = "ExistingQName"
	field_NewQName      = "NewQName"
	MaxCUDs             = 351 // max rawID in perftest template is 351
)
