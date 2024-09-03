/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 */

package invite

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/sys/smtp"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/utils/federation"
)

func Provide(sr istructsmem.IStatelessResources, time coreutils.ITime,
	federation federation.IFederation, itokens itokens.ITokens, smtpCfg smtp.Cfg) {
	provideCmdInitiateInvitationByEMail(sr, time)
	provideCmdInitiateJoinWorkspace(sr, time)
	provideCmdInitiateUpdateInviteRoles(sr, time)
	provideCmdInitiateCancelAcceptedInvite(sr, time)
	provideCmdInitiateLeaveWorkspace(sr, time)
	provideCmdCancelSentInvite(sr, time)
	provideCmdCreateJoinedWorkspace(sr)
	provideCmdUpdateJoinedWorkspaceRoles(sr)
	provideCmdDeactivateJoinedWorkspace(sr)
	sr.AddProjectors(appdef.SysPackagePath,
		asyncProjectorApplyInvitation(time, federation, itokens, smtpCfg),
		asyncProjectorApplyJoinWorkspace(time, federation, itokens),
		asyncProjectorApplyUpdateInviteRoles(time, federation, itokens, smtpCfg),
		asyncProjectorApplyCancelAcceptedInvite(time, federation, itokens),
		asyncProjectorApplyLeaveWorkspace(time, federation, itokens),
		syncProjectorInviteIndex(),
		syncProjectorJoinedWorkspaceIndex(),
		applyViewSubjectsIdx(),
	)
}
