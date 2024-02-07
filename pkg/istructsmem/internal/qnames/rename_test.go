/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package qnames

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructsmem/internal/consts"
	"github.com/voedger/voedger/pkg/istructsmem/internal/teststore"
	"github.com/voedger/voedger/pkg/istructsmem/internal/utils"
	"github.com/voedger/voedger/pkg/istructsmem/internal/vers"
)

func TestRenameQName(t *testing.T) {

	require := require.New(t)

	oldQName := appdef.NewQName("test", "old")
	newQName := appdef.NewQName("test", "new")

	storage := teststore.NewStorage()

	t.Run("prepare storage with old QName", func(t *testing.T) {
		versions := vers.New()
		err := versions.Prepare(storage)
		require.NoError(err)

		appDefBuilder := appdef.New()
		_ = appDefBuilder.AddObject(oldQName)
		appDef, err := appDefBuilder.Build()
		require.NoError(err)

		names := New()
		err = names.Prepare(storage, versions, appDef, nil)
		require.NoError(err)
	})

	t.Run("basic usage", func(t *testing.T) {
		err := Rename(storage, oldQName, newQName)
		require.NoError(err)
	})

	t.Run("check result", func(t *testing.T) {
		versions := vers.New()
		err := versions.Prepare(storage)
		require.NoError(err)

		names := New()
		err = names.Prepare(storage, versions, nil, nil)
		require.NoError(err)

		t.Run("check old is deleted", func(t *testing.T) {
			id, err := names.ID(oldQName)
			require.ErrorIs(err, ErrNameNotFound)
			require.Equal(NullQNameID, id)
		})

		t.Run("check new is not null", func(t *testing.T) {
			id, err := names.ID(newQName)
			require.NoError(err)
			require.Greater(id, QNameIDSysLast)
		})
	})
}

func TestRenameQName_Errors(t *testing.T) {

	require := require.New(t)

	oldQName := appdef.NewQName("test", "old")
	newQName := appdef.NewQName("test", "new")
	other := appdef.NewQName("test", "other")

	storage := teststore.NewStorage()

	t.Run("prepare storage with old QName", func(t *testing.T) {
		versions := vers.New()
		err := versions.Prepare(storage)
		require.NoError(err)

		appDefBuilder := appdef.New()
		_ = appDefBuilder.AddObject(oldQName)
		_ = appDefBuilder.AddObject(other)
		appDef, err := appDefBuilder.Build()
		require.NoError(err)

		names := New()
		err = names.Prepare(storage, versions, appDef, nil)
		require.NoError(err)
	})

	t.Run("must error if old and new are equals", func(t *testing.T) {
		err := Rename(storage, oldQName, oldQName)
		require.ErrorContains(err, "equals")
	})

	t.Run("must error if twice rename", func(t *testing.T) {
		err := Rename(storage, oldQName, newQName)
		require.NoError(err)

		err = Rename(storage, oldQName, newQName)
		require.ErrorIs(err, ErrNameNotFound)

		t.Run("but must ok reverse rename", func(t *testing.T) {
			err = Rename(storage, newQName, oldQName)
			require.NoError(err)
		})
	})

	t.Run("must error if old name not found", func(t *testing.T) {
		err := Rename(storage, appdef.NewQName("test", "unknown"), newQName)
		require.ErrorIs(err, ErrNameNotFound)
	})

	t.Run("must error if new name is already exists", func(t *testing.T) {
		err := Rename(storage, oldQName, other)
		require.ErrorContains(err, "exists")
	})
}

func TestRenameQName_Fails(t *testing.T) {

	require := require.New(t)

	oldQName := appdef.NewQName("test", "old")
	newQName := appdef.NewQName("test", "new")

	t.Run("must error if unsupported version of Versions system view", func(t *testing.T) {
		testError := errors.New("error read versions")
		storage := teststore.NewStorage()

		versions := vers.New()
		err := versions.Prepare(storage)
		require.NoError(err)
		versions.Put(vers.SysQNamesVersion, latestVersion+1) // future version

		storage.ScheduleGetError(testError, utils.ToBytes(consts.SysView_Versions), nil)

		err = Rename(storage, oldQName, newQName)
		require.ErrorIs(err, testError)
	})

	t.Run("must error if unsupported version of QNames system view", func(t *testing.T) {
		storage := teststore.NewStorage()

		versions := vers.New()
		err := versions.Prepare(storage)
		require.NoError(err)
		versions.Put(vers.SysQNamesVersion, latestVersion+1) // future version

		err = Rename(storage, oldQName, newQName)
		require.ErrorIs(err, vers.ErrorInvalidVersion)
	})

	storage := teststore.NewStorage()

	t.Run("prepare storage with old QName", func(t *testing.T) {
		versions := vers.New()
		err := versions.Prepare(storage)
		require.NoError(err)

		appDefBuilder := appdef.New()
		_ = appDefBuilder.AddObject(oldQName)
		appDef, err := appDefBuilder.Build()
		require.NoError(err)

		names := New()
		err = names.Prepare(storage, versions, appDef, nil)
		require.NoError(err)
	})

	t.Run("must error if storage read failed", func(t *testing.T) {
		testError := errors.New("can not read old qname")

		storage.ScheduleGetError(testError, nil, []byte(oldQName.String()))

		err := Rename(storage, oldQName, newQName)
		require.ErrorIs(err, testError)
	})

	t.Run("must error if storage put failed", func(t *testing.T) {
		testError := errors.New("can not delete old qname")

		storage.SchedulePutError(testError, nil, []byte(newQName.String()))

		err := Rename(storage, oldQName, newQName)
		require.ErrorIs(err, testError)
	})
}
