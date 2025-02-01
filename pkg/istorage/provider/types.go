/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package provider

import (
	"sync"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istorage"
)

type implIAppStorageProvider struct {
	cache       map[appdef.AppQName]istorage.IAppStorage
	asf         istorage.IAppStorageFactory
	lock        sync.Mutex
	metaStorage istorage.IAppStorage
	suffix      string // used in tests only (ITs in 2 packages are run simultaneously on the same config -> e.g. on CreateLogin: login exists already)
	isStopping  bool
}
