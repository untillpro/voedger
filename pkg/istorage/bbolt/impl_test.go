/*
 * Copyright (c) 2022-present Sigma-Soft, Ltd.
 * @author: Dmitry Molchanovsky
 * @author: Maxim Geraskin (refactoring)
 */

package bbolt

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
	istorageimpl "github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istructs"
)

func TestBasicUsage(t *testing.T) {
	params := prepareTestData()
	defer cleanupTestData(params)

	factory := Provide(params, coreutils.MockTime)
	istorage.TechnologyCompatibilityKit(t, factory)
}

func Test_MyTestBasicUsage(t *testing.T) {
	require := require.New(t)

	params := prepareTestData()
	defer cleanupTestData(params)

	// creating a StorageProvider
	factory := Provide(params, coreutils.MockTime)
	storageProvider := istorageimpl.Provide(factory)

	// get the required AppStorage for the app
	appStorage, err := storageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)

	// write the application data to the database
	err = appStorage.Put([]byte("pKey"), []byte("cCols"), []byte("test data string"))
	require.NoError(err)

	// read the data from the database
	value := make([]byte, 0)
	ok, err := appStorage.Get([]byte("pKey"), []byte("cCols"), &value)
	require.True(ok)
	require.NoError(err)
	require.Equal([]byte("test data string"), value)
}

func prepareTestData() (params ParamsType) {
	dbDir, err := os.MkdirTemp("", "bolt")
	if err != nil {
		panic(err)
	}
	params.DBDir = dbDir
	return
}

func cleanupTestData(params ParamsType) {
	if params.DBDir != "" {
		os.RemoveAll(params.DBDir)
		params.DBDir = ""
	}
}

func Test_PutGet(t *testing.T) {
	require := require.New(t)

	params := prepareTestData()
	defer cleanupTestData(params)

	factory := Provide(params, coreutils.MockTime)
	storageProvider := istorageimpl.Provide(factory)

	appStorage, err := storageProvider.AppStorage(istructs.AppQName_test1_app1)
	require.NoError(err)

	err = appStorage.Put([]byte("persons"), []byte("NNV"), []byte("Nikitin Nikolay Valeryevich"))
	require.NoError(err)

	err = appStorage.Put([]byte("persons"), []byte("MDA"), []byte("Molchanovsky Dmitry Anatolyevich"))
	require.NoError(err)

	value := make([]byte, 0)

	ok, err := appStorage.Get([]byte("persons"), []byte("NNV"), &value)
	require.NoError(err)
	require.True(ok)
	require.Equal("Nikitin Nikolay Valeryevich", string(value))

	ok, err = appStorage.Get([]byte("persons"), []byte("MDA"), &value)
	require.NoError(err)
	require.True(ok)
	require.Equal("Molchanovsky Dmitry Anatolyevich", string(value))
}
