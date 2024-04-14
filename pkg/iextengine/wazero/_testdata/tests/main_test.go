package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	test "github.com/voedger/voedger/pkg/exttinygo/exttinygotests"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/state/teststate"
)

const testPkg = "github.com/org/app/packages/mypkg"
const testWSID = istructs.WSID(1)

func Test_ActualizerStorages(t *testing.T) {
	// Construct test context
	test := test.NewTestAPI(
		teststate.ProcKind_Actualizer,
		testPkg,
		teststate.TestWorkspace{WorkspaceDescriptor: "TestWorkspaceDescriptor", WSID: testWSID})

	test.PutSecret("smtpPassword", []byte("GOD"))

	offs1, _ := test.PutEvent(testWSID, appdef.NewFullQName(testPkg, "dummyCmd"), func(_ istructs.IObjectBuilder, _ istructs.ICUD) {})
	offs2, _ := test.PutEvent(testWSID, appdef.NewFullQName(testPkg, "dummyCmd"), func(_ istructs.IObjectBuilder, _ istructs.ICUD) {})
	require.Equal(t, istructs.Offset(1), offs1)
	require.Equal(t, istructs.Offset(2), offs2)

	test.PutEvent(testWSID, appdef.NewFullQName(testPkg, "CmdToTestWlogStorage"), func(arg istructs.IObjectBuilder, _ istructs.ICUD) {
		arg.PutInt64("Offset", int64(offs1))
		arg.PutInt64("Count", int64(2))
	})

	// Call the extension
	ProjectorToTestWlogStorage()
	test.RequireIntent(t, state.View, appdef.NewFullQName(testPkg, "Results"), func(_ istructs.IStateKeyBuilder) {}).Equal(func(value istructs.IStateValueBuilder) {
		value.PutInt32("IntVal", 2)
		value.PutQName("QNameVal", appdef.NewQName(teststate.TestPkgAlias, "dummyCmd"))
	})

	// Call the extension
	ProjectorToTestSendMailAndSecretsStorage()
	test.RequireIntent(t, state.SendMail, appdef.NullFullQName, func(email istructs.IStateKeyBuilder) {
		email.PutString("Host", "smtp.gmail.com")
		email.PutInt32("Port", 587)
		email.PutString("From", "no-reply@gmail.com")
		email.PutString("To", "email@gmail.com")
		email.PutString("Subject", "Test")
		email.PutString("Body", "TheBody")
		email.PutString("Username", "User")
		email.PutString("Password", "GOD")
	}).Exists()
}

func Test_CommandStorages(t *testing.T) {
	test := test.NewTestAPI(
		teststate.ProcKind_CommandProcessor,
		testPkg,
		teststate.TestWorkspace{WorkspaceDescriptor: "TestWorkspaceDescriptor", WSID: testWSID})

	// Create a Doc1 record
	_, newIds := test.PutEvent(testWSID, appdef.NewFullQName(testPkg, "dummyCmd"), func(_ istructs.IObjectBuilder, cud istructs.ICUD) {
		c := cud.Create(appdef.NewQName(teststate.TestPkgAlias, "Doc1"))
		c.PutRecordID(appdef.SystemField_ID, 1)
		c.PutInt32("Value", 42)
	})
	require.Len(t, newIds, 1)

	test.PutEvent(testWSID, appdef.NewFullQName(testPkg, "CmdToTestRecordStorage"), func(arg istructs.IObjectBuilder, _ istructs.ICUD) {
		arg.PutInt64("IdToRead", int64(newIds[0]))
	})
	CmdToTestRecordStorage()

	test.RequireIntent(t, state.Result, appdef.NullFullQName, func(_ istructs.IStateKeyBuilder) {}).Equal(func(value istructs.IStateValueBuilder) {
		value.PutInt32("ReadValue", 42)
	})

	test.RequireIntent(t, state.Record, appdef.NewFullQName(testPkg, "Doc1"), func(_ istructs.IStateKeyBuilder) {}).Equal(func(value istructs.IStateValueBuilder) {
		value.PutInt32("Value", 43)
	})
}
