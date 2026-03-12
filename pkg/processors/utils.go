/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package processors

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/authnz"
)

func CheckResponseIntent(st state.IHostState) error {
	kb, err := st.KeyBuilder(sys.Storage_Response, appdef.NullQName)
	if err != nil {
		// notest
		return err
	}
	respIntent := st.FindIntent(kb)
	if respIntent == nil {
		return nil
	}
	respIntentValue := respIntent.BuildValue()
	statusCode := respIntentValue.AsInt32(sys.Storage_Response_Field_StatusCode)
	if statusCode == http.StatusOK {
		return nil
	}
	return coreutils.NewHTTPErrorf(int(statusCode), respIntentValue.AsString(sys.Storage_Response_Field_ErrorMessage))
}

// returns ErrWSNotInited
func GetWSDesc(wsid istructs.WSID, appStructs istructs.IAppStructs) (wsDesc istructs.IRecord, err error) {
	wsDesc, err = appStructs.Records().GetSingleton(wsid, authnz.QNameCDocWorkspaceDescriptor)
	if err == nil && wsDesc.QName() == appdef.NullQName {
		err = ErrWSNotInited
	}
	return wsDesc, err
}

func GetRoles(principals []iauthnz.Principal) (roles []appdef.QName) {
	for _, prn := range principals {
		if prn.Kind != iauthnz.PrincipalKind_Role {
			continue
		}
		roles = append(roles, prn.QName)
	}
	return roles
}

func cudOpToStringForLog(cud istructs.ICUDRow) string {
	if cud.IsNew() {
		return "create"
	}
	if cud.IsDeactivated() {
		return "deactivate"
	}
	if cud.IsActivated() {
		return "activate"
	}
	return "update"
}

func SetPrincipalsForAnonymousOnlyFunc(appDef appdef.IAppDef, funcQName appdef.QName, wsid istructs.WSID, setter interface{ SetPrincipals([]iauthnz.Principal) }) (ok bool) {
	queryType := appDef.Type(funcQName)
	rulesForQuery := []appdef.IACLRule{}
	for _, acl := range appDef.ACL() {
		if acl.Filter().Match(queryType) {
			rulesForQuery = append(rulesForQuery, acl)
		}
	}
	if len(rulesForQuery) == 1 {
		if len(rulesForQuery[0].Ops()) == 1 &&
			rulesForQuery[0].Ops()[0] == appdef.OperationKind_Execute &&
			rulesForQuery[0].Policy() == appdef.PolicyKind_Allow &&
			rulesForQuery[0].Principal().Kind() == appdef.TypeKind_Role &&
			rulesForQuery[0].Principal().QName() == iauthnz.QNameRoleAnonymous {
			setter.SetPrincipals([]iauthnz.Principal{{
				Kind:  iauthnz.PrincipalKind_Role,
				WSID:  wsid,
				QName: iauthnz.QNameRoleAnonymous,
			}})
			return true
		}
	}
	return false
}

// returns logCtx enriched by `woffset`, `poffset`, `evqname` log attribs
// returns initial logCtx if verbose level is off
func LogEventAndCUDs(logCtx context.Context, event istructs.IPLogEvent, pLogOffset istructs.Offset, appDef appdef.IAppDef,
	skipStackFrames int, perCUDLogCallback func(istructs.ICUDRow) (bool, string, error), preCUDMessage string) (enrichedCtx context.Context, err error) {
	if !logger.IsVerbose() {
		return logCtx, nil
	}
	enrichedCtx = logger.WithContextAttrs(logCtx, map[string]any{
		"woffset": event.WLogOffset(),
		"poffset": pLogOffset,
		"evqname": event.QName(),
	})
	argsJSON := []byte("{}")
	if event.ArgumentObject() != nil && event.ArgumentObject().QName() != appdef.NullQName {
		argsJSON, err = json.Marshal(coreutils.ObjectToMap(event.ArgumentObject(), appDef))
		if err != nil {
			// notest
			return nil, err
		}
	}
	msg := fmt.Sprintf("args=%s", argsJSON)
	if len(preCUDMessage) > 0 {
		msg += ", " + preCUDMessage
	}
	logger.LogCtx(enrichedCtx, skipStackFrames+1, logger.LogLevelVerbose, msg)
	for cud := range event.CUDs {
		shouldLog, extraMsg := true, ""
		if perCUDLogCallback != nil {
			shouldLog, extraMsg, err = perCUDLogCallback(cud)
			if err != nil {
				return nil, err
			}
		}
		if !shouldLog {
			continue
		}
		newFieldsJSON, err := json.Marshal(coreutils.FieldsToMap(cud, appDef))
		if err != nil {
			// notest
			return nil, err
		}
		cudCtx := logger.WithContextAttrs(enrichedCtx, map[string]any{
			"rectype": cud.QName(),
			"recid":   cud.ID(),
			"op":      cudOpToStringForLog(cud),
		})
		msg := fmt.Sprintf("newfields=%s", newFieldsJSON)
		if len(extraMsg) > 0 {
			msg += ", " + extraMsg
		}
		logger.LogCtx(cudCtx, skipStackFrames+4, logger.LogLevelVerbose, msg) // +4 because call stack goes from cudType.enumRecs() here
	}
	return enrichedCtx, nil
}
