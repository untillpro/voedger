/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package iauthnzimpl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/goutils/logger"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
)

const (
	alienWSID     = 3
	nonInitedWSID = 4
)

func TestBasicUsage(t *testing.T) {
	require := require.New(t)
	logger.SetLogLevel(logger.LogLevelVerbose)
	defer logger.SetLogLevel(logger.LogLevelInfo)

	tokens := itokensjwt.ProvideITokens(itokensjwt.SecretKeyExample, coreutils.NewITime())
	appTokens := payloads.ProvideIAppTokensFactory(tokens).New(istructs.AppQName_test1_app1)
	pp := payloads.PrincipalPayload{
		Login:       "testlogin",
		SubjectKind: istructs.SubjectKind_User,
		Roles: []payloads.RoleType{
			{
				WSID:  42,
				QName: iauthnz.QNameRoleWorkspaceDevice,
			},
		},
		ProfileWSID: 1,
	}
	token, err := appTokens.IssueToken(time.Minute, &pp)
	require.NoError(err)

	appStructs := AppStructsWithTestStorage(istructs.AppQName_test1_app1, map[istructs.WSID]map[appdef.QName]map[istructs.RecordID]map[string]interface{}{
		// WSID 1 is the user profile, not necessary to store docs there

		// workspace owned by the user
		istructs.WSID(2): {
			qNameCDocWorkspaceDescriptor: {
				// cdoc.sys.WorkspaceDescriptor.ID=1, .OwnerWSID=1
				1: {
					"OwnerWSID": int64(1), // the same as ProfileWSID
				},
			},
		},

		// child workspace. Parent is WSID 2
		istructs.WSID(3): {
			qNameCDocWorkspaceDescriptor: {
				// cdoc.sys.WorkspaceDescriptor.ID=1, .OwnerWSID=2
				1: {
					"OwnerWSID": int64(2),
				},
			},
		},
	})
	authn := NewDefaultAuthenticator(TestSubjectRolesGetter, TestIsDeviceAllowedFuncs)
	t.Run("authenticate in the profile", func(t *testing.T) {
		req := iauthnz.AuthnRequest{
			Host:        "127.0.0.1",
			RequestWSID: 1,
			Token:       token,
		}
		principals, principalPayload, err := authn.Authenticate(context.Background(), appStructs, appTokens, req)
		require.NoError(err)
		require.Len(principals, 5)
		require.Equal(iauthnz.PrincipalKind_Role, principals[0].Kind)
		require.Equal(iauthnz.QNameRoleEveryone, principals[0].QName)

		require.Equal(iauthnz.PrincipalKind_Role, principals[1].Kind)
		require.Equal(iauthnz.QNameRoleAuthenticatedUser, principals[1].QName)

		require.Equal(iauthnz.PrincipalKind_User, principals[2].Kind)
		require.Equal("testlogin", principals[2].Name)

		// request to the profile -> ProfileOwner role got
		require.Equal(iauthnz.PrincipalKind_Role, principals[3].Kind)
		require.Equal(iauthnz.QNameRoleProfileOwner, principals[3].QName)

		require.Equal(iauthnz.PrincipalKind_Host, principals[4].Kind)
		require.Equal("127.0.0.1", principals[4].Name)

		require.Equal(pp, principalPayload)
	})

	t.Run("authenticate in the owned workspace", func(t *testing.T) {
		req := iauthnz.AuthnRequest{
			Host:        "127.0.0.1",
			RequestWSID: 2,
			Token:       token,
		}
		// request to WSID 2, there is a cdoc.sys.WorkspaceDescriptor.OwnerWSID = 1 -> the workspace is owned by the user with ProfileWSID=1
		principals, principalPayload, err := authn.Authenticate(context.Background(), appStructs, appTokens, req)
		require.NoError(err)
		require.Len(principals, 5)
		require.Equal(iauthnz.PrincipalKind_Role, principals[0].Kind)
		require.Equal(iauthnz.QNameRoleEveryone, principals[0].QName)

		require.Equal(iauthnz.PrincipalKind_Role, principals[1].Kind)
		require.Equal(iauthnz.QNameRoleAuthenticatedUser, principals[1].QName)

		require.Equal(iauthnz.PrincipalKind_User, principals[2].Kind)
		require.Equal("testlogin", principals[2].Name)

		// request to the owned workspace -> WorkspaceOwner role got
		require.Equal(iauthnz.PrincipalKind_Role, principals[3].Kind)
		require.Equal(iauthnz.QNameRoleWorkspaceOwner, principals[3].QName)

		require.Equal(iauthnz.PrincipalKind_Host, principals[4].Kind)
		require.Equal("127.0.0.1", principals[4].Name)

		require.Equal(pp, principalPayload)
	})

	t.Run("authenticate in the child workspace", func(t *testing.T) {
		pp := payloads.PrincipalPayload{
			Login:       "testlogin",
			SubjectKind: istructs.SubjectKind_User,
			Roles: []payloads.RoleType{
				{
					WSID:  2,
					QName: iauthnz.QNameRoleWorkspaceOwner,
				},
			},
			ProfileWSID: 1,
		}
		token, err := appTokens.IssueToken(time.Minute, &pp)
		require.NoError(err)
		req := iauthnz.AuthnRequest{
			Host:        "127.0.0.1",
			RequestWSID: 3,
			Token:       token,
		}
		// request to WSID 2, there is a cdoc.sys.WorkspaceDescriptor.OwnerWSID = 1 -> the workspace is owned by the user with ProfileWSID=1
		principals, principalPayload, err := authn.Authenticate(context.Background(), appStructs, appTokens, req)
		require.NoError(err)
		require.Len(principals, 5)
		require.Equal(iauthnz.PrincipalKind_Role, principals[0].Kind)
		require.Equal(iauthnz.QNameRoleEveryone, principals[0].QName)

		require.Equal(iauthnz.PrincipalKind_Role, principals[1].Kind)
		require.Equal(iauthnz.QNameRoleAuthenticatedUser, principals[1].QName)

		require.Equal(iauthnz.PrincipalKind_User, principals[2].Kind)
		require.Equal("testlogin", principals[2].Name)

		// request to a workspace with a token enriched by WorkspaceOwne role -> WorkspaceOwner role got
		require.Equal(iauthnz.PrincipalKind_Role, principals[3].Kind)
		require.Equal(iauthnz.QNameRoleWorkspaceOwner, principals[3].QName)

		require.Equal(iauthnz.PrincipalKind_Host, principals[4].Kind)
		require.Equal("127.0.0.1", principals[4].Name)
		require.Equal(pp, principalPayload)
	})
}

func TestAuthenticate(t *testing.T) {
	require := require.New(t)

	tokens := itokensjwt.ProvideITokens(itokensjwt.SecretKeyExample, coreutils.NewITime())
	appTokens := payloads.ProvideIAppTokensFactory(tokens).New(istructs.AppQName_test1_app1)
	login := "testlogin"
	pp := payloads.PrincipalPayload{
		Login:       login,
		SubjectKind: istructs.SubjectKind_User,
		ProfileWSID: 1,
	}
	userToken, err := appTokens.IssueToken(time.Minute, &pp)
	require.NoError(err)

	testRole := appdef.NewQName(appdef.SysPackage, "test")
	apiKeyToken, err := IssueAPIToken(appTokens, time.Hour, []appdef.QName{
		testRole,
	}, 2, pp)
	require.NoError(err)

	pp.ProfileWSID = istructs.NullWSID
	sysToken, err := appTokens.IssueToken(time.Minute, &pp)
	require.NoError(err)
	pp.ProfileWSID = 1

	pp.SubjectKind = istructs.SubjectKind_Device
	deviceToken, err := appTokens.IssueToken(time.Minute, &pp)
	require.NoError(err)

	qNameCDocComputers := appdef.NewQName("untill", "computers")

	appStructs := AppStructsWithTestStorage(istructs.AppQName_test1_app1, map[istructs.WSID]map[appdef.QName]map[istructs.RecordID]map[string]interface{}{
		// WSID 1 is the user profile
		istructs.WSID(1): {
			qNameViewDeviceProfileWSIDIdx: {
				1: {
					field_dummy:                 int32(1),
					field_DeviceProfileWSID:     int64(1),
					appdef.SystemField_IsActive: true,
					field_ComputersID:           istructs.RecordID(2),
					field_RestaurantComputersID: istructs.RecordID(3),
				},
			},
			// wrong to store in the user profile wsid, but ok for test
			qNameCDocComputers: {
				2: {
					appdef.SystemField_QName:    qNameCDocComputers,
					appdef.SystemField_IsActive: true,
				},
				5: {
					appdef.SystemField_QName:    qNameCDocComputers,
					appdef.SystemField_IsActive: false,
				},
			},
			// not used for authorization, but keep for an example
			appdef.NewQName("untill", "restaurant_computers"): {
				3: {},
				6: {},
			},
		},

		// workspace owned by the user
		istructs.WSID(2): {
			qNameCDocWorkspaceDescriptor: {
				// cdoc.sys.WorkspaceDescriptor.ID=1, .OwnerWSID=1
				1: {
					"OwnerWSID": int64(1), // the same as ProfileWSID
				},
			},
		},

		// child workspace. Parent is WSID 2
		istructs.WSID(3): {
			qNameCDocWorkspaceDescriptor: {
				// cdoc.sys.WorkspaceDescriptor.ID=1, .OwnerWSID=2
				1: {
					"OwnerWSID": int64(2),
				},
			},
		},
	})

	testCases := []struct {
		desc               string
		req                iauthnz.AuthnRequest
		expectedPrincipals []iauthnz.Principal
		subjects           []appdef.QName
	}{
		{
			desc: "no auth -> host + Guest user",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 1,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_User, Name: istructs.SysGuestLogin, WSID: istructs.GuestWSID},
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleAnonymous},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "system token -> host and system role",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 1,
				Token:       sysToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 1, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 1, QName: iauthnz.QNameRoleSystem},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "request to profile -> Everyone + authenticatedUser + user + profileOwner",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 1,
				Token:       userToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 1, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_User, WSID: 1, Name: login},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 1, QName: iauthnz.QNameRoleProfileOwner},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "request to an owned workspace -> everyone + AuthenticatedUser + user + workspaceOwner + host",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 2,
				Token:       userToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_User, WSID: 1, Name: login},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleWorkspaceOwner},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "request to a non-owned workspace -> Everyone + AuthenticatedUser + user + host",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: alienWSID,
				Token:       userToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: alienWSID, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_User, WSID: 1, Name: login},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "request to a non-initialized workspace -> host + user",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: nonInitedWSID,
				Token:       userToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: nonInitedWSID, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_User, WSID: 1, Name: login},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "device -> Everyone + AuthenticatedUser + device + ProfileOwner + host",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 1,
				Token:       deviceToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 1, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_Device, WSID: 1},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 1, QName: iauthnz.QNameRoleProfileOwner},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
		},
		{
			desc: "ResellerAdmin -> WorkspaceAdmin",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 2,
				Token:       userToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: qNameRoleResellersAdmin},
				{Kind: iauthnz.PrincipalKind_User, WSID: 1, Name: login},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleWorkspaceOwner},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleWorkspaceAdmin},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
			subjects: []appdef.QName{qNameRoleResellersAdmin},
		},
		{
			desc: "UntillPaymentsReseller -> WorkspaceAdmin",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 2,
				Token:       userToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: qNameRoleUntillPaymentsReseller},
				{Kind: iauthnz.PrincipalKind_User, WSID: 1, Name: login},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleWorkspaceOwner},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleWorkspaceAdmin},
				{Kind: iauthnz.PrincipalKind_Host, Name: "127.0.0.1"},
			},
			subjects: []appdef.QName{qNameRoleUntillPaymentsReseller},
		},
		{
			desc: "IsPersonalAccessToken -> principals are built by provided roles only",
			req: iauthnz.AuthnRequest{
				Host:        "127.0.0.1",
				RequestWSID: 2,
				Token:       apiKeyToken,
			},
			expectedPrincipals: []iauthnz.Principal{
				{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleEveryone},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: iauthnz.QNameRoleAuthenticatedUser},
				{Kind: iauthnz.PrincipalKind_Role, WSID: 2, QName: testRole},
			},
		},
	}
	var subjects *[]appdef.QName
	subjectsGetter := func(context.Context, string, istructs.IAppStructs, istructs.WSID) ([]appdef.QName, error) {
		return *subjects, nil
	}
	authn := NewDefaultAuthenticator(subjectsGetter, TestIsDeviceAllowedFuncs)
	for _, tc := range testCases {
		localVarSubjects := &tc.subjects
		t.Run(tc.desc, func(t *testing.T) {
			subjects = localVarSubjects
			principals, _, err := authn.Authenticate(context.Background(), appStructs, appTokens, tc.req)
			require.NoError(err)
			require.Equal(tc.expectedPrincipals, principals, tc.desc)
		})
	}
}

func TestACLAllow(t *testing.T) {
	defer logger.SetLogLevel(logger.LogLevelInfo)
	require := require.New(t)
	testQName1 := appdef.NewQName(appdef.SysPackage, "testQName")

	type req struct {
		req  iauthnz.AuthzRequest
		prns [][]iauthnz.Principal
	}

	cases := []struct {
		acl  ACL
		reqs []req
	}{
		{
			acl: ACL{
				{
					desc: "allow rule",
					pattern: PatternType{
						qNamesPattern:  []appdef.QName{testQName1},
						opKindsPattern: []iauthnz.OperationKindType{iauthnz.OperationKind_INSERT},
						principalsPattern: [][]iauthnz.Principal{
							// OR
							{{Kind: iauthnz.PrincipalKind_User, Name: "testname", ID: 1, WSID: 2}},
							{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceOwner}},
						},
					},
					policy: ACPolicy_Allow,
				},
			},
			reqs: []req{
				{
					req: iauthnz.AuthzRequest{
						OperationKind: iauthnz.OperationKind_INSERT,
						Resource:      testQName1,
						Fields:        []string{"fld1", "fld2"}, // just an example
					},
					prns: [][]iauthnz.Principal{
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "wrong",
							},
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testname",
								ID:   1,
								WSID: 2,
							},
							{
								Kind: iauthnz.PrincipalKind_Host,
								Name: "127.0.0.1",
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleProfileOwner,
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleWorkspaceOwner,
							},
							{
								Kind:  iauthnz.PrincipalKind_Group,
								QName: appdef.NewQName(appdef.SysPackage, "testGroup"),
							},
							{
								Kind: iauthnz.PrincipalKind_Device,
								Name: "testDevice",
							},
						},
					},
				},
			},
		},
		{
			acl: ACL{
				{
					desc: "non-first principal in the pattern matches",
					pattern: PatternType{
						qNamesPattern: []appdef.QName{qNameCmdCreateUPProfile},
						principalsPattern: [][]iauthnz.Principal{
							// OR
							{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsUser}},
							{{Kind: iauthnz.PrincipalKind_Role, QName: qNameRoleUntillPaymentsReseller, WSID: 42}},
						},
					},
					policy: ACPolicy_Allow,
				},
			},
			reqs: []req{
				{
					req: iauthnz.AuthzRequest{
						OperationKind: iauthnz.OperationKind_EXECUTE,
						Resource:      qNameCmdCreateUPProfile,
					},
					prns: [][]iauthnz.Principal{
						{
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: qNameRoleUntillPaymentsReseller,
								WSID:  42,
							},
						},
					},
				},
			},
		},
		{
			acl: ACL{
				{
					desc:   "allow everything",
					policy: ACPolicy_Allow,
				},
				{
					desc: "but deny to select one field",
					pattern: PatternType{
						opKindsPattern: []iauthnz.OperationKindType{iauthnz.OperationKind_SELECT},
						qNamesPattern:  []appdef.QName{qNameCDocUntillPayments},
						fieldsPattern:  [][]string{{appdef.SystemField_IsActive}},
					},
					policy: ACPolicy_Deny,
				},
			},
			reqs: []req{
				{
					req: iauthnz.AuthzRequest{
						OperationKind: iauthnz.OperationKind_SELECT,
						Resource:      qNameCDocUntillPayments,
						Fields:        []string{appdef.SystemField_ID}, // select non-denied field -> expect allow
					},
					prns: [][]iauthnz.Principal{
						nil,
					},
				},
			},
		},
	}

	for _, c := range cases {
		authz := implIAuthorizer{acl: c.acl}
		for _, req := range c.reqs {
			for _, prns := range req.prns {
				ok, err := authz.Authorize(nil, prns, req.req)
				require.NoError(err)
				require.True(ok)
			}
		}
	}
}

func TestACLDeny(t *testing.T) {
	logger.SetLogLevel(logger.LogLevelVerbose)
	defer logger.SetLogLevel(logger.LogLevelInfo)
	require := require.New(t)
	testQName1 := appdef.NewQName(appdef.SysPackage, "testQName")

	type req struct {
		req  iauthnz.AuthzRequest
		prns [][]iauthnz.Principal
	}

	cases := []struct {
		acl  ACL
		reqs []req
	}{
		{
			acl: ACL{
				{
					desc: "deny rule",
					pattern: PatternType{
						qNamesPattern:  []appdef.QName{testQName1},
						opKindsPattern: []iauthnz.OperationKindType{iauthnz.OperationKind_INSERT},
						principalsPattern: [][]iauthnz.Principal{
							// OR
							{{Kind: iauthnz.PrincipalKind_User, Name: "testnamefordeny", ID: 1, WSID: 2}},
							{{Kind: iauthnz.PrincipalKind_Role, QName: iauthnz.QNameRoleWorkspaceOwner}},
						},
					},
					policy: ACPolicy_Deny,
				},
			},
			reqs: []req{
				{
					req: iauthnz.AuthzRequest{
						OperationKind: iauthnz.OperationKind_INSERT,
						Resource:      testQName1,
						Fields:        []string{"fld1", "fld2"}, // just an example
					},
					prns: [][]iauthnz.Principal{
						{{}},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testname",
							},
						},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "wrongName",
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleWorkspaceOwner,
							},
						},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testname",
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleWorkspaceOwner,
							},
						},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testname",
								ID:   1,
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleWorkspaceOwner,
							},
						},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testname",
								WSID: 2,
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleWorkspaceOwner,
							},
						},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testnamefordeny",
								ID:   1,
								WSID: 42,
							},
						},
						{
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "wrong",
							},
							{
								Kind: iauthnz.PrincipalKind_User,
								Name: "testnamefordeny",
								ID:   1,
								WSID: 2,
							},
							{
								Kind: iauthnz.PrincipalKind_Host,
								Name: "127.0.0.1",
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleProfileOwner,
							},
							{
								Kind:  iauthnz.PrincipalKind_Role,
								QName: iauthnz.QNameRoleWorkspaceOwner,
							},
						},
					},
				},
			},
		},
		{
			acl: ACL{
				{
					desc:   "allow all",
					policy: ACPolicy_Allow,
				},
				{
					desc: "but deny to select one field",
					pattern: PatternType{
						opKindsPattern: []iauthnz.OperationKindType{iauthnz.OperationKind_SELECT},
						qNamesPattern:  []appdef.QName{qNameCDocUntillPayments},
						fieldsPattern:  [][]string{{appdef.SystemField_IsActive}},
					},
					policy: ACPolicy_Deny,
				},
			},
			reqs: []req{
				{
					req: iauthnz.AuthzRequest{
						OperationKind: iauthnz.OperationKind_SELECT,
						Resource:      qNameCDocUntillPayments,
						Fields:        []string{appdef.SystemField_IsActive, appdef.SystemField_ID},
					},
					prns: [][]iauthnz.Principal{
						nil,
					},
				},
			},
		},
	}

	for _, c := range cases {
		authz := implIAuthorizer{acl: c.acl}
		for _, req := range c.reqs {
			for _, prns := range req.prns {
				ok, err := authz.Authorize(nil, prns, req.req)
				require.NoError(err)
				require.False(ok)
			}
		}
	}
}

func TestErrors(t *testing.T) {
	require := require.New(t)

	tokens := itokensjwt.ProvideITokens(itokensjwt.SecretKeyExample, coreutils.NewITime())
	appTokens := payloads.ProvideIAppTokensFactory(tokens).New(istructs.AppQName_test1_app1)

	appStructs := &implIAppStructs{}
	authn := NewDefaultAuthenticator(TestSubjectRolesGetter, TestIsDeviceAllowedFuncs)

	t.Run("wrong token", func(t *testing.T) {
		req := iauthnz.AuthnRequest{
			RequestWSID: 1,
			Token:       "wrong token",
		}
		_, _, err := authn.Authenticate(context.Background(), appStructs, appTokens, req)
		require.Error(err)
	})

	t.Run("unsupported subject kind", func(t *testing.T) {

		pp := payloads.PrincipalPayload{
			Login:       "testlogin",
			SubjectKind: istructs.SubjectKind_FakeLast,
			ProfileWSID: 1,
		}
		token, err := appTokens.IssueToken(time.Minute, &pp)
		require.NoError(err)
		req := iauthnz.AuthnRequest{
			RequestWSID: 1,
			Token:       token,
		}
		_, _, err = authn.Authenticate(context.Background(), appStructs, appTokens, req)
		require.Error(err)
	})

	t.Run("personal access token for a system role", func(t *testing.T) {
		for _, sysRole := range iauthnz.SysRoles {
			token, err := IssueAPIToken(appTokens, time.Hour, []appdef.QName{sysRole}, 1, payloads.PrincipalPayload{})
			require.ErrorIs(err, ErrPersonalAccessTokenOnSystemRole)
			require.Empty(token)
		}
	})

	t.Run("personal access token for NullWSID", func(t *testing.T) {
		token, err := IssueAPIToken(appTokens, time.Hour, []appdef.QName{appdef.NewQName(appdef.SysPackage, "test")}, istructs.NullWSID, payloads.PrincipalPayload{})
		require.ErrorIs(err, ErrPersonalAccessTokenOnNullWSID)
		require.Empty(token)
	})
}

// with principals cache:  1455242       782.8 ns/op	     432 B/op	       9 allocs/op
// without principals cache: 45534	     24370 ns/op	    7964 B/op	     126 allocs/op
func BenchmarkBasic(b *testing.B) {
	tokens := itokensjwt.ProvideITokens(itokensjwt.SecretKeyExample, coreutils.NewITime())
	appTokens := payloads.ProvideIAppTokensFactory(tokens).New(istructs.AppQName_test1_app1)
	pp := payloads.PrincipalPayload{
		Login:       "testlogin",
		SubjectKind: istructs.SubjectKind_User,
		ProfileWSID: 1,
	}
	token, err := appTokens.IssueToken(time.Minute, &pp)
	require.NoError(b, err)
	var principals []iauthnz.Principal
	appStructs := &implIAppStructs{}
	authn := NewDefaultAuthenticator(TestSubjectRolesGetter, TestIsDeviceAllowedFuncs)
	authz := NewDefaultAuthorizer()
	reqn := iauthnz.AuthnRequest{
		Host:        "127.0.0.1",
		RequestWSID: 1,
		Token:       token,
	}
	reqz := iauthnz.AuthzRequest{
		OperationKind: iauthnz.OperationKind_EXECUTE,
		Resource:      appdef.NewQName(appdef.SysPackage, "SomeCmd"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		principals, _, err = authn.Authenticate(context.Background(), appStructs, appTokens, reqn)
		if err != nil {
			b.Fatal()
		}
		ok, err := authz.Authorize(appStructs, principals, reqz)
		if !ok || err != nil {
			b.Fatal()
		}
	}
}

func AppStructsWithTestStorage(appQName appdef.AppQName, data map[istructs.WSID]map[appdef.QName]map[istructs.RecordID]map[string]interface{}) istructs.IAppStructs {
	recs := &implIRecords{data: data}
	return &implIAppStructs{records: recs, views: &implIViewRecords{records: recs}, appQName: appQName}
}

type implIAppStructs struct {
	records  *implIRecords
	views    *implIViewRecords
	appQName appdef.AppQName
}

func (as *implIAppStructs) AppDef() appdef.IAppDef                             { panic("") }
func (as *implIAppStructs) Events() istructs.IEvents                           { panic("") }
func (as *implIAppStructs) Records() istructs.IRecords                         { return as.records }
func (as *implIAppStructs) ViewRecords() istructs.IViewRecords                 { return as.views }
func (as *implIAppStructs) ObjectBuilder(appdef.QName) istructs.IObjectBuilder { panic("") }
func (as *implIAppStructs) Resources() istructs.IResources                     { panic("") }
func (as *implIAppStructs) ClusterAppID() istructs.ClusterAppID                { panic("") }
func (as *implIAppStructs) AppQName() appdef.AppQName                          { return as.appQName }
func (as *implIAppStructs) IsFunctionRateLimitsExceeded(appdef.QName, istructs.WSID) bool {
	panic("")
}
func (as *implIAppStructs) DescribePackageNames() []string              { panic("") }
func (as *implIAppStructs) DescribePackage(string) interface{}          { panic("") }
func (as *implIAppStructs) SyncProjectors() istructs.Projectors         { panic("") }
func (as *implIAppStructs) AsyncProjectors() istructs.Projectors        { panic("") }
func (as *implIAppStructs) CUDValidators() []istructs.CUDValidator      { panic("") }
func (as *implIAppStructs) EventValidators() []istructs.EventValidator  { panic("") }
func (as *implIAppStructs) NumAppWorkspaces() istructs.NumAppWorkspaces { panic("") }
func (as *implIAppStructs) AppTokens() istructs.IAppTokens              { panic("") }

type implIRecords struct {
	data map[istructs.WSID]map[appdef.QName]map[istructs.RecordID]map[string]interface{}
}

func (implIRecords) Apply(istructs.IPLogEvent) error                          { panic("") }
func (implIRecords) Apply2(istructs.IPLogEvent, func(istructs.IRecord)) error { panic("") }
func (implIRecords) PutJSON(istructs.WSID, map[appdef.FieldName]any) error    { panic("") }
func (r *implIRecords) Get(wsid istructs.WSID, _ bool, id istructs.RecordID) (istructs.IRecord, error) {
	if wsData, ok := r.data[wsid]; ok {
		for qName, qNameRecs := range wsData {
			for recID, recData := range qNameRecs {
				if recID == id {
					return &implIRecord{TestObject: coreutils.TestObject{Data: recData}, qName: qName}, nil
				}
			}
		}
	}
	return istructsmem.NewNullRecord(id), nil
}
func (implIRecords) GetBatch(istructs.WSID, bool, []istructs.RecordGetBatchItem) error { panic("") }
func (r *implIRecords) GetSingleton(wsid istructs.WSID, qName appdef.QName) (record istructs.IRecord, err error) {
	if wsData, ok := r.data[wsid]; ok {
		if qNameRecs, ok := wsData[qName]; ok {
			if len(qNameRecs) > 1 {
				panic(">1 records for a singleton")
			}
			for _, data := range qNameRecs {
				return &implIRecord{qName: qName, TestObject: coreutils.TestObject{Data: data}}, nil
			}
		}
	}
	return istructsmem.NewNullRecord(istructs.NullRecordID), nil
}
func (implIRecords) Read(istructs.WSID, bool, istructs.RecordID) (istructs.IRecord, error) { panic("") }

func (r *implIRecords) GetSingletonID(qName appdef.QName) (istructs.RecordID, error) {
	for _, wsData := range r.data {
		if qNameRecs, ok := wsData[qName]; ok {
			if len(qNameRecs) > 1 {
				panic(">1 records for a singleton")
			}
			for _, data := range qNameRecs {
				iRecord := &implIRecord{qName: qName, TestObject: coreutils.TestObject{Data: data}}
				return iRecord.ID(), nil
			}
		}
	}

	return istructs.NullRecordID, nil
}

type implIRecord struct {
	coreutils.TestObject
	qName appdef.QName
}

func (r *implIRecord) QName() appdef.QName                                    { return r.qName }
func (r *implIRecord) ID() istructs.RecordID                                  { return r.AsRecordID(appdef.SystemField_ID) }
func (implIRecord) Parent() istructs.RecordID                                 { panic("") }
func (implIRecord) Container() string                                         { panic("") }
func (implIRecord) RecordIDs(bool) func(func(string, istructs.RecordID) bool) { panic("") }
func (r *implIRecord) FieldNames(cb func(fieldName string) bool)              { r.TestObject.FieldNames(cb) }

type implIViewRecords struct {
	records *implIRecords
}

func (implIViewRecords) KeyBuilder(view appdef.QName) istructs.IKeyBuilder {
	return &implIKeyBuilder{qName: view, TestObject: coreutils.TestObject{Data: map[string]interface{}{}}}
}
func (implIViewRecords) NewValueBuilder(appdef.QName) istructs.IValueBuilder { panic("") }
func (implIViewRecords) UpdateValueBuilder(appdef.QName, istructs.IValue) istructs.IValueBuilder {
	panic("")
}
func (implIViewRecords) Put(istructs.WSID, istructs.IKeyBuilder, istructs.IValueBuilder) error {
	panic("")
}
func (implIViewRecords) PutBatch(istructs.WSID, []istructs.ViewKV) error                  { panic("") }
func (implIViewRecords) PutJSON(istructs.WSID, map[appdef.FieldName]any) error            { panic("") }
func (implIViewRecords) Get(istructs.WSID, istructs.IKeyBuilder) (istructs.IValue, error) { panic("") }
func (vr *implIViewRecords) GetBatch(workspace istructs.WSID, kv []istructs.ViewRecordGetBatchItem) error {
	if wsData, ok := vr.records.data[workspace]; ok {
		for biIdx, bi := range kv {
			kb := bi.Key.(*implIKeyBuilder)
			if qNameRecs, ok := wsData[kb.qName]; ok {
				for _, qNameRec := range qNameRecs {
					matchedFields := 0
					for keyField, keyValue := range kb.Data {
						if recFieldValue, ok := qNameRec[keyField]; ok {
							if recFieldValue == keyValue {
								matchedFields++
							}
						}
					}
					if len(kb.Data) == matchedFields {
						kv[biIdx].Ok = true
						kv[biIdx].Value = &implIValue{TestObject: coreutils.TestObject{Data: qNameRec}}
						break
					}
				}
			}
		}
	}
	return nil
}
func (implIViewRecords) Read(context.Context, istructs.WSID, istructs.IKeyBuilder, istructs.ValuesCallback) error {
	panic("")
}

type implIKeyBuilder struct {
	coreutils.TestObject
	qName appdef.QName
}

func (kb *implIKeyBuilder) PartitionKey() istructs.IRowWriter             { return &kb.TestObject }
func (kb *implIKeyBuilder) ClusteringColumns() istructs.IRowWriter        { return &kb.TestObject }
func (kb *implIKeyBuilder) Equals(istructs.IKeyBuilder) bool              { panic("implement me") }
func (kb *implIKeyBuilder) ToBytes(istructs.WSID) ([]byte, []byte, error) { return nil, nil, nil }

type implIValue struct {
	coreutils.TestObject
}

func (v *implIValue) AsRecord(name string) (record istructs.IRecord) { panic("") }
func (v *implIValue) AsEvent(name string) (event istructs.IDbEvent)  { panic("") }
