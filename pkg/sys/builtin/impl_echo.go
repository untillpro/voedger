/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package builtin

import (
	"context"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
)

func (e *echoRR) AsString(string) string { return e.text }

func provideQryEcho(cfg *istructsmem.AppConfigType) {
	cfg.Resources.Add(istructsmem.NewQueryFunction(
		appdef.NewQName(appdef.SysPackage, "Echo"),
		func(_ context.Context, args istructs.ExecQueryArgs, callback istructs.ExecQueryCallback) (err error) {
			text := args.ArgumentObject.AsString("Text")
			return callback(&echoRR{text: text})
		},
	))
}
