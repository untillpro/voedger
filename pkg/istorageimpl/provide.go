/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package istorageimpl

import (
	"github.com/google/uuid"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
)

// keyspaceNameSuffix is used in tests only
// see https://dev.untill.com/projects/#!638565
func Provide(asf istorage.IAppStorageFactory, keyspaceNameSuffix ...string) istorage.IAppStorageProvider {
	res := &implIAppStorageProvider{
		asf:   asf,
		cache: map[istructs.AppQName]istorage.IAppStorage{},
	}
	if len(keyspaceNameSuffix) > 0 {
		res.suffix = keyspaceNameSuffix[0]
	} else {
		res.suffix = uuid.NewString()
	}
	return res
}
