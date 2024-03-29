/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package registry

import (
	"fmt"
	"hash/crc32"
	"strconv"

	"github.com/untillpro/goutils/iterate"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/workspace"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

// sys/registry app, triggered by cdoc.registry.Login
// at pseudoWSID translated to AppWSID
func invokeCreateWorkspaceIDProjector(federation coreutils.IFederation, tokensAPI itokens.ITokens) func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
	return func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
		return iterate.ForEachError(event.CUDs, func(rec istructs.ICUDRow) error {
			if rec.QName() != QNameCDocLogin || !rec.IsNew() {
				return nil
			}
			loginHash := rec.AsString(authnz.Field_LoginHash)
			wsName := strconv.FormatUint(uint64(crc32.ChecksumIEEE([]byte(loginHash))), decimalBase)
			var wsKind appdef.QName
			switch istructs.SubjectKindType(rec.AsInt32(authnz.Field_SubjectKind)) {
			case istructs.SubjectKind_Device:
				wsKind = authnz.QNameCDoc_WorkspaceKind_DeviceProfile
			case istructs.SubjectKind_User:
				wsKind = authnz.QNameCDoc_WorkspaceKind_UserProfile
			default:
				return fmt.Errorf("unsupported cdoc.registry.Login.subjectKind: %d", rec.AsInt32(authnz.Field_SubjectKind))
			}
			targetClusterID := istructs.ClusterID(rec.AsInt32(authnz.Field_ProfileCluster))
			targetApp := rec.AsString(authnz.Field_AppName)
			ownerWSID := event.Workspace()
			wsidToCallCreateWSIDAt := istructs.NewWSID(targetClusterID, ownerWSID.BaseWSID())
			templateName := ""
			templateParams := ""
			return workspace.ApplyInvokeCreateWorkspaceID(federation, s.App(), tokensAPI, wsName, wsKind, wsidToCallCreateWSIDAt,
				targetApp, templateName, templateParams, rec, ownerWSID)
		})
	}
}
