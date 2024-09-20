/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
  * @author: Dmitry Molchanovsky
*/

package iratesce

import (
	"github.com/voedger/voedger/pkg/coreutils"
	irates "github.com/voedger/voedger/pkg/irates"
)

// Provide: constructs bucketFactory
func Provide(time coreutils.ITime) (buckets irates.IBuckets) {
	return &bucketsType{
		buckets:       map[irates.BucketKey]*bucketType{},
		defaultStates: map[string]irates.BucketState{},
		time:          time,
	}
}
