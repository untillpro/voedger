/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Michael Saigachenko
 */
package query2

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/sys/authnz"
)

// [~server.authnz/cmp.authLoginHandler~impl]
func authLoginHandler() apiPathHandler {
	return apiPathHandler{
		exec: func(ctx context.Context, qw *queryWork) (err error) {

			args := coreutils.MapObject(qw.msg.QueryParams().Argument)
			login, _, err := args.AsString(fieldLogin)
			if err != nil {
				return coreutils.NewHTTPError(http.StatusBadRequest, err)
			}
			password, _, err := args.AsString(fieldPassword)
			if err != nil {
				return coreutils.NewHTTPError(http.StatusBadRequest, err)
			}

			pseudoWSID := coreutils.GetPseudoWSID(istructs.NullWSID, login, istructs.CurrentClusterID())

			tokenArgs, err := json.Marshal(map[string]string{
				"Login":    login,
				"Password": password,
				"AppName":  qw.msg.AppQName().String(),
			})
			if err != nil {
				return err
			}
			reqURL := fmt.Sprintf(`api/v2/apps/%s/%s/workspaces/%d/queries/registry.IssuePrincipalToken?args=%s`,
				istructs.SysOwner, istructs.AppQName_sys_registry.Name(), pseudoWSID,
				url.QueryEscape(string(tokenArgs)))

			// WithRetry to avoid WSAECONNREFUSED errors on stress tests on Windows
			federationWithRetry := qw.federation.WithRetry()
			resp, err := federationWithRetry.Query(reqURL)
			if err != nil {
				return err
			}
			if resp.IsEmpty() {
				return errors.New("sys.IssuePrincipalToken response is empty")
			}
			token := resp.QPv2Response.Result()["PrincipalToken"].(string)
			wsError := resp.QPv2Response.Result()["WSError"].(string)
			wsid := resp.QPv2Response.Result()["WSID"].(float64)

			if len(wsError) > 0 {
				return errors.New("the login profile is created with an error: " + wsError)
			}

			if wsid == 0 {
				return coreutils.NewHTTPError(http.StatusConflict, fmt.Errorf("profile workspace is not yet ready, try again later"))
			}

			expiresInSeconds := authnz.DefaultPrincipalTokenExpiration.Seconds()
			json := fmt.Sprintf(`{
				"%s": "%s",
				"%s": %d,
				"%s": %d
			}`, fieldPrincipalToken, token, fieldExpiresInSeconds, int(expiresInSeconds), fieldProfileWSID, int(wsid))
			return qw.msg.Responder().Respond(bus.ResponseMeta{ContentType: httpu.ContentType_ApplicationJSON, StatusCode: http.StatusOK}, json)
		},
	}
}
