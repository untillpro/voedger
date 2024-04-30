/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package iauthnzimpl

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/istructs"
)

func (acl ACL) IsAllowed(principals []iauthnz.Principal, req iauthnz.AuthzRequest) bool {
	policy := ACPolicy_Deny
	var lastDenyingACElem ACElem
	for _, acElem := range acl {
		if matchOrNotSpecified_OpKinds(acElem.pattern.opKindsPattern, req.OperationKind) &&
			matchOrNotSpecified_QNames(acElem.pattern.qNamesPattern, req.Resource) &&
			matchOrNotSpecified_Fields(acElem.pattern.fieldsPattern, req.Fields) &&
			matchOrNotSpecified_Principals(acElem.pattern.principalsPattern, principals) {
			if policy = acElem.policy; policy == ACPolicy_Deny {
				lastDenyingACElem = acElem
			}
		}
	}
	if policy == ACPolicy_Deny && logger.IsVerbose() {
		logger.Verbose(fmt.Sprintf("%s for %s: %s -> deny", authNZToString(req), prnsToString(principals), lastDenyingACElem.desc))
	}
	return policy == ACPolicy_Allow
}

var defaultACL = ACL{
	{
		desc: "null auth policy",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdLinkDeviceToRestaurant,
				qNameQryIssuePrincipalToken,
				qNameCmdCreateLogin,
				qNameQryEcho,
				qNameQryGRCount,
				qNameCmdResetPasswordByEmail,
				qNameQryInitiateResetPasswordByEmail,
				qNameQryIssueVerifiedValueTokenForResetPassword,
				qNameCmdChangePassword,
				qNameQryModules,
				// https://dev.untill.com/projects/#!688808
				qNameQryGetDigitalReceipt,
				// https://dev.untill.com/projects/#!688808
				qNameQrySendReceiptByEmail,
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "everything is allowed to WorkspaceOwner",
		pattern: PatternType{
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceOwner}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "deny all on few QNames from all",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdStoreSubscriptionProfile, qNameCmdUpdateSubscription,

				qNameCDocSubscriptionProfile, qNameCDocUnTillOrders, qNameCDocUnTillPBill,
				qNameTestDeniedCmd, qNameTestDeniedCDoc, qNameCDocLogin, qNameCDocChildWorkspace, qNameTestDeniedQry,

				qNameCDocWorkspaceKindUser,
				qNameCDocWorkspaceKindDevice,
				qNameCDocWorkspaceKindRestaurant,
				qNameCDocWorkspaceKindAppWorkspace,
				qNameCmdSendEmailVerificationCode,

				qNameQryDescribePackage,
				qNameQryDescribePackageNames,
			},
		},
		policy: ACPolicy_Deny,
	},
	{
		desc: "update only is allowed for CDoc<$wsKind> for WorkspaceOwner",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCDocWorkspaceKindUser,
				qNameCDocWorkspaceKindDevice,
				qNameCDocWorkspaceKindRestaurant,
				qNameCDocWorkspaceKindAppWorkspace,
				qNameCDocReseller,
				qNameCDocUntillPayments,
			},
			opKindsPattern:    []iauthnz.OperationKindType{iauthnz.OperationKind_UPDATE},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceOwner}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// DENY ALL FROM LOGIN 'untillchargebeeagent'
		desc: "deny all from 'untillchargebeeagent' login",
		pattern: PatternType{
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_User, Name: untillChargebeeAgentLogin}}},
		},
		policy: ACPolicy_Deny,
	},
	{
		// GRANT EXEC ON c.air.UpdateSubscription TO ROLE ProfileUser AND LOGIN 'untillchargebeeagent'
		desc: "c.air.UpdateSubscription is allowed for 'untillchargebeeagent' login only and in its profile only",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{qNameCmdUpdateSubscription},
			principalsPattern: [][]iauthnz.Principal{
				{
					{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleProfileOwner},
					// AND
					{Kind: iauthnz.PrincipalKind_User, Name: untillChargebeeAgentLogin},
				},
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		// GRANT SELECT q.sys.DescribePackage* TO ROLE ProfileUser
		desc: "q.sys.DescribePackage* is allowed to be called in profile only",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameQryDescribePackage,
				qNameQryDescribePackageNames,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleProfileOwner}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "c.sys.InitiateJoinWorkspace is allowed for authenticated users",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdInitiateJoinWorkspace,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_User}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "c.sys.InitiateLeaveWorkspace is allowed for authenticated users",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdInitiateLeaveWorkspace,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_User}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// WorkspaceAdmin role asssigned automatically if has e.g. RoleResellersAdmin or RoleUntillPaymentsReseller
		desc: "allow few reseller-related commands to WorkspaceAdmin",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdInitiateInvitationByEmail,
				qNameQryCollection,
				qNameCmdInitiateUpdateInviteRoles,
				qNameCmdInitiateCancelAcceptedInvite,
				qNameCmdCancelSentInvite,
				qNameCDocChildWorkspace,
				qNameCmdInitChildWorkspace,
				qNameCmdEnrichPrincipalToken,
				istructs.QNameCommandCUD,

				// https://github.com/voedger/voedger/issues/208
				qNameCmdInitiateDeactivateWorkspace,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceAdmin}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// https://github.com/voedger/voedger/issues/125
		desc: "grant UPDATE on air.UntillPayments to role sys.WorkspaceAdmin",
		pattern: PatternType{
			qNamesPattern:     []appdef.QName{qNameCDocUntillPayments},
			opKindsPattern:    []iauthnz.OperationKindType{iauthnz.OperationKind_UPDATE},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceAdmin}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// ACL for portals https://dev.untill.com/projects/#!637208
		desc: "allow SELECT cdoc.air.ResellerSubscriptionsProfile to air.AirReseller",
		pattern: PatternType{
			opKindsPattern:    []iauthnz.OperationKindType{iauthnz.OperationKind_SELECT},
			qNamesPattern:     []appdef.QName{qNameCDocResellerSubscriptionsProfile},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleAirReseller}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// ACL for portals https://dev.untill.com/projects/#!637208
		desc: "allow exec few portals-related funcs to air.AirReseller",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdStoreResellerSubscriptionsProfile,
				qNameQryGetHostedAirSubscriptions,
				qNameQryCollection,

				// https://dev.untill.com/projects/#!638320
				qNameQryGetUPStatus,
				// https://dev.untill.com/projects/#!673032
				qNameQryListIssuedSubscriptionInvoices,
				qNameQryListIssuedSubscriptionResellers,
				qNameQryListPaidSubscriptionInvoices,
				qNameQryListPaidSubscriptionResellers,
				// https://dev.untill.com/projects/#!679811
				qNameQryIsDirectReseller,
				// https://dev.untill.com/projects/#!675263
				qNameQryPaidSubscriptionInvoicesReport,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleAirReseller}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// ACL for portals https://dev.untill.com/projects/#!637208
		desc: "allow SELECT cdoc.air.UPProfile to air.UntillPaymentsReseller and air.AirReseller",
		pattern: PatternType{
			opKindsPattern: []iauthnz.OperationKindType{iauthnz.OperationKind_SELECT},
			qNamesPattern:  []appdef.QName{qNameCDocUPProfile},
			principalsPattern: [][]iauthnz.Principal{
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}},
				// OR
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsReseller}},
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		// ACL for portals https://dev.untill.com/projects/#!637208
		desc: "allow few portal-related funcs to air.UntillPaymentsReseller and air.UntillPaymentsUser",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdCreateUPProfile,
				qNameQryGetUPOnboardingPage,
				qNameQryGetUPVerificationStatus,
				qNameQryGetUPAccountStatus,
				qNameQryGetUPEventHistory,
				qNameQryCollection,
			},
			principalsPattern: [][]iauthnz.Principal{
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}},
				// OR
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsReseller}},
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "q.air.QueryResellerInfo is allowed for authenticated users",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameQryQueryResellerInfo,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_User}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "grant exec on few funcs to role air.UntillPaymentsUser",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameQryGetUPStatus,
				qNameCmdCreateUntillPayment,

				// https://github.com/voedger/voedger/issues/57
				qNameCmdEnsureUPPredefinedPaymentModesExist,

				// https://dev.untill.com/projects/#!641315
				qNameQryGetUPTerminals,
				qNameQryActivateUPTerminal,
				qNameQryGetUPPaymentMethods,
				qNameQryToggleUPPaymentMethod,
				qNameQryRequestUPPaymentMethod,
				qNameQryGetUPTransactionsOverview,
				qNameQryGetUPTransactionReceipts,
				// https://dev.untill.com/projects/#!664899
				qNameQryGetUPLocationSubjects,
				// https://dev.untill.com/projects/#!659825
				qNameQryGetLocationDailyUPReport,
				// https://dev.untill.com/projects/#!653069
				qNameCmdVoidUntillPayment,
				// https://dev.untill.com/projects/#!683625
				qNameQryCreateTap2PaySession,
			},
			principalsPattern: [][]iauthnz.Principal{
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}},
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		// https://dev.untill.com/projects/#!640535
		desc: "grant exec on c.air.RegenerateUPProfileApiToken to role air.UntillPaymentsReseller and air.UntillPaymentsUser",
		pattern: PatternType{
			opKindsPattern: []iauthnz.OperationKindType{iauthnz.OperationKind_EXECUTE},
			qNamesPattern:  []appdef.QName{qNameCmdRegenerateUPProfileApiToken},
			principalsPattern: [][]iauthnz.Principal{
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsReseller}},
				// OR
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}},
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "grant exec on q.air.UPTerminalWebhook to role air.UntillPaymentsTerminal",
		pattern: PatternType{
			qNamesPattern:     []appdef.QName{qNameQryUPTerminalWebhook},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsTerminal}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		// https://github.com/voedger/voedger/issues/422
		// https://dev.untill.com/projects/#!649352
		// https://dev.untill.com/projects/#!650998
		// https://dev.untill.com/projects/#!653137
		// https://dev.untill.com/projects/#!665805
		// https://dev.untill.com/projects/#!663035
		desc: "grant exec on few funcs to role air.UntillPaymentsReseller and role air.UntillPaymentsUser",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameQryGetUPPayouts,
				qNameQryGetUPInvoiceParties,
				qNameQryGetUPTransferInstrument,
				qNameCmdRetryTransferUPPayout,
				// https://dev.untill.com/projects/#!685617
				qNameQryGetUPLocationRates,
				// https://dev.untill.com/projects/#!685179
				qNameQryUpdateShopperStatement,
			},
			principalsPattern: [][]iauthnz.Principal{
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsReseller}},
				// OR
				{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}},
			},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "grant exec on few funcs to role air.UntillPaymentsReseller",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameCmdUpdateUPLocationRates,
				qNameQryGetUPFeesOverview,
				// https://dev.untill.com/projects/#!664876
				qNameQryIsDirectReseller,
				// https://dev.untill.com/projects/#!659825
				qNameQryGetResellerDailyUPReport,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsReseller}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "grant exec on c.air.UpdateUPProfile to role air.UntillPaymentsUser",
		pattern: PatternType{
			qNamesPattern:     []appdef.QName{qNameCmdUpdateUPProfile},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "grant exec on few funcs to role air.UntillPaymentsManager",
		pattern: PatternType{
			qNamesPattern: []appdef.QName{
				qNameQryGetAllUPPayouts,
				qNameQryGetUPLocationInvoiceParties,
			},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsManager}}},
		},
		policy: ACPolicy_Allow,
	},
	{
		desc: "allow update cdoc.air.Reseller to sys.RoleWorkspaceAdmin",
		pattern: PatternType{
			opKindsPattern:    []iauthnz.OperationKindType{iauthnz.OperationKind_UPDATE},
			qNamesPattern:     []appdef.QName{qNameCDocReseller},
			principalsPattern: [][]iauthnz.Principal{{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceAdmin}}},
		},
		policy: ACPolicy_Allow,
	},
}
