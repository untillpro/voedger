/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package sys

import (
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/apps"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/istructsmem"
	commandprocessor "github.com/voedger/voedger/pkg/processors/command"
	"github.com/voedger/voedger/pkg/projectors"
	"github.com/voedger/voedger/pkg/sys/authnz/signupin"
	"github.com/voedger/voedger/pkg/sys/authnz/workspace"
	"github.com/voedger/voedger/pkg/sys/authnz/wskinds"
	"github.com/voedger/voedger/pkg/sys/blobber"
	"github.com/voedger/voedger/pkg/sys/builtin"
	"github.com/voedger/voedger/pkg/sys/collection"
	"github.com/voedger/voedger/pkg/sys/describe"
	"github.com/voedger/voedger/pkg/sys/invite"
	"github.com/voedger/voedger/pkg/sys/journal"
	"github.com/voedger/voedger/pkg/sys/smtp"
	"github.com/voedger/voedger/pkg/sys/sqlquery"
	"github.com/voedger/voedger/pkg/sys/uniques"
	"github.com/voedger/voedger/pkg/sys/verifier"
	"github.com/voedger/voedger/pkg/vvm"
)

func Provide(appAPI apps.AppAPI, cfg *istructsmem.AppConfigType, appDefBuilder appdef.IAppDefBuilder, smtpCfg smtp.Cfg,
	ep extensionpoints.IExtensionPoint, wsPostInitFunc workspace.WSPostInitFunc) {
	blobber.ProvideBlobberCmds(cfg, appDefBuilder)
	collection.ProvideCollectionFunc(cfg, appDefBuilder)
	collection.ProvideCDocFunc(cfg, appDefBuilder)
	collection.ProvideStateFunc(cfg, appDefBuilder)
	journal.Provide(cfg, appDefBuilder, sep.EPJournalIndices(), sep.EPJournalPredicates())
	wskinds.ProvideCDocsWorkspaceKinds(appDefBuilder)
	builtin.ProvideCmdCUD(cfg)
	builtin.ProvideCmdInit(cfg)   // for import from air-importbo
	builtin.ProivdeCmdImport(cfg) // for sync
	builtin.ProvideQryEcho(cfg, appDefBuilder)
	builtin.ProvideQryGRCount(cfg, appDefBuilder)
	workspace.Provide(cfg, appDefBuilder, appAPI.IAppStructsProvider, appAPI.TimeFunc)
	sqlquery.Provide(cfg, appDefBuilder, appAPI.IAppStructsProvider, vvm.NumCommandProcessors)
	projectors.ProvideOffsetsDef(appDefBuilder)
	commandprocessor.ProvideJSONFuncParamsDef(appDefBuilder)
	verifier.Provide(cfg, appDefBuilder, itokens, federation, asp)
	signupin.ProvideQryRefreshPrincipalToken(cfg, appDefBuilder, itokens)
	signupin.ProvideCDocLogin(appDefBuilder)
	invite.Provide(cfg, appDefBuilder, timeFunc)
	cfg.AddAsyncProjectors(
		journal.ProvideWLogDatesAsyncProjectorFactory(),
		workspace.ProvideAsyncProjectorFactoryInvokeCreateWorkspace(federation, cfg.Name, itokens),
		workspace.ProvideAsyncProjectorFactoryInvokeCreateWorkspaceID(federation, cfg.Name, itokens),
		workspace.ProvideAsyncProjectorInitializeWorkspace(federation, timeFunc, cfg.Name, sep.EPWSTemplates(), itokens, wsPostInitFunc),
		verifier.ProvideAsyncProjectorFactory_SendEmailVerificationCode(federation, smtpCfg),
		invite.ProvideAsyncProjectorApplyInvitationFactory(timeFunc, federation, cfg.Name, itokens, smtpCfg),
		invite.ProvideAsyncProjectorApplyJoinWorkspaceFactory(timeFunc, federation, cfg.Name, itokens),
		invite.ProvideAsyncProjectorApplyUpdateInviteRolesFactory(timeFunc, federation, cfg.Name, itokens, smtpCfg),
		invite.ProvideAsyncProjectorApplyCancelAcceptedInviteFactory(timeFunc, federation, cfg.Name, itokens),
		invite.ProvideAsyncProjectorApplyLeaveWorkspaceFactory(timeFunc, federation, cfg.Name, itokens),
	)
	cfg.AddSyncProjectors(
		workspace.ProvideSyncProjectorChildWorkspaceIdxFactory(),
		invite.ProvideSyncProjectorInviteIndexFactory(),
		invite.ProvideSyncProjectorJoinedWorkspaceIndexFactory(),
		workspace.ProvideAsyncProjectorWorkspaceIDIdx(),
	)
	cfg.AddSyncProjectors(collection.ProvideSyncProjectorFactories(appDefBuilder)...)
	uniques.Provide(cfg, appDefBuilder)
	describe.Provide(cfg, asp, appDefBuilder)
	signupin.ProvideCmdEnrichPrincipalToken(cfg, appDefBuilder, atf)
	cfg.AddCUDValidators(builtin.ProvideRefIntegrityValidator())
}
