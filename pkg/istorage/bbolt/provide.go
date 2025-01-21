/*
 * Copyright (c) 2022-present Sigma-Soft, Ltd.
 * @author: Dmitry Molchanovsky
 */

package bbolt

import (
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/istorage"
)

func Provide(params ParamsType, iTime coreutils.ITime) istorage.IAppStorageFactory {
	return &appStorageFactory{
		bboltParams: params,
		iTime:       iTime,
	}
}
