/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

// nolint
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
)

func (asp *implIAppStorageProvider) AppStorage(appQName appdef.AppQName) (storage istorage.IAppStorage, err error) {
	asp.lock.Lock()
	defer asp.lock.Unlock()

	if asp.isStopping {
		return nil, ErrStoppingState
	}

	if storage, ok := asp.cache[appQName]; ok {
		return storage, nil
	}
	if asp.metaStorage == nil {
		if asp.metaStorage, err = asp.getMetaStorage(); err != nil {
			return nil, err
		}
	}

	exists, appStorageDesc, err := readAppStorageDesc(appQName, asp.metaStorage)
	if err != nil {
		return nil, err
	}
	if !exists {
		if appStorageDesc, err = getNewAppStorageDesc(appQName, asp.metaStorage); err != nil {
			return nil, err
		}
	}

	if len(appStorageDesc.Error) == 0 && appStorageDesc.Status == istorage.AppStorageStatus_Pending {
		if err := asp.asf.Init(asp.clarifyKeyspaceName(appStorageDesc.SafeName)); err != nil {
			appStorageDesc.Error = err.Error()
		} else {
			appStorageDesc.Status = istorage.AppStorageStatus_Done
		}
		// possible: new SafeAppName written , but appDesc write is failed. No problem in this case because we'll just have an orphaned record
		if err = storeAppDesc(appQName, appStorageDesc, asp.metaStorage); err != nil {
			return nil, err
		}
	}
	if len(appStorageDesc.Error) > 0 {
		return nil, fmt.Errorf("%s: %w: %s", appStorageDesc.SafeName.String(), ErrStorageInitError, appStorageDesc.Error)
	}
	storage, err = asp.asf.AppStorage(asp.clarifyKeyspaceName(appStorageDesc.SafeName))
	if err == nil {
		asp.cache[appQName] = storage
	}
	return storage, err
}

func (asp *implIAppStorageProvider) getMetaStorage() (istorage.IAppStorage, error) {
	if err := asp.asf.Init(asp.clarifyKeyspaceName(istorage.SysMetaSafeName)); err != nil && err != istorage.ErrStorageAlreadyExists {
		return nil, err
	}
	return asp.asf.AppStorage(asp.clarifyKeyspaceName(istorage.SysMetaSafeName))
}

func (asp *implIAppStorageProvider) clarifyKeyspaceName(sn istorage.SafeAppName) istorage.SafeAppName {
	if coreutils.IsTest() {
		// unique safe keyspace name is generated at istorage.NewSafeAppName()
		// uuid suffix is need in tests only avoiding the case:
		// - go test ./... in github using Scylla
		// - integration tests for different packages are run in simultaneously in separate processes
		// - 2 processes using the same shared VIT config -> 2 VITs are initialized on the same keyspaces names -> conflict when e.g. creating the same logins
		// see also getNewAppStorageDesc() below
		newName := sn.String() + strings.ToLower(asp.suffix)
		newName = strings.ReplaceAll(newName, "-", "")
		if len(newName) > istorage.MaxSafeNameLength {
			newName = newName[:istorage.MaxSafeNameLength]
		}
		sn = istorage.NewTestSafeName(newName)
	}
	return sn
}

func (asp *implIAppStorageProvider) Prepare(_ any) error { return nil }

func (asp *implIAppStorageProvider) Run(_ context.Context) {}

func (asp *implIAppStorageProvider) Stop() {
	asp.lock.Lock()
	defer asp.lock.Unlock()
	asp.isStopping = true
	asp.asf.StopGoroutines()
}

func storeAppDesc(appQName appdef.AppQName, appDesc istorage.AppStorageDesc, metaStorage istorage.IAppStorage) error {
	pkBytes := []byte(appQName.String())
	cColsBytes := cCols_AppStorageDesc
	appDescJSON, err := json.Marshal(&appDesc)
	if err != nil {
		// notest
		return err
	}
	return metaStorage.Put(pkBytes, cColsBytes, appDescJSON)
}

func getNewAppStorageDesc(appQName appdef.AppQName, metaStorage istorage.IAppStorage) (res istorage.AppStorageDesc, err error) {
	san, err := istorage.NewSafeAppName(appQName, func(name string) (bool, error) {
		pkBytes := []byte(name)
		exists, err := metaStorage.Get(pkBytes, cCols_SafeAppName, &value_SafeAppName)
		if err != nil {
			return false, err
		}
		return !exists, nil
	})
	if err != nil {
		return res, err
	}
	// store new SafeAppName
	pkBytes := []byte(san.String())
	if err := metaStorage.Put(pkBytes, cCols_SafeAppName, value_SafeAppName); err != nil {
		return res, err
	}
	return istorage.AppStorageDesc{
		SafeName: san,
		Status:   istorage.AppStorageStatus_Pending,
	}, nil
}

func readAppStorageDesc(appQName appdef.AppQName, metaStorage istorage.IAppStorage) (ok bool, appStorageDesc istorage.AppStorageDesc, err error) {
	pkBytes := []byte(appQName.String())
	appDescJSON := []byte{}
	if ok, err = metaStorage.Get(pkBytes, cCols_AppStorageDesc, &appDescJSON); err != nil {
		return
	}
	if ok {
		err = json.Unmarshal(appDescJSON, &appStorageDesc)
	}
	return
}
