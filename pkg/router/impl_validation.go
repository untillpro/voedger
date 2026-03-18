/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/istructs"
)

func withValidateForFuncs(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, handler func(req *http.Request, rw http.ResponseWriter, data validatedData)) http.HandlerFunc {
	return withValidate(numsAppsWorkspaces, handler, readBody, cookiesTokenToHeaders)
}

func withValidateForN10N(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, handler func(req *http.Request, rw http.ResponseWriter, data validatedData)) http.HandlerFunc {
	return withValidate(numsAppsWorkspaces, handler, readBody, cookiesTokenToHeaders)
}

func withValidateForBLOBs(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, handler func(req *http.Request, rw http.ResponseWriter, data validatedData)) http.HandlerFunc {
	return withValidate(numsAppsWorkspaces, handler, cookiesTokenToHeaders)
}

func withValidate(numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, handler func(req *http.Request, rw http.ResponseWriter, data validatedData), validators ...validatorFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		data, err := validate(req, numsAppsWorkspaces, validators...)
		if err != nil {
			logger.ErrorCtx(req.Context(), "routing.validation", err)
			ReplyCommonError(rw, err.Error(), http.StatusBadRequest)
			return
		}
		handler(req, rw, data)
	}
}

func validateRequest(req *http.Request, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces) (validatedData, error) {
	vars := mux.Vars(req)
	wsidStr := vars[URLPlaceholder_wsid]
	var wsid istructs.WSID
	var err error
	if len(wsidStr) > 0 {
		wsid, err = coreutils.ClarifyJSONWSID(json.Number(wsidStr))
		if err != nil {
			return validatedData{}, fmt.Errorf("invalid wsid: %w", err)
		}
	}

	appQNameStr := vars[URLPlaceholder_appOwner] + appdef.AppQNameQualifierChar + vars[URLPlaceholder_appName]
	appQName, err := appdef.ParseAppQName(appQNameStr)
	if err != nil {
		return validatedData{}, fmt.Errorf("invalid app QName: %w", err)
	}

	if numAppWorkspaces, ok := numsAppsWorkspaces[appQName]; ok {
		baseWSID := wsid.BaseWSID()
		if baseWSID <= istructs.MaxPseudoBaseWSID {
			wsid = coreutils.PseudoWSIDToAppWSID(wsid, numAppWorkspaces)
		}
	}

	res := validatedData{
		vars:     vars,
		wsid:     wsid,
		appQName: appQName,
		header:   map[string]string{},
	}

	for k, v := range req.Header {
		res.header[k] = v[0]
	}
	return res, nil
}

func validate(req *http.Request, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, validators ...validatorFunc) (validatedData, error) {
	validatedData, err := validateRequest(req, numsAppsWorkspaces)
	if err != nil {
		return validatedData, err
	}
	for _, validator := range validators {
		validatedData, err = validator(validatedData, req)
		if err != nil {
			return validatedData, err
		}
	}
	return validatedData, nil
}

func readBody(validatedData validatedData, req *http.Request) (validatedData, error) {
	if req.Body == nil || req.Body == http.NoBody {
		return validatedData, nil
	}
	var err error
	validatedData.body, err = io.ReadAll(req.Body)
	if err != nil {
		// notest
		return validatedData, fmt.Errorf("failed to read body: %w", err)
	}
	return validatedData, nil
}

// does not read body
func cookiesTokenToHeaders(validatedData validatedData, req *http.Request) (validatedData, error) {
	if _, ok := validatedData.header[httpu.Authorization]; !ok {
		// no token among headers -> look among cookies
		// no token among cookies as well -> just do nothing, 403 will happen on call helper commands further in BLOBs processor
		cookieBearerToken, ok, err := GetCookieBearerAuth(req)
		if err != nil {
			return validatedData, err
		}
		if ok {
			// authorization token in cookies -> q.sys.DownloadBLOBAuthnz requires it in headers
			validatedData.header[httpu.Authorization] = cookieBearerToken
		}
	}
	return validatedData, nil
}
