/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package builtin

import (
	"context"
	"runtime"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	istructsmem "github.com/voedger/voedger/pkg/istructsmem"
)

type grcountRR struct {
	istructs.NullObject
}

func (e *grcountRR) AsInt32(string) int32 { return int32(runtime.NumGoroutine()) }

func provideQryGRCount(cfg *istructsmem.AppConfigType) {
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		appdef.NewQName(appdef.SysPackage, "GRCount"),
		func(_ context.Context, _ istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
			return callback(&grcountRR{})
		},
	))
}
