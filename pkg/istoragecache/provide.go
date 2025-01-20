/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package istoragecache

import (
	"github.com/voedger/voedger/pkg/coreutils"
	istorage "github.com/voedger/voedger/pkg/istorage"
	imetrics "github.com/voedger/voedger/pkg/metrics"
)

// Provide s.e.
func Provide(
	maxBytes int,
	storageProvider istorage.IAppStorageProvider,
	metrics imetrics.IMetrics,
	vvmName string,
	iTime coreutils.ITime,
) istorage.IAppStorageProvider {
	return &implCachingAppStorageProvider{
		maxBytes:        maxBytes,
		storageProvider: storageProvider,
		metrics:         metrics,
		vvmName:         vvmName,
		iTime:           iTime,
	}
}
