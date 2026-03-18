/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/strconvu"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/iblobstoragestg"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/processors"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
)

func (s *routerService) registerHandlersV2() {
	// create: /api/v2/apps/{owner}/{app}/workspaces/{wsid}/docs/{pkg}.{table}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table),
		corsHandler(requestHandlerV2_table(s.requestSender, processors.APIPath_Docs, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPost).Name("create")

	// update, deactivate, read single doc: /api/v2/apps/{owner}/{app}/workspaces/{wsid}/docs/{pkg}.{table}/{id}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}/{%s:[0-9]+}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table,
		URLPlaceholder_id),
		corsHandler(requestHandlerV2_table(s.requestSender, processors.APIPath_Docs, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPatch, http.MethodDelete, http.MethodGet).Name("update or read single")

	// read collection: /api/v2/apps/{owner}/{app}/workspaces/{wsid}/cdocs/{pkg}.{table}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s:[0-9]+}/cdocs/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table),
		corsHandler(requestHandlerV2_table(s.requestSender, processors.APIPath_CDocs, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("read collection")

	// execute cmd: /api/v2/apps/{owner}/{app}/workspaces/{wsid}/commands/{pkg}.{command}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s:[0-9]+}/commands/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_command),
		corsHandler(requestHandlerV2_extension(s.requestSender, processors.APIPath_Commands, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPost).Name("exec cmd")

	// execute query: /api/v2/apps/{owner}/{app}/workspaces/{wsid}/queries/{pkg}.{query}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s:[0-9]+}/queries/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_query),
		corsHandler(requestHandlerV2_extension(s.requestSender, processors.APIPath_Queries, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("exec query")

	// view: /api/v2/apps/{owner}/{app}/workspaces/{wsid}/views/{pkg}.{view}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s:[0-9]+}/views/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_view),
		corsHandler(requestHandlerV2_view(s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("view")

	// schemas: get workspace schema: /api/v2/apps/{owner}/{app}/schemas
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/schemas",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_schemas(s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("schemas")

	// schemas, workspace roles: get workspace schema: /api/v2/apps/{owner}/{app}/schemas/{pkg}.{workspace}/roles
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/schemas/{%s}.{%s}/roles",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_pkg, URLPlaceholder_workspaceName),
		corsHandler(requestHandlerV2_schemas_wsRoles(s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("schemas, workspace roles")

	// schemas, workspace role: get workspace schema: /api/v2/apps/{owner}/{app}/schemas/{pkg}.{workspace}/roles/{pkg}.{role}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/schemas/{%s}.{%s}/roles/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_pkg, URLPlaceholder_workspaceName, URLPlaceholder_rolePkg, URLPlaceholder_role),
		corsHandler(requestHandlerV2_schemas_wsRole(s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("schemas, workspace role")

	// auth/login: /api/v2/apps/{owner}/{app}/auth/login
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/auth/login",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_auth_login(s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPost).Name("auth login")

	// auth/login: /api/v2/apps/{owner}/{app}/auth/login
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/auth/refresh",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_auth_refresh(s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPost).Name("auth refresh")

	// create user /api/v2/apps/{owner}/{app}/users
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/users",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_create_user(s.numsAppsWorkspaces, s.iTokens, s.federation))).
		Methods(http.MethodOptions, http.MethodPost).Name("create user")

	// change password user /api/v2/apps/{owner}/{app}/users/change-password
	// [~server.users/cmp.routerUsersChangePasswordPathHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/users/change-password",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_changePassword(s.numsAppsWorkspaces, s.federation))).
		Methods(http.MethodOptions, http.MethodPost).Name("change password")

	// create device /api/v2/apps/{owner}/{app}/devices
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/devices",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_create_device(s.numsAppsWorkspaces, s.federation))).
		Methods(http.MethodOptions, http.MethodPost).Name("create device")

	// blob create /api/v2/apps/{owner}/{app}/workspaces/{wsid}/docs/{doc}/blobs/{field}
	// [~server.apiv2.blobs/cmp.routerBlobsCreatePathHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s}/docs/{%s}.{%s}/blobs/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table, URLPlaceholder_field),
		corsHandler(requestHandlerV2_blobs_create(s.blobRequestHandler, s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPost).Name("blobs create")

	// blob read GET /api/v2/apps/{owner}/{app}/workspaces/{wsid}/docs/{pkg}.{table}/{id}/blobs/{fieldName}
	// [~server.apiv2.blobs/cmp.routerBlobsReadPathHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s}/docs/{%s}.{%s}/{%s}/blobs/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg,
		URLPlaceholder_table, URLPlaceholder_id, URLPlaceholder_field),
		corsHandler(requestHandlerV2_blobs_read(s.blobRequestHandler, s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("blobs read")

	// temp blob create /api/v2/apps/{owner}/{app}/workspaces/{wsid}/tblobs
	// [~server.apiv2.tblobs/cmp.routerTBlobsCreatePathHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s}/tblobs",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid),
		corsHandler(requestHandlerV2_tempblobs_create(s.blobRequestHandler, s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodPost).Name("temp blobs create")

	// temp blob read GET /api/v2/apps/{owner}/{app}/workspaces/{wsid}/tblobs/{suuid}
	// [~server.apiv2.blobs/cmp.routerTBlobsReadPathHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/workspaces/{%s}/tblobs/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_blobIDOrSUUID),
		corsHandler(requestHandlerV2_tempblobs_read(s.blobRequestHandler, s.requestSender, s.numsAppsWorkspaces))).
		Methods(http.MethodOptions, http.MethodGet).Name("temp blobs read")

	// notifications subscribe+watch /api/v2/apps/{owner}/{app}/notifications
	// [~server.n10n/cmp.routerCreateChannelHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/notifications",
		URLPlaceholder_appOwner, URLPlaceholder_appName),
		corsHandler(requestHandlerV2_notifications_subscribeAndWatch(s.numsAppsWorkspaces, s.requestSender))).
		Methods(http.MethodOptions, http.MethodPost).Name("notifications subscribe + watch")

	// notifications unsubscribe /api/v2/apps/{owner}/{app}/notifications/{channelId}/workspaces/{wsid}/subscriptions/{entity}
	// [~server.n10n/cmp.routerUnsubscribeHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/notifications/{%s}/workspaces/{%s}/subscriptions/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_channelID, URLPlaceholder_wsid, URLPlaceholder_view),
		corsHandler(requestHandlerV2_notifications(s.numsAppsWorkspaces, s.requestSender))).
		Methods(http.MethodOptions, http.MethodDelete).Name("notifications unsubscribe")

	// notifications subscribe to an extra view /api/v2/apps/{owner}/{app}/notifications/{channelId}/workspaces/{wsid}/subscriptions/{entity}
	// [~server.n10n/cmp.routerAddSubscriptionHandler~impl]
	s.router.HandleFunc(fmt.Sprintf("/api/v2/apps/{%s}/{%s}/notifications/{%s}/workspaces/{%s}/subscriptions/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_channelID, URLPlaceholder_wsid, URLPlaceholder_view),
		corsHandler(requestHandlerV2_notifications(s.numsAppsWorkspaces, s.requestSender))).
		Methods(http.MethodOptions, http.MethodPut).Name("notifications subscribe to an extra view")
}

func requestHandlerV2_schemas(reqSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(processors.APIPaths_Schema)
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

func requestHandlerV2_schemas_wsRoles(reqSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(processors.APIPath_Schemas_WorkspaceRoles)
		busRequest.WorkspaceQName = appdef.NewQName(data.vars[URLPlaceholder_pkg], data.vars[URLPlaceholder_workspaceName])
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

// [~server.users/cmp.routerUsersChangePasswordPathHandler~impl]
func requestHandlerV2_changePassword(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, federation federation.IFederation) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		login, oldPassword, newPassword, err := parseChangePasswordArgs(string(busRequest.Body))
		if err != nil {
			ReplyCommonError(rw, err.Error(), http.StatusBadRequest)
			return
		}
		pseudoWSID := coreutils.GetPseudoWSID(istructs.NullWSID, login, istructs.CurrentClusterID())
		bodyBytes, err := json.Marshal(map[string]any{
			"args": map[string]any{
				"Login":   login,
				"AppName": busRequest.AppQName.String(),
			},
			"unloggedArgs": map[string]any{
				"OldPassword": oldPassword,
				"NewPassword": newPassword,
			},
		})
		if err != nil {
			ReplyCommonError(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		body := string(bodyBytes)
		url := fmt.Sprintf("api/v2/apps/sys/registry/workspaces/%d/commands/registry.ChangePassword", pseudoWSID)
		if _, err = federation.Func(url, body, httpu.WithMethod(http.MethodPost), httpu.WithDiscardResponse()); err != nil { // null auth
			replyErr(rw, err)
			return
		}
		ReplyJSON(rw, "", http.StatusOK)
	})
}

// [~server.users/cmp.router.UsersCreatePathHandler~impl]
func requestHandlerV2_create_user(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces,
	iTokens itokens.ITokens, federation federation.IFederation) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		verifiedEmailToken, displayName, pwd, err := parseCreateLoginArgs(string(busRequest.Body))
		if err != nil {
			ReplyCommonError(rw, err.Error(), http.StatusBadRequest)
			return
		}
		payload := payloads.VerifiedValuePayload{}
		_, err = iTokens.ValidateToken(verifiedEmailToken, &payload)
		if err != nil {
			ReplyCommonError(rw, fmt.Sprintf("verifiedEmailToken validation failed: %s", err.Error()), http.StatusBadRequest)
			return
		}
		email := payload.Value.(string)
		pseudoWSID := coreutils.GetPseudoWSID(istructs.NullWSID, email, istructs.CurrentClusterID())
		url := fmt.Sprintf("api/v2/apps/sys/registry/workspaces/%d/commands/registry.CreateEmailLogin", pseudoWSID)
		wsKindInitData := fmt.Sprintf(`{"DisplayName":%q}`, displayName)
		bodyBytes, err := json.Marshal(map[string]any{
			"args": map[string]any{
				"Email":                    verifiedEmailToken,
				"AppName":                  busRequest.AppQName.String(),
				"SubjectKind":              istructs.SubjectKind_User,
				"WSKindInitializationData": wsKindInitData,
				"ProfileCluster":           istructs.CurrentClusterID(),
			},
			"unloggedArgs": map[string]any{
				"Password": pwd,
			},
		})
		if err != nil {
			ReplyCommonError(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		body := string(bodyBytes)
		sysToken, err := payloads.GetSystemPrincipalToken(iTokens, istructs.AppQName_sys_registry)
		if err != nil {
			// notest
			ReplyCommonError(rw, "failed to issue sys token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		resp, err := federation.Func(url, body,
			httpu.WithAuthorizeBy(sysToken),
			httpu.WithMethod(http.MethodPost),
		)
		if err != nil {
			replyErr(rw, err)
			return
		}
		ReplyJSON(rw, resp.Body, http.StatusCreated)
	})
}

func requestHandlerV2_notifications_subscribeAndWatch(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, reqSender bus.IRequestSender) http.HandlerFunc {
	return withValidateForN10N(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		_, ok := rw.(http.Flusher)
		if !ok {
			// notest
			WriteTextResponse(rw, "streaming unsupported!", http.StatusInternalServerError)
			return
		}
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.IsN10N = true
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

// handles both unsubscribe and subscribe to an extra view
func requestHandlerV2_notifications(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, reqSender bus.IRequestSender) http.HandlerFunc {
	return withValidateForN10N(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		var err error
		busRequest := createBusRequest(data, req)
		vars := mux.Vars(req)
		busRequest.Resource = vars[URLPlaceholder_channelID]
		busRequest.IsAPIV2 = true
		busRequest.IsN10N = true
		busRequest.QName, err = appdef.ParseQName(vars[URLPlaceholder_view])
		if err != nil {
			ReplyCommonError(rw, err.Error(), http.StatusBadRequest)
			return
		}

		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

// [~server.devices/cmp.routerDevicesCreatePathHandler~impl]
func requestHandlerV2_create_device(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, federation federation.IFederation) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		if len(busRequest.Body) > 0 {
			ReplyCommonError(rw, "unexpected body", http.StatusBadRequest)
			return
		}
		login, pwd := coreutils.DeviceRandomLoginPwd()
		pseudoWSID := coreutils.GetPseudoWSID(istructs.NullWSID, login, istructs.CurrentClusterID())
		url := fmt.Sprintf("api/v2/apps/sys/registry/workspaces/%d/commands/registry.CreateLogin", pseudoWSID)
		body := fmt.Sprintf(`{"args":{"Login":"%s","AppName":"%s","SubjectKind":%d,"WSKindInitializationData":"{}","ProfileCluster":%d},"unloggedArgs":{"Password":"%s"}}`,
			login, busRequest.AppQName, istructs.SubjectKind_Device, istructs.CurrentClusterID(), pwd)
		_, err := federation.Func(url, body, httpu.WithMethod(http.MethodPost))
		if err != nil {
			replyErr(rw, err)
			return
		}
		result := fmt.Sprintf(`{"%s":"%s","%s":"%s"}`, fieldLogin, login, fieldPassword, pwd)
		ReplyJSON(rw, result, http.StatusCreated)
	})
}

// [~server.authnz/cmp.routerRefreshHandler~impl]
func requestHandlerV2_auth_refresh(reqSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(processors.APIPath_Auth_Refresh)
		busRequest.Method = http.MethodGet
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

// [~server.authnz/cmp.routerLoginPathHandler~impl]
func requestHandlerV2_auth_login(reqSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(processors.APIPath_Auth_Login)
		busRequest.Method = http.MethodGet
		queryParams := map[string]string{}
		queryParams["args"] = string(busRequest.Body)
		busRequest.Query = queryParams
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

func requestHandlerV2_blobs_read(blobRequestHandler blobprocessor.IRequestHandler, requestSender bus.IRequestSender,
	numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForBLOBs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		vars := mux.Vars(req)
		ownerRecord := appdef.NewQName(vars[URLPlaceholder_pkg], vars[URLPlaceholder_table])
		ownerRecordField := vars[URLPlaceholder_field]
		ownerID, err := strconvu.ParseUint64(vars[URLPlaceholder_id])
		if err != nil {
			// notest: checked by router url rule
			panic(err)
		}
		if !blobRequestHandler.HandleRead_V2(data.appQName, data.wsid, data.header, req.Context(),
			newBLOBOKResponseIniter(rw, http.StatusOK), func(sysErr coreutils.SysError) {
				replyErr(rw, sysErr)
			}, ownerRecord, ownerRecordField, istructs.RecordID(ownerID), requestSender, iblobstoragestg.RLimiter_Null) {
			replyServiceUnavailable(rw)
		}
	})
}

func requestHandlerV2_tempblobs_read(blobRequestHandler blobprocessor.IRequestHandler, requestSender bus.IRequestSender,
	numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForBLOBs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		vars := mux.Vars(req)
		suuid := iblobstorage.SUUID(vars[URLPlaceholder_blobIDOrSUUID])
		if !blobRequestHandler.HandleReadTemp_V2(data.appQName, data.wsid, data.header, req.Context(),
			newBLOBOKResponseIniter(rw, http.StatusOK), func(sysErr coreutils.SysError) {
				replyErr(rw, sysErr)
			}, requestSender, suuid, iblobstoragestg.RLimiter_Null) {
			replyServiceUnavailable(rw)
		}
	})
}

func requestHandlerV2_tempblobs_create(blobRequestHandler blobprocessor.IRequestHandler, requestSender bus.IRequestSender,
	numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForBLOBs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		if !blobRequestHandler.HandleWriteTemp_V2(data.appQName, data.wsid, data.header, req.Context(),
			newBLOBOKResponseIniter(rw, http.StatusCreated), req.Body, func(sysErr coreutils.SysError) {
				replyErr(rw, sysErr)
			}, requestSender) {
			replyServiceUnavailable(rw)
		}
	})
}

func requestHandlerV2_blobs_create(blobRequestHandler blobprocessor.IRequestHandler, requestSender bus.IRequestSender,
	numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForBLOBs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		vars := mux.Vars(req)
		ownerRecord := appdef.NewQName(vars[URLPlaceholder_pkg], vars[URLPlaceholder_table])
		ownerRecordField := vars[URLPlaceholder_field]
		if !blobRequestHandler.HandleWrite_V2(data.appQName, data.wsid, data.header, req.Context(),
			newBLOBOKResponseIniter(rw, http.StatusCreated), req.Body, func(sysErr coreutils.SysError) {
				replyErr(rw, sysErr)
			}, requestSender, ownerRecord, ownerRecordField) {
			replyServiceUnavailable(rw)
		}
	})
}

func requestHandlerV2_schemas_wsRole(reqSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(processors.APIPath_Schemas_WorkspaceRole)
		busRequest.WorkspaceQName = appdef.NewQName(data.vars[URLPlaceholder_pkg], data.vars[URLPlaceholder_workspaceName])
		busRequest.QName = appdef.NewQName(data.vars[URLPlaceholder_rolePkg], data.vars[URLPlaceholder_role])
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

func requestHandlerV2_view(reqSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(processors.APIPath_Views)
		busRequest.QName = appdef.NewQName(data.vars[URLPlaceholder_pkg], data.vars[URLPlaceholder_view])
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

func requestHandlerV2_extension(reqSender bus.IRequestSender, apiPath processors.APIPath, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		entity := ""
		switch apiPath {
		case processors.APIPath_Commands:
			entity = data.vars[URLPlaceholder_command]
		case processors.APIPath_Queries:
			entity = data.vars[URLPlaceholder_query]
		}
		busRequest := createBusRequest(data, req)
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(apiPath)
		busRequest.QName = appdef.NewQName(data.vars[URLPlaceholder_pkg], entity)
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

func requestHandlerV2_table(reqSender bus.IRequestSender, apiPath processors.APIPath, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) http.HandlerFunc {
	return withValidateForFuncs(numsAppsWorkspaces, func(req *http.Request, rw http.ResponseWriter, data validatedData) {
		busRequest := createBusRequest(data, req)
		if recordIDStr, hasDocID := data.vars[URLPlaceholder_id]; hasDocID {
			docID, err := strconvu.ParseUint64(recordIDStr)
			if err != nil {
				// notest
				panic(err)
			}
			busRequest.DocID = istructs.IDType(docID)

		}
		busRequest.IsAPIV2 = true
		busRequest.APIPath = int(apiPath)
		busRequest.QName = appdef.NewQName(data.vars[URLPlaceholder_pkg], data.vars[URLPlaceholder_table])
		sendRequestAndReadResponse(req, busRequest, reqSender, rw, data)
	})
}

func sendRequestAndReadResponse(req *http.Request, busRequest bus.Request, reqSender bus.IRequestSender, rw http.ResponseWriter, data validatedData) {
	reqCtxWithExtensionAttrib := withLogAttribs(req.Context(), data, busRequest, req)

	// req's BaseContext is router service's context. See service.Start()
	// router app closing or client disconnected -> req.Context() is done
	// will create new cancellable context and cancel it if http section send is failed.
	// case: the client is gracefully disconnected
	// 	- reqCtxWithExtensionAttrib is done, no additional cancelation needed
	// case: the ethernet cable is broken
	//   - reqCtxWithExtensionAttrib is not done
	//   - the router is failed to send data to the client.
	//   - need to explicitly cancel the context to inform the handler that the client is gone.
	requestCtx, cancel := context.WithCancel(reqCtxWithExtensionAttrib)
	defer cancel() // to avoid context leak

	logServeRequest(requestCtx)

	sentAt := time.Now()
	respCh, respMeta, respErr, err := reqSender.SendRequest(requestCtx, busRequest)
	if err != nil {
		logger.ErrorCtx(requestCtx, "routing.send2vvm.error", "sending request to VVM on", busRequest.QName, "is failed:", err, ". Body:\n", string(busRequest.Body))
		ReplyCommonError(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	logLatency(requestCtx, sentAt)

	initResponse(rw, respMeta)
	reply_v2(requestCtx, rw, respCh, respErr, cancel, respMeta)
}

func parseChangePasswordArgs(body string) (login, oldPassword, newPassword string, err error) {
	args := coreutils.MapObject{}
	if err = json.Unmarshal([]byte(body), &args); err != nil {
		return "", "", "", fmt.Errorf("failed to unmarshal body: %w", err)
	}
	ok := false
	login, ok, err = args.AsString("login")
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", errors.New("login field missing")
	}
	oldPassword, ok, err = args.AsString("oldPassword")
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", errors.New("oldPassword field missing")
	}
	newPassword, ok, err = args.AsString("newPassword")
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", errors.New("newPassword field missing")
	}
	return login, oldPassword, newPassword, nil
}

func parseCreateLoginArgs(body string) (verifiedEmailToken, displayName, pwd string, err error) {
	args := coreutils.MapObject{}
	if err = json.Unmarshal([]byte(body), &args); err != nil {
		return "", "", "", fmt.Errorf("failed to unmarshal body: %w", err)
	}
	ok := false
	verifiedEmailToken, ok, err = args.AsString("verifiedEmailToken")
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", errors.New("verifiedEmailToken field missing")
	}
	displayName, ok, err = args.AsString("displayName")
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", errors.New("displayName field missing")
	}
	pwd, ok, err = args.AsString("password")
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", errors.New("password field missing")
	}
	return
}
