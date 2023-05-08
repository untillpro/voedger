/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package containers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorageimpl"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem/internal/consts"
	"github.com/voedger/voedger/pkg/istructsmem/internal/teststore"
	"github.com/voedger/voedger/pkg/istructsmem/internal/utils"
	"github.com/voedger/voedger/pkg/istructsmem/internal/vers"
)

func TestContainers(t *testing.T) {
	require := require.New(t)

	sp := istorageimpl.Provide(istorage.ProvideMem())
	storage, _ := sp.AppStorage(istructs.AppQName_test1_app1)

	versions := vers.New()
	if err := versions.Prepare(storage); err != nil {
		panic(err)
	}

	containerName := "test"

	containers := New()
	if err := containers.Prepare(storage, versions,
		func() appdef.IAppDef {
			defName := appdef.NewQName("test", "el")
			appDef := appdef.New()
			appDef.AddStruct(defName, appdef.DefKind_Element).
				AddContainer(containerName, defName, 0, 1)
			result, err := appDef.Build()
			require.NoError(err)
			return result
		}()); err != nil {
		panic(err)
	}

	t.Run("basic Containers methods", func(t *testing.T) {

		check := func(containers *Containers, name string) ContainerID {
			id, err := containers.GetID(name)
			require.NoError(err)
			require.NotEqual(NullContainerID, id)

			n, err := containers.GetContainer(id)
			require.NoError(err)
			require.Equal(name, n)

			return id
		}

		id := check(containers, containerName)

		t.Run("must be ok to load early stored names", func(t *testing.T) {
			versions1 := vers.New()
			if err := versions1.Prepare(storage); err != nil {
				panic(err)
			}

			containers1 := New()
			if err := containers1.Prepare(storage, versions, nil); err != nil {
				panic(err)
			}

			require.Equal(id, check(containers1, containerName))
		})

		t.Run("must be ok to redeclare containers", func(t *testing.T) {
			versions2 := vers.New()
			if err := versions2.Prepare(storage); err != nil {
				panic(err)
			}

			containers2 := New()
			if err := containers2.Prepare(storage, versions,
				func() appdef.IAppDef {
					defName := appdef.NewQName("test", "el")
					appDef := appdef.New()
					appDef.AddStruct(defName, appdef.DefKind_Element).
						AddContainer(containerName, defName, 0, 1)
					result, err := appDef.Build()
					require.NoError(err)
					return result
				}()); err != nil {
				panic(err)
			}

			require.Equal(id, check(containers2, containerName))
		})
	})

	t.Run("must be error if unknown container", func(t *testing.T) {
		id, err := containers.GetID("unknown")
		require.Equal(NullContainerID, id)
		require.ErrorIs(err, ErrContainerNotFound)
	})

	t.Run("must be error if unknown id", func(t *testing.T) {
		n, err := containers.GetContainer(ContainerID(MaxAvailableContainerID))
		require.Equal("", n)
		require.ErrorIs(err, ErrContainerIDNotFound)
	})
}

func TestContainersPrepareErrors(t *testing.T) {
	require := require.New(t)

	t.Run("must be error if unknown system view version", func(t *testing.T) {
		sp := istorageimpl.Provide(istorage.ProvideMem())
		storage, _ := sp.AppStorage(istructs.AppQName_test1_app1)

		versions := vers.New()
		if err := versions.Prepare(storage); err != nil {
			panic(err)
		}

		versions.Put(vers.SysContainersVersion, lastestVersion+1)

		names := New()
		err := names.Prepare(storage, versions, nil)
		require.ErrorIs(err, vers.ErrorInvalidVersion)
	})

	t.Run("must be error if invalid Container readed from system view ", func(t *testing.T) {
		sp := istorageimpl.Provide(istorage.ProvideMem())
		storage, _ := sp.AppStorage(istructs.AppQName_test1_app1)

		versions := vers.New()
		if err := versions.Prepare(storage); err != nil {
			panic(err)
		}

		versions.Put(vers.SysContainersVersion, lastestVersion)
		const badName = "-test-error-name-"
		storage.Put(utils.ToBytes(consts.SysView_Containers, ver01), []byte(badName), utils.ToBytes(ContainerID(512)))

		names := New()
		err := names.Prepare(storage, versions, nil)
		require.ErrorIs(err, appdef.ErrInvalidName)
	})

	t.Run("must be ok if deleted Container readed from system view ", func(t *testing.T) {
		sp := istorageimpl.Provide(istorage.ProvideMem())
		storage, _ := sp.AppStorage(istructs.AppQName_test1_app1)

		versions := vers.New()
		if err := versions.Prepare(storage); err != nil {
			panic(err)
		}

		versions.Put(vers.SysContainersVersion, lastestVersion)
		storage.Put(utils.ToBytes(consts.SysView_Containers, ver01), []byte("deleted"), utils.ToBytes(NullContainerID))

		names := New()
		err := names.Prepare(storage, versions, nil)
		require.NoError(err)
	})

	t.Run("must be error if invalid (small) ContainerID readed from system view ", func(t *testing.T) {
		sp := istorageimpl.Provide(istorage.ProvideMem())
		storage, _ := sp.AppStorage(istructs.AppQName_test1_app1)

		versions := vers.New()
		if err := versions.Prepare(storage); err != nil {
			panic(err)
		}

		versions.Put(vers.SysContainersVersion, lastestVersion)
		storage.Put(utils.ToBytes(consts.SysView_Containers, ver01), []byte("test"), utils.ToBytes(ContainerID(1)))

		names := New()
		err := names.Prepare(storage, versions, nil)
		require.ErrorIs(err, ErrWrongContainerID)
	})

	t.Run("must be error if too many Containers", func(t *testing.T) {
		sp := istorageimpl.Provide(istorage.ProvideMem())
		storage, _ := sp.AppStorage(istructs.AppQName_test1_app1)

		versions := vers.New()
		if err := versions.Prepare(storage); err != nil {
			panic(err)
		}

		names := New()
		err := names.Prepare(storage, versions,
			func() appdef.IAppDef {
				appDef := appdef.New()
				qName := appdef.NewQName("test", "test")
				def := appDef.AddStruct(qName, appdef.DefKind_Element)
				for i := 0; i <= MaxAvailableContainerID; i++ {
					def.AddContainer(fmt.Sprintf("cont_%d", i), qName, 0, 1)
				}
				result, err := appDef.Build()
				require.NoError(err)
				return result
			}())
		require.ErrorIs(err, ErrContainerIDsExceeds)
	})

	t.Run("must be error if write to storage failed", func(t *testing.T) {
		containerName := "testContainerName"
		writeError := errors.New("storage write error")

		t.Run("must be error if write some name failed", func(t *testing.T) {
			storage := teststore.NewStorage()

			versions := vers.New()
			if err := versions.Prepare(storage); err != nil {
				panic(err)
			}

			storage.SchedulePutError(writeError, utils.ToBytes(consts.SysView_Containers, ver01), []byte(containerName))

			names := New()
			err := names.Prepare(storage, versions,
				func() appdef.IAppDef {
					defName := appdef.NewQName("test", "el")
					appDef := appdef.New()
					appDef.AddStruct(defName, appdef.DefKind_Element).
						AddContainer(containerName, defName, 0, 1)
					result, err := appDef.Build()
					require.NoError(err)
					return result
				}())
			require.ErrorIs(err, writeError)
		})

		t.Run("must be error if write system view version failed", func(t *testing.T) {
			storage := teststore.NewStorage()

			versions := vers.New()
			if err := versions.Prepare(storage); err != nil {
				panic(err)
			}

			storage.SchedulePutError(writeError, utils.ToBytes(consts.SysView_Versions), utils.ToBytes(vers.SysContainersVersion))

			names := New()
			err := names.Prepare(storage, versions,
				func() appdef.IAppDef {
					defName := appdef.NewQName("test", "el")
					appDef := appdef.New()
					appDef.AddStruct(defName, appdef.DefKind_Element).
						AddContainer(containerName, defName, 0, 1)
					result, err := appDef.Build()
					require.NoError(err)
					return result
				}())
			require.ErrorIs(err, writeError)
		})
	})
}
