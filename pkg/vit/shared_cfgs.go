/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package vit

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/parser"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/smtp"
	"github.com/voedger/voedger/pkg/sys/sysprovide"
	sys_test_template "github.com/voedger/voedger/pkg/vit/testdata"
	"github.com/voedger/voedger/pkg/vvm"
	builtinapps "github.com/voedger/voedger/pkg/vvm/builtin"
	"github.com/voedger/voedger/pkg/vvm/storage"
)

const (
	TestEmail  = "123@123.com"
	TestEmail2 = "124@124.com"

	app1PkgName = "app1pkg"
	App1PkgPath = "github.com/voedger/voedger/pkg/vit/app1pkg"

	app2PkgName = "app2pkg"
	app2PkgPath = "github.com/voedger/voedger/pkg/vit/app2pkg"
)

const (
	Field_Year            = "Year"
	Field_Month           = "Month"
	Field_Day             = "Day"
	Field_StringValue     = "StringValue"
	Field_Number          = "Number"
	Field_CharCode        = "CharCode"
	Field_Code            = "Code"
	Field_FirstName       = "FirstName"
	Field_LastName        = "LastName"
	Field_DOB             = "DOB"
	Field_Wallet          = "Wallet"
	Field_Balance         = "Balance"
	Field_Currency        = "Currency"
	Field_Name            = "Name"
	Field_Country         = "Country"
	Field_Client          = "Client"
	Field_Withdraw        = "Withdraw"
	Field_Deposit         = "Deposit"
	Field_Capabilities    = "Capabilities"
	Field_Cfg             = "Cfg"
	Field_GroupA          = "GroupA"
	Field_GroupB          = "GroupB"
	Field_Blob            = "Blob"
	Field_BlobReadDenied  = "BlobReadDenied"
	testSMTPPwdSecretName = "smtp-pwd-secret-name"
)

var (
	QNameApp1_TestWSKind                     = appdef.NewQName(app1PkgName, "test_ws")
	QNameApp1_TestWSKind_another             = appdef.NewQName(app1PkgName, "test_ws_another")
	QNameTestView                            = appdef.NewQName(app1PkgName, "View")
	QNameApp1_ViewCategoryIdx                = appdef.NewQName(app1PkgName, "CategoryIdx")
	QNameApp1_ViewDailyIdx                   = appdef.NewQName(app1PkgName, "DailyIdx")
	QNameApp1_ViewDailyIdxSmall              = appdef.NewQName(app1PkgName, "DailyIdxSmall")
	QNameApp1_ViewClients                    = appdef.NewQName(app1PkgName, "Clients")
	QNameApp1_TestEmailVerificationDoc       = appdef.NewQName(app1PkgName, "Doc")
	QNameApp1_DocConstraints                 = appdef.NewQName(app1PkgName, "DocConstraints")
	QNameApp1_DocConstraintsString           = appdef.NewQName(app1PkgName, "DocConstraintsString")
	QNameApp1_DocConstraintsFewUniques       = appdef.NewQName(app1PkgName, "DocConstraintsFewUniques")
	QNameApp1_DocConstraintsOldAndNewUniques = appdef.NewQName(app1PkgName, "DocConstraintsOldAndNewUniques")
	QNameApp1_CDocCategory                   = appdef.NewQName(app1PkgName, "category")
	QNameApp1_CDocDaily                      = appdef.NewQName(app1PkgName, "Daily")
	QNameApp1_CDocCurrency                   = appdef.NewQName(app1PkgName, "Currency")
	QNameApp1_CDocCountry                    = appdef.NewQName(app1PkgName, "Country")
	QNameApp1_CDocCfg                        = appdef.NewQName(app1PkgName, "Cfg")
	QNameApp1_CDocBatch                      = appdef.NewQName(app1PkgName, "Batch")
	QNameApp1_CRecordTask                    = appdef.NewQName(app1PkgName, "Task")
	QNameApp1_WDocClient                     = appdef.NewQName(app1PkgName, "Client")
	QNameApp1_WDocWallet                     = appdef.NewQName(app1PkgName, "Wallet")
	QNameApp1_WDocCapabilities               = appdef.NewQName(app1PkgName, "Capabilities")
	QNameCmdRated                            = appdef.NewQName(app1PkgName, "RatedCmd")
	QNameQryRated                            = appdef.NewQName(app1PkgName, "RatedQry")
	QNameCmdIPRated                          = appdef.NewQName(app1PkgName, "IPRatedCmd")
	QNameQryIPRated                          = appdef.NewQName(app1PkgName, "IPRatedQry")
	QNameODoc1                               = appdef.NewQName(app1PkgName, "odoc1")
	QNameODoc2                               = appdef.NewQName(app1PkgName, "odoc2")
	TestSMTPCfg                              = smtp.Cfg{
		Host:      "smtp.testserver.com",
		Port:      1,
		Username:  "username@gmail.com",
		PwdSecret: testSMTPPwdSecretName,
	}
	QNameDocWithBLOB  = appdef.NewQName(app1PkgName, "DocWithBLOB")
	QNameDocBLOB      = appdef.NewQName(app1PkgName, "DocBLOB")
	QNameODocWithBLOB = appdef.NewQName(app1PkgName, "ODocWithBLOB")

	// BLOBMaxSize 5
	SharedConfig_App1 = NewSharedVITConfig(
		WithApp(istructs.AppQName_test1_app1, ProvideApp1,
			WithWorkspaceTemplate(QNameApp1_TestWSKind, "test_template", sys_test_template.TestTemplateFS),
			WithUserLogin("login", "pwd"),
			WithUserLogin(TestEmail, "1"),
			WithUserLogin(TestEmail2, "1"),
			WithChildWorkspace(QNameApp1_TestWSKind, "test_ws", "test_template", "", "login", map[string]interface{}{"IntFld": 42},
				WithChild(QNameApp1_TestWSKind, "test_ws2", "test_template", "", "login", map[string]interface{}{"IntFld": 42},
					WithSubject(TestEmail, istructs.SubjectKind_User, []appdef.QName{iauthnz.QNameRoleWorkspaceOwner}))),
			WithChildWorkspace(QNameApp1_TestWSKind_another, "test_ws_another", "", "", "login", map[string]interface{}{}),
			WithChildWorkspace(QNameApp1_TestWSKind, "test_ws_qp2", "test_template", "", "login", map[string]interface{}{"IntFld": 42}),
			WithChildWorkspace(QNameApp1_TestWSKind, "test_ws3", "test_template", "", "login", map[string]interface{}{"IntFld": 42}),
		),
		WithApp(istructs.AppQName_test1_app2, ProvideApp2, WithUserLogin("login", "1")),
		WithVVMConfig(func(cfg *vvm.VVMConfig) {
			const app1_BLOBMaxSize = 5
			cfg.BLOBMaxSize = app1_BLOBMaxSize

			cfg.SMTPConfig = TestSMTPCfg
		}),
		WithSecret(testSMTPPwdSecretName, []byte("smtpPassword")),
		WithCleanup(func(_ *VIT) {
			MockCmdExec = func(input string, args istructs.ExecCommandArgs) error { panic("") }
			MockQryExec = func(input string, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) error { panic("") }
		}),
	)
	MockQryExec func(input string, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) error
	MockCmdExec func(input string, args istructs.ExecCommandArgs) error
)

func ProvideApp2(apis builtinapps.APIs, cfg *istructsmem.AppConfigType, ep extensionpoints.IExtensionPoint) builtinapps.Def {
	sysPackageFS := sysprovide.Provide(cfg)
	app2PackageFS := parser.PackageFS{
		Path: app2PkgPath,
		FS:   SchemaTestApp2FS,
	}
	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app2PkgName, "testCmd"), istructsmem.NullCommandExec))
	ep.AddNamed(builtinapps.EPIsDeviceAllowedFunc, func(as istructs.IAppStructs, requestWSID istructs.WSID, deviceProfileWSID istructs.WSID) (ok bool, err error) {
		// simulate we could not work in any non-profile WS
		return false, err
	})
	return builtinapps.Def{
		AppQName:                istructs.AppQName_test1_app2,
		Packages:                []parser.PackageFS{sysPackageFS, app2PackageFS},
		AppDeploymentDescriptor: TestAppDeploymentDescriptor,
	}
}

func ProvideApp2WithJob(apis builtinapps.APIs, cfg *istructsmem.AppConfigType, ep extensionpoints.IExtensionPoint) builtinapps.Def {
	sysPackageFS := sysprovide.Provide(cfg)
	app2PackageFS := parser.PackageFS{
		Path: app2PkgPath,
		FS:   SchemaTestApp2WithJobFS,
	}
	ep.AddNamed(builtinapps.EPIsDeviceAllowedFunc, func(as istructs.IAppStructs, requestWSID istructs.WSID, deviceProfileWSID istructs.WSID) (ok bool, err error) {
		// simulate we could not work in any non-profile WS
		return false, err
	})
	cfg.AddJobs(istructsmem.BuiltinJob{
		Name: appdef.NewQName(app2PkgName, "Job1_builtin"),
		Func: func(st istructs.IState, intents istructs.IIntents) error {
			jobsQName := appdef.NewQName("app2pkg", "Jobs")
			kb, err := st.KeyBuilder(sys.Storage_View, jobsQName)
			if err != nil {
				return err
			}
			kb.PutInt64("RunUnixMilli", apis.ITime.Now().UnixMilli())
			kb.PutInt32("Dummy1", 1)
			vb, err := intents.NewValue(kb)
			if err != nil {
				return err
			}
			vb.PutInt32("Dummy2", 1)
			return nil
		},
	})
	return builtinapps.Def{
		AppQName:                istructs.AppQName_test1_app2,
		Packages:                []parser.PackageFS{sysPackageFS, app2PackageFS},
		AppDeploymentDescriptor: TestAppDeploymentDescriptor,
	}
}

func ProvideApp2WithJobSendMail(apis builtinapps.APIs, cfg *istructsmem.AppConfigType, ep extensionpoints.IExtensionPoint) builtinapps.Def {
	sysPackageFS := sysprovide.Provide(cfg)
	app2PackageFS := parser.PackageFS{
		Path: app2PkgPath,
		FS:   SchemaTestApp2WithJobSendMailFS,
	}
	cfg.AddJobs(istructsmem.BuiltinJob{
		Name: appdef.NewQName(app2PkgName, "JobSendEmail"),
		Func: func(st istructs.IState, intents istructs.IIntents) error {
			kb, err := st.KeyBuilder(sys.Storage_SendMail, appdef.NullQName)
			if err != nil {
				return err
			}
			kb.PutInt32(sys.Storage_SendMail_Field_Port, 1)
			kb.PutString(sys.Storage_SendMail_Field_Host, "localhost")
			kb.PutString(sys.Storage_SendMail_Field_Username, "user")
			kb.PutString(sys.Storage_SendMail_Field_Password, "pwd")
			kb.PutString(sys.Storage_SendMail_Field_Subject, "Test Subject")
			kb.PutString(sys.Storage_SendMail_Field_From, "from@test.com")
			kb.PutString(sys.Storage_SendMail_Field_To, "to@test.com")
			kb.PutString(sys.Storage_SendMail_Field_Body, "Test body")
			_, err = intents.NewValue(kb)
			return err
		},
	})
	return builtinapps.Def{
		AppQName: istructs.AppQName_test1_app2,
		Packages: []parser.PackageFS{sysPackageFS, app2PackageFS},
		AppDeploymentDescriptor: appparts.AppDeploymentDescriptor{
			NumParts:         1,
			EnginePoolSize:   appparts.PoolSize(1, 1, 1, 1),
			NumAppWorkspaces: 1,
		},
	}
}

func ProvideApp1(apis builtinapps.APIs, cfg *istructsmem.AppConfigType, ep extensionpoints.IExtensionPoint) builtinapps.Def {
	// sys package
	sysPackageFS := sysprovide.Provide(cfg)

	// for rates test
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		QNameQryRated,
		istructsmem.NullQueryExec,
	))

	cfg.Resources.Add(istructsmem.NewCommandFunction(
		QNameCmdRated,
		istructsmem.NullCommandExec,
	))

	cfg.Resources.Add(istructsmem.NewQueryFunction(
		QNameQryIPRated,
		istructsmem.NullQueryExec,
	))

	cfg.Resources.Add(istructsmem.NewCommandFunction(
		QNameCmdIPRated,
		istructsmem.NullCommandExec,
	))

	cfg.Resources.Add(istructsmem.NewQueryFunction(
		appdef.NewQName(app1PkgName, "MockQry"),
		func(_ context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
			input := args.ArgumentObject.AsString(field_Input)
			return MockQryExec(input, args, callback)
		},
	))

	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(app1PkgName, "MockCmd"),
		func(args istructs.ExecCommandArgs) (err error) {
			input := args.ArgumentObject.AsString(field_Input)
			return MockCmdExec(input, args)
		},
	))

	testCmdResult := appdef.NewQName(app1PkgName, "TestCmdResult")
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(app1PkgName, "TestCmd"),
		func(args istructs.ExecCommandArgs) (err error) {
			key, err := args.State.KeyBuilder(sys.Storage_Result, testCmdResult)
			if err != nil {
				return err
			}

			value, err := args.Intents.NewValue(key)
			if err != nil {
				return err
			}

			arg1 := args.ArgumentObject.AsInt32("Arg1")
			switch arg1 {
			case 1:
				value.PutString("Str", "Str")
				value.PutInt32("Int", 42)
			case 2:
				value.PutInt32("Int", 42)
			case 3:
				value.PutString("Str", "Str")
			case 4:
				value.PutString("Int", "wrong")
			}
			return nil
		},
	))

	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(app1PkgName, "CmdODocOne"),
		istructsmem.NullCommandExec,
	))

	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(app1PkgName, "CmdODocTwo"),
		istructsmem.NullCommandExec,
	))

	cfg.AddAsyncProjectors(
		istructs.Projector{
			Name: appdef.NewQName(app1PkgName, "ProjDummyQName"),
			Func: func(istructs.IPLogEvent, istructs.IState, istructs.IIntents) (err error) { return nil },
		},
		istructs.Projector{
			Name: appdef.NewQName(app1PkgName, "ProjDummy"),
			Func: func(istructs.IPLogEvent, istructs.IState, istructs.IIntents) (err error) { return nil },
		},
		istructs.Projector{
			Name: appdef.NewQName(app1PkgName, "ApplyClient"),
			Func: func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
				for cud := range event.CUDs {
					if cud.QName() != QNameApp1_WDocClient {
						continue
					}
					dob := time.UnixMilli(cud.AsInt64(Field_DOB))
					skbViewClients, err := s.KeyBuilder(sys.Storage_View, QNameApp1_ViewClients)
					if err != nil {
						return err
					}
					skbViewClients.PutInt32(Field_Year, int32(dob.Year()))   // nolint G115
					skbViewClients.PutInt32(Field_Month, int32(dob.Month())) // nolint G115
					skbViewClients.PutInt32(Field_Day, int32(dob.Day()))     // nolint G115
					skbViewClients.PutRecordID(Field_Client, cud.ID())
					svbViewClients, err := intents.NewValue(skbViewClients)
					if err != nil {
						return err
					}
					svbViewClients.PutInt64(state.ColOffset, int64(event.WLogOffset())) // nolint G115
				}
				return
			},
		},
	)

	cfg.AddSyncProjectors(
		istructs.Projector{
			Name: appdef.NewQName(app1PkgName, "ApplyCategoryIdx"),
			Func: func(event istructs.IPLogEvent, st istructs.IState, intents istructs.IIntents) error {
				for cud := range event.CUDs {
					if cud.QName() != QNameApp1_CDocCategory {
						continue
					}
					kb, err := st.KeyBuilder(sys.Storage_View, QNameApp1_ViewCategoryIdx)
					if err != nil {
						return err
					}
					kb.PutInt32("IntFld", 43)
					kb.PutInt32("Dummy", 1)
					b, err := intents.NewValue(kb)
					if err != nil {
						return err
					}
					b.PutInt32("Val", 42)
					b.PutString("Name", cud.AsString("name"))
					b.PutInt64(state.ColOffset, int64(event.WLogOffset())) // nolint G115
				}
				return nil
			},
		},
		istructs.Projector{
			Name: appdef.NewQName(app1PkgName, "ApplyDailyIdx"),
			Func: func(event istructs.IPLogEvent, s istructs.IState, intents istructs.IIntents) (err error) {
				for cud := range event.CUDs {
					if cud.QName() != QNameApp1_CDocDaily {
						continue
					}
					year, month, day := cud.AsInt32(Field_Year), cud.AsInt32(Field_Month), cud.AsInt32(Field_Day)
					stringValue := cud.AsString(Field_StringValue)
					offs := int64(event.WLogOffset()) // nolint G115

					skbViewDailyIdx, err := s.KeyBuilder(sys.Storage_View, QNameApp1_ViewDailyIdx)
					if err != nil {
						return err
					}
					skbViewDailyIdx.PutInt32(Field_Year, year)
					skbViewDailyIdx.PutInt32(Field_Month, month)
					skbViewDailyIdx.PutInt32(Field_Day, day)
					svbViewDailyIdx, err := intents.NewValue(skbViewDailyIdx)
					if err != nil {
						return err
					}
					svbViewDailyIdx.PutString(Field_StringValue, stringValue)
					svbViewDailyIdx.PutInt64(state.ColOffset, offs)

					skbViewDailyIdxSmall, err := s.KeyBuilder(sys.Storage_View, QNameApp1_ViewDailyIdxSmall)
					if err != nil {
						return err
					}
					skbViewDailyIdxSmall.PutInt16(Field_Year, int16(year)) //nolint G115
					skbViewDailyIdxSmall.PutInt8(Field_Month, int8(month)) //nolint G115
					skbViewDailyIdxSmall.PutInt8(Field_Day, int8(day))     //nolint G115
					svbViewDailyIdxSmall, err := intents.NewValue(skbViewDailyIdxSmall)
					if err != nil {
						return err
					}
					svbViewDailyIdxSmall.PutString(Field_StringValue, stringValue)
					svbViewDailyIdxSmall.PutInt64(state.ColOffset, offs)
				}
				return
			},
		},
	)

	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "testCmd"), istructsmem.NullCommandExec))
	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "TestCmdRawArg"), istructsmem.NullCommandExec))
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		appdef.NewQName(app1PkgName, "TestQryRawArg"),
		func(_ context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
			body := args.ArgumentObject.AsString(processors.Field_RawObject_Body)
			return callback(&coreutils.TestObject{Data: map[string]interface{}{"Res": body}})
		},
	))
	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "TestDeniedCmd"), istructsmem.NullCommandExec))
	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "TestDeniedQuery"), istructsmem.NullQueryExec))

	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "QryIntents"), func(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
		kb, err := args.State.KeyBuilder(sys.Storage_Result, appdef.NewQName(app1PkgName, "QryIntentsResult"))
		if err != nil {
			return err
		}
		vb, err := args.Intents.NewValue(kb)
		if err != nil {
			return err
		}
		vb.PutString("Fld1", "hello")
		return nil
	}))

	funcWithResponseIntents := func(args istructs.PrepareArgs, st istructs.IState, intents istructs.IIntents) (err error) {
		kb, err := st.KeyBuilder(sys.Storage_Response, appdef.NullQName)
		if err != nil {
			return err
		}
		vb, err := intents.NewValue(kb)
		if err != nil {
			return err
		}
		vb.PutInt32(sys.Storage_Response_Field_StatusCode, args.ArgumentObject.AsInt32("StatusCodeToReturn"))
		vb.PutString(sys.Storage_Response_Field_ErrorMessage, "error from response intent")
		return nil
	}

	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "CmdWithResponseIntent"), func(args istructs.ExecCommandArgs) (err error) {
		return funcWithResponseIntents(args.PrepareArgs, args.State, args.Intents)
	}))

	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "QryWithResponseIntent"), func(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
		return funcWithResponseIntents(args.PrepareArgs, args.State, args.Intents)
	}))

	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "QryReturnsCategory"), func(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
		q := appdef.NewQName(app1PkgName, "category")
		kb, err := args.State.KeyBuilder(sys.Storage_Record, q)
		if err != nil {
			return err
		}
		kb.PutRecordID(sys.Storage_Record_Field_ID, istructs.RecordID(args.ArgumentObject.AsInt64("CategoryID"))) // nolint G115
		_, err = args.State.MustExist(kb)
		if err != nil {
			return err
		}
		return callback(&qryCategory{id: args.ArgumentObject.AsInt64("CategoryID")})
	}))

	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "QryDailyIdx"), func(ctx context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
		skbViewDailyIdx, err := args.State.KeyBuilder(sys.Storage_View, QNameApp1_ViewDailyIdx)
		if err != nil {
			return
		}
		if year := args.ArgumentObject.AsInt32(Field_Year); year > 0 {
			skbViewDailyIdx.PutInt32(Field_Year, year)
		}
		if month := args.ArgumentObject.AsInt32(Field_Month); month > 0 {
			skbViewDailyIdx.PutInt32(Field_Month, month)
		}
		if day := args.ArgumentObject.AsInt32(Field_Day); day > 0 {
			skbViewDailyIdx.PutInt32(Field_Day, day)
		}
		return args.State.Read(skbViewDailyIdx, func(key istructs.IKey, value istructs.IStateValue) (err error) {
			return callback(&qryDailyIdxResult{
				year:        key.AsInt32(Field_Year),
				month:       key.AsInt32(Field_Month),
				day:         key.AsInt32(Field_Day),
				stringValue: value.AsString(Field_StringValue),
			})
		})
	}))

	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "QryVoid"), istructsmem.NullQueryExec))
	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "CmdVoid"), istructsmem.NullCommandExec))

	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "CmdODocWithBLOB"), istructsmem.NullCommandExec))

	cfg.Resources.Add(istructsmem.NewCommandFunction(appdef.NewQName(app1PkgName, "CmdAllowedToAnonymousOnly"), istructsmem.NullCommandExec))
	cfg.Resources.Add(istructsmem.NewQueryFunction(appdef.NewQName(app1PkgName, "QryAllowedToAnonymousOnly"), istructsmem.NullQueryExec))

	ttlStorageResultQName := appdef.NewQName(app1PkgName, "TTLStorageResult")
	cfg.Resources.Add(istructsmem.NewCommandFunction(
		appdef.NewQName(app1PkgName, "TTLStorageCmd"),
		func(args istructs.ExecCommandArgs) (err error) {
			ttlStorage := args.State.AppStructs().AppTTLStorage()
			operation := args.ArgumentObject.AsString("Operation")
			key := args.ArgumentObject.AsString("Key")
			value := args.ArgumentObject.AsString("Value")
			expectedValue := args.ArgumentObject.AsString("ExpectedValue")
			ttlSeconds := int(args.ArgumentObject.AsInt32("TTLSeconds"))

			var ok bool
			switch operation {
			case "Put":
				ok, err = ttlStorage.InsertIfNotExists(key, value, ttlSeconds)
			case "CompareAndSwap":
				ok, err = ttlStorage.CompareAndSwap(key, expectedValue, value, ttlSeconds)
			case "CompareAndDelete":
				ok, err = ttlStorage.CompareAndDelete(key, expectedValue)
			default:
				return coreutils.NewHTTPErrorf(http.StatusBadRequest, "unknown operation: ", operation)
			}
			if err != nil {
				if errors.Is(err, storage.ErrAppTTLValidation) {
					return coreutils.WrapSysError(err, http.StatusBadRequest)
				}
				return err
			}

			resultKey, err := args.State.KeyBuilder(sys.Storage_Result, ttlStorageResultQName)
			if err != nil {
				return err
			}
			resultValue, err := args.Intents.NewValue(resultKey)
			if err != nil {
				return err
			}
			resultValue.PutBool("Ok", ok)
			return nil
		},
	))

	ttlGetResultQName := appdef.NewQName(app1PkgName, "TTLGetResult")
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		appdef.NewQName(app1PkgName, "TTLGetQry"),
		func(_ context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
			ttlStorage := args.State.AppStructs().AppTTLStorage()
			key := args.ArgumentObject.AsString("Key")

			value, exists, err := ttlStorage.TTLGet(key)
			if err != nil {
				if errors.Is(err, storage.ErrAppTTLValidation) {
					return coreutils.WrapSysError(err, http.StatusBadRequest)
				}
				return err
			}

			return callback(&ttlGetResult{value: value, exists: exists, qname: ttlGetResultQName})
		},
	))

	app1PackageFS := parser.PackageFS{
		Path: App1PkgPath,
		FS:   SchemaTestApp1FS,
	}
	return builtinapps.Def{
		AppQName:                istructs.AppQName_test1_app1,
		Packages:                []parser.PackageFS{sysPackageFS, app1PackageFS},
		AppDeploymentDescriptor: TestAppDeploymentDescriptor,
	}
}

type qryCategory struct {
	istructs.NullObject
	id int64
}

func (q *qryCategory) AsInt64(name appdef.FieldName) int64 {
	return q.id
}

func (q *qryCategory) AsRecordID(name appdef.FieldName) istructs.RecordID {
	return istructs.RecordID(q.id) // nolint G115
}

type qryDailyIdxResult struct {
	istructs.IObject
	year        int32
	month       int32
	day         int32
	stringValue string
}

func (r qryDailyIdxResult) AsInt32(name appdef.FieldName) int32 {
	switch name {
	case Field_Year:
		return r.year
	case Field_Month:
		return r.month
	case Field_Day:
		return r.day
	default:
		return 0
	}
}
func (r qryDailyIdxResult) AsString(name appdef.FieldName) string {
	switch name {
	case Field_StringValue:
		return r.stringValue
	default:
		return ""
	}
}

type ttlGetResult struct {
	istructs.NullObject
	value  string
	exists bool
	qname  appdef.QName
}

func (r *ttlGetResult) AsString(name appdef.FieldName) string {
	if name == "Value" {
		return r.value
	}
	return ""
}

func (r *ttlGetResult) AsBool(name appdef.FieldName) bool {
	if name == "Exists" {
		return r.exists
	}
	return false
}

func (r *ttlGetResult) QName() appdef.QName {
	return r.qname
}
