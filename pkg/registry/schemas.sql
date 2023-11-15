-- Copyright (c) 2021-present unTill Pro, Ltd.
-- @author Denis Gribanov

ALTER WORKSPACE AppWorkspaceWS (
	TABLE Login INHERITS CDoc (
		ProfileCluster int32 NOT NULL,
		PwdHash bytes NOT NULL,
		AppName varchar NOT NULL,
		SubjectKind int32,
		LoginHash varchar NOT NULL,
		WSID int64,
		WSError varchar(1024),                          -- to be written after workspace init
		WSKindInitializationData varchar(1024) NOT NULL -- to be written after workspace init
	);

	TYPE CreateLoginParams (
		Login text NOT NULL,
		AppName text NOT NULL,
		SubjectKind int32 NOT NULL,
		WSKindInitializationData text(1024) NOT NULL,
		ProfileCluster int32 NOT NULL
	);

	TYPE CreateLoginUnloggedParams (
		Password text NOT NULL
	);

	TYPE IssuePrincipalTokenParams (
		Login text NOT NULL,
		Password text NOT NULL,
		AppName text NOT NULL
	);

	TYPE IssuePrincipalTokenResult (
		PrincipalToken text NOT NULL,
		WSID int64 NOT NULL,
		WSError text(1024) NOT NULL
	);

	TYPE ChangePasswordParams (
		Login text NOT NULL,
		AppName text NOT NULL
	);

	TYPE ChangePasswordUnloggedParams (
		OldPassword text NOT NULL,
		NewPassword text NOT NULL
	);

	TYPE InitiateResetPasswordByEmailParams (
		AppName text NOT NULL,
		Email text NOT NULL,
		Language text
	);

	TYPE InitiateResetPasswordByEmailResult (
		VerificationToken text NOT NULL,
		ProfileWSID int64 NOT NULL
	);

	TYPE IssueVerifiedValueTokenForResetPasswordParams (
		VerificationToken text(32768) NOT NULL,
		VerificationCode text NOT NULL,
		ProfileWSID int64 NOT NULL,
		AppName text NOT NULL
	);

	TYPE IssueVerifiedValueTokenForResetPasswordResult (
		VerifiedValueToken text NOT NULL
	);

	TYPE ResetPasswordByEmailParams (
		AppName text NOT NULL
	);

	TYPE ResetPasswordByEmailUnloggedParams (
		Email text NOT NULL VERIFIABLE,
		NewPwd text NOT NULL
	);

	VIEW LoginIdx (
		AppWSID int64 NOT NULL,
		AppIDLoginHash text NOT NULL,
		CDocLoginID ref(Login) NOT NULL,
		PRIMARY KEY((AppWSID), AppIDLoginHash)
	) AS RESULT OF ProjectorLoginIdx;

	EXTENSION ENGINE BUILTIN (
		COMMAND CreateLogin (CreateLoginParams, UNLOGGED CreateLoginUnloggedParams);
		COMMAND ChangePassword (ChangePasswordParams, UNLOGGED ChangePasswordUnloggedParams);
		COMMAND ResetPasswordByEmail (ResetPasswordByEmailParams, UNLOGGED ResetPasswordByEmailUnloggedParams);
		QUERY IssuePrincipalToken (IssuePrincipalTokenParams) RETURNS IssuePrincipalTokenResult;
		QUERY InitiateResetPasswordByEmail (InitiateResetPasswordByEmailParams) RETURNS InitiateResetPasswordByEmailResult;
		QUERY IssueVerifiedValueTokenForResetPassword (IssueVerifiedValueTokenForResetPasswordParams) RETURNS IssueVerifiedValueTokenForResetPasswordResult;
		SYNC PROJECTOR ProjectorLoginIdx AFTER INSERT ON Login INTENTS(View(LoginIdx));
		PROJECTOR InvokeCreateWorkspaceID_registry AFTER INSERT ON(Login);
	);
);
