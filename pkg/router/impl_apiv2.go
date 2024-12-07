/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package router

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/voedger/voedger/pkg/appdef"
)

func (s *httpService) registerHandlersV2() {
	// create: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/docs/{pkg}.{table}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table),
		corsHandler(requestHandlerV2_table())).
		Methods(http.MethodPost)

	// update: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/docs/{pkg}.{table}/{id}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table,
		URLPlaceholder_id),
		corsHandler(requestHandlerV2_table())).
		Methods(http.MethodPatch)

	// deactivate: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/docs/{pkg}.{table}/{id}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table,
		URLPlaceholder_id),
		corsHandler(requestHandlerV2_table())).
		Methods(http.MethodDelete)

	// read single: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/docs/{pkg}.{table}/{id}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}/{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table,
		URLPlaceholder_id),
		corsHandler(requestHandlerV2_table())).
		Methods(http.MethodGet)

	// read collection: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/docs/{pkg}.{table}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/docs/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_table),
		corsHandler(requestHandlerV2_table())).
		Methods(http.MethodGet)

	// execute cmd: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/commands/{pkg}.{command}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/commands/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_command),
		corsHandler(requestHandlerV2_extension())).
		Methods(http.MethodPost)

	// execute query: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/queries/{pkg}.{query}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/query/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_query),
		corsHandler(requestHandlerV2_extension())).
		Methods(http.MethodGet)

	// view: /api/v2/users/{owner}/apps/{app}/workspaces/{wsid}/views/{pkg}.{view}
	s.router.HandleFunc(fmt.Sprintf("/api/v2/users/{%s}/apps/{%s}/workspaces/{%s:[0-9]+}/views/{%s}.{%s}",
		URLPlaceholder_appOwner, URLPlaceholder_appName, URLPlaceholder_wsid, URLPlaceholder_pkg, URLPlaceholder_view),
		corsHandler(requestHandlerV2_extension())).
		Methods(http.MethodGet)

}

func requestHandlerV2_extension() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		funcQNameStr := vars[URLPlaceholder_pkg] + "."
		switch req.Method {
		case http.MethodPost: // command
			funcQNameStr += vars[URLPlaceholder_command]
		case http.MethodGet: // query
			funcQNameStr += vars[URLPlaceholder_query]
		}
		funcQName, err := appdef.ParseQName(funcQNameStr)
		if err != nil {
			// protected already by url regexp
			// notest
			WriteTextResponse(resp, "failed to parse func QName: "+err.Error(), http.StatusBadRequest)
			return
		}
		_ = funcQName
		WriteTextResponse(resp, "not implemented", http.StatusNotImplemented)
	}
}

func requestHandlerV2_table() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		switch req.Method {
		case http.MethodGet:
			recordIDStr, isSingleRecord := vars[URLPlaceholder_id]
			_ = isSingleRecord
			_ = recordIDStr
		case http.MethodPost:
		case http.MethodPatch:
		case http.MethodDelete:
		}
		docQName, err := appdef.ParseQName(vars[URLPlaceholder_pkg+"."+URLPlaceholder_table])
		if err != nil {
			// protected already by url regexp
			// notest
			WriteTextResponse(resp, "failed to parse doc QName: "+err.Error(), http.StatusBadRequest)
			return
		}
		_ = docQName
		WriteTextResponse(resp, "not implemented", http.StatusNotImplemented)
	}
}

func requestHandlerV2() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		WriteTextResponse(resp, "not implemented", http.StatusNotImplemented)
	}
}
