/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package processors

const Field_RawObject_Body = "Body"

const (
	APIPath_null APIPath = iota
	APIPath_Docs
	APIPath_CDocs
	APIPath_Queries
	APIPath_Views
	APIPath_Commands
	APIPaths_Schema
	APIPath_Schemas_WorkspaceRoles
	APIPath_Schemas_WorkspaceRole
	APIPath_Auth_Login
	APIPath_Auth_Refresh
	APIPath_Users
	APIPath_N10N_SubscribeAndWatch
)
