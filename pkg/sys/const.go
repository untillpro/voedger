/*
 * Copyright (c) 2024-present unTill Pro, Ltd.
 * @author Alisher Nurmanov
 */

package sys

import (
	"embed"

	"github.com/voedger/voedger/pkg/appdef"
)

const PackageName = "sys"
const PackagePath = "github.com/voedger/voedger/pkg/sys"
const VApp_SysVoedger = "sys/voedger"

//go:embed *.vsql
var SysFS embed.FS

var (
	// Storage names
	Storage_Record            = appdef.NewQName(PackageName, "Record")
	Storage_View              = appdef.NewQName(PackageName, "View")
	Storage_WLog              = appdef.NewQName(PackageName, "WLog")
	Storage_HTTP              = appdef.NewQName(PackageName, "Http")
	Storage_SendMail          = appdef.NewQName(PackageName, "SendMail")
	Storage_AppSecret         = appdef.NewQName(PackageName, "AppSecret")
	Storage_RequestSubject    = appdef.NewQName(PackageName, "RequestSubject")
	Storage_Result            = appdef.NewQName(PackageName, "Result")
	Storage_Event             = appdef.NewQName(PackageName, "Event")
	Storage_CommandContext    = appdef.NewQName(PackageName, "CommandContext")
	Storage_QueryContext      = appdef.NewQName(PackageName, "QueryContext")
	Storage_JobContext        = appdef.NewQName(PackageName, "JobContext")
	Storage_Response          = appdef.NewQName(PackageName, "Response")
	Storage_FederationCommand = appdef.NewQName(PackageName, "FederationCommand")
	Storage_FederationBlob    = appdef.NewQName(PackageName, "FederationBlob")
	Storage_Uniq              = appdef.NewQName(PackageName, "Uniq")
	Storage_Logger            = appdef.NewQName(PackageName, "Logger")
)

const (
	// Storage Field names

	Storage_Record_Field_ID          = "ID"
	Storage_Record_Field_WSID        = "WSID"
	Storage_Record_Field_Singleton   = "Singleton" // Deprecated: use Storage_Record_Field_IsSingleton instead
	Storage_Record_Field_IsSingleton = "IsSingleton"

	Storage_View_Field_WSID = "WSID"

	Storage_HTTP_Field_URL                           = "Url"
	Storage_HTTP_Field_Method                        = "Method"
	Storage_HTTP_Field_Header                        = "Header"
	Storage_HTTP_Field_Body                          = "Body"
	Storage_HTTP_Field_HTTPClientTimeoutMilliseconds = "HTTPClientTimeoutMilliseconds"
	Storage_HTTP_Field_StatusCode                    = "StatusCode"
	Storage_HTTP_Field_HandleErrors                  = "HandleErrors"
	Storage_HTTP_Field_Error                         = "Error"

	Storage_WLog_Field_Offset         = "Offset"
	Storage_WLog_Field_ArgumentObject = "ArgumentObject"
	Storage_WLog_Field_Count          = "Count"
	Storage_WLog_Field_WSID           = "WSID"
	Storage_WLog_Field_RegisteredAt   = "RegisteredAt"
	Storage_WLog_Field_DeviceID       = "DeviceID"
	Storage_WLog_Field_SyncedAt       = "SyncedAt"
	Storage_WLog_Field_CUDs           = "CUDs"
	Storage_WLog_Field_CUDs_IsNew     = CUDs_Field_IsNew

	Storage_Event_Field_Offset          = "Offset"
	Storage_Event_Field_Error           = "Error"
	Storage_Event_Field_ErrStr          = "ErrStr"
	Storage_Event_Field_QNameFromParams = "QNameFromParams"
	Storage_Event_Field_ValidEvent      = "ValidEvent"
	Storage_Event_Field_QName           = "QName"
	Storage_Event_Field_ArgumentObject  = "ArgumentObject"
	Storage_Event_Field_Synced          = "Synced"
	Storage_Event_Field_RegisteredAt    = "RegisteredAt"
	Storage_Event_Field_DeviceID        = "DeviceID"
	Storage_Event_Field_SyncedAt        = "SyncedAt"
	Storage_Event_Field_WLogOffset      = "WLogOffset"
	Storage_Event_Field_Workspace       = "Workspace"
	Storage_Event_Field_CUDs            = "CUDs"
	Storage_Event_Field_CUDs_IsNew      = CUDs_Field_IsNew

	Storage_CommandContext_Field_ArgumentObject         = "ArgumentObject"
	Storage_CommandContext_Field_ArgumentUnloggedObject = "ArgumentUnloggedObject"
	Storage_CommandContext_Field_WLogOffset             = "WLogOffset"
	Storage_CommandContext_Field_Workspace              = "Workspace"
	Storage_CommandContext_Field_Origin                 = "Origin"

	Storage_QueryContext_Field_ArgumentObject = "ArgumentObject"
	Storage_QueryContext_Field_Workspace      = "Workspace"

	Storage_JobContext_Field_Workspace = "Workspace"
	Storage_JobContext_Field_UnixTime  = "UnixTime"

	Storage_Uniq_Field_ID = "ID"

	Storage_SendMail_Field_From         = "From"
	Storage_SendMail_Field_To           = "To"
	Storage_SendMail_Field_CC           = "CC"
	Storage_SendMail_Field_BCC          = "BCC"
	Storage_SendMail_Field_Subject      = "Subject"
	Storage_SendMail_Field_Body         = "Body"
	Storage_SendMail_Field_Username     = "Username"
	Storage_SendMail_Field_Password     = "Password"
	Storage_SendMail_Field_Host         = "Host"
	Storage_SendMail_Field_Port         = "Port"
	Storage_SendMail_Field_Success      = "Success"
	Storage_SendMail_Field_ErrorMessage = "ErrorMessage"

	Storage_FederationCommand_Field_Command       = "Command"
	Storage_FederationCommand_Field_Body          = "Body"
	Storage_FederationCommand_Field_WSID          = "WSID"
	Storage_FederationCommand_Field_StatusCode    = "StatusCode"
	Storage_FederationCommand_Field_Token         = "Token"
	Storage_FederationCommand_Field_Owner         = "Owner"
	Storage_FederationCommand_Field_AppName       = "AppName"
	Storage_FederationCommand_Field_ExpectedCodes = "ExpectedCodes"
	Storage_FederationCommand_Field_NewIDs        = "NewIDs"
	Storage_FederationCommand_Field_Result        = "Result"

	Storage_FederationBlob_Field_Body             = "Body"
	Storage_FederationBlob_Field_WSID             = "WSID"
	Storage_FederationBlob_Field_Token            = "Token"
	Storage_FederationBlob_Field_Owner            = "Owner"
	Storage_FederationBlob_Field_AppName          = "AppName"
	Storage_FederationBlob_Field_ExpectedCodes    = "ExpectedCodes"
	Storage_FederationBlob_Field_OwnerRecord      = "OwnerRecord"
	Storage_FederationBlob_Field_OwnerRecordField = "OwnerRecordField"
	Storage_FederationBlob_Field_OwnerID          = "OwnerID"

	Storage_AppSecretField_Secret = "Secret"

	Storage_Response_Field_StatusCode   = "StatusCode"
	Storage_Response_Field_ErrorMessage = "ErrorMessage"

	Storage_RequestSubject_Field_Kind        = "Kind"
	Storage_RequestSubject_Field_ProfileWSID = "ProfileWSID"
	Storage_RequestSubject_Field_Name        = "Name"
	Storage_RequestSubject_Field_Token       = "Token"

	Storage_Logger_Field_LogLevel = "LogLevel"
	Storage_Logger_Field_Message  = "Message"

	// Common Field names
	CUDs_Field_IsNew = "IsNew"
)
