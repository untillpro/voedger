/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package payloads

import (
	"fmt"
	"net/http"

	"github.com/voedger/voedger/pkg/istructs"
	itokens "github.com/voedger/voedger/pkg/itokens"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func GetSystemPrincipalToken(itokens itokens.ITokens, appQName istructs.AppQName) (string, error) {
	systemPrincipalToken, err := itokens.IssueToken(appQName, DefaultSystemPrincipalDuration, &systemPrincipalPayload)
	if err != nil {
		return "", fmt.Errorf("failed to issue system principal token: %w", err)
	}
	return systemPrincipalToken, nil
}

func GetSystemPrincipalTokenApp(appTokens istructs.IAppTokens) (string, error) {
	systemPrincipalToken, err := appTokens.IssueToken(DefaultSystemPrincipalDuration, &systemPrincipalPayload)
	if err != nil {
		return "", fmt.Errorf("failed to issue system principal token: %w", err)
	}
	return systemPrincipalToken, nil
}

func IsSystemPrincipal(principalPayload *PrincipalPayload) bool {
	return principalPayload != nil && principalPayload.ProfileWSID == istructs.NullWSID
}

func GetPayloadRegistry(itokens itokens.ITokens, token string, payload interface{}) (gp istructs.GenericPayload, err error) {
	if gp, err = itokens.ValidateToken(token, payload); err != nil {
		err = coreutils.NewHTTPError(http.StatusUnauthorized, err)
	}
	return
}

func GetPrincipalPayload(appTokens istructs.IAppTokens, principalToken string) (principalPayload PrincipalPayload, err error) {
	_, err = GetPayload(appTokens, principalToken, &principalPayload)
	return
}

// nolint (gp is never used)
func GetPayload(appTokens istructs.IAppTokens, token string, payload interface{}) (gp istructs.GenericPayload, err error) {
	if gp, err = appTokens.ValidateToken(token, payload); err != nil {
		err = coreutils.NewHTTPError(http.StatusUnauthorized, err)
	}
	return
}
