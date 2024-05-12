/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package state

import (
	"context"

	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
)

func implProvideCommandProcessorState(ctx context.Context, appStructsFunc AppStructsFunc, partitionIDFunc PartitionIDFunc,
	wsidFunc WSIDFunc, secretReader isecrets.ISecretReader, cudFunc CUDFunc, principalsFunc PrincipalsFunc,
	tokenFunc TokenFunc, intentsLimit int, cmdResultBuilderFunc CmdResultBuilderFunc, argFunc ArgFunc, unloggedArgFunc UnloggedArgFunc,
	wlogOffsetFunc WLogOffsetFunc) IHostState {
	bs := newHostState("CommandProcessor", intentsLimit, appStructsFunc)

	bs.addStorage(View, newViewRecordsStorage(ctx, appStructsFunc, wsidFunc, nil), S_GET|S_GET_BATCH)
	bs.addStorage(Record, newRecordsStorage(appStructsFunc, wsidFunc, cudFunc), S_GET|S_GET_BATCH|S_INSERT|S_UPDATE)

	bs.addStorage(WLog, &wLogStorage{
		ctx:        ctx,
		eventsFunc: func() istructs.IEvents { return appStructsFunc().Events() },
		wsidFunc:   wsidFunc,
	}, S_GET)

	bs.addStorage(AppSecret, &appSecretsStorage{secretReader: secretReader}, S_GET)

	bs.addStorage(RequestSubject, &subjectStorage{
		principalsFunc: principalsFunc,
		tokenFunc:      tokenFunc,
	}, S_GET)

	bs.addStorage(Result, newCmdResultStorage(cmdResultBuilderFunc), S_INSERT)

	bs.addStorage(Response, &cmdResponseStorage{}, S_INSERT)

	bs.addStorage(CommandContext, &commandContextStorage{
		argFunc:         argFunc,
		unloggedArgFunc: unloggedArgFunc,
		wsidFunc:        wsidFunc,
		wlogOffsetFunc:  wlogOffsetFunc,
	}, S_GET)

	return bs
}
