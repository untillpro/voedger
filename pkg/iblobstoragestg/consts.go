/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 */

package iblobstoragestg

import "github.com/voedger/voedger/pkg/iblobstorage"

const (
	chunkSize  uint64 = 102400
	zeroCCol   uint64 = 0
	zeroBucket uint64 = 0
	bucketSize uint64 = 100
	keyLength  byte   = 28
	uint64Size        = 8
)

var RLimiter_Null iblobstorage.RLimiterType = func(wantReadBytes uint64) error { return nil }
