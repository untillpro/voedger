/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package state

import (
	"context"

	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
)

type commandProcessorState struct {
	*hostState
	commandPrepareArgs CommandPrepareArgsFunc
}

func (s commandProcessorState) CommandPrepareArgs() istructs.CommandPrepareArgs {
	return s.commandPrepareArgs()
}

func implProvideCommandProcessorState(
	ctx context.Context,
	appStructsFunc AppStructsFunc,
	partitionIDFunc PartitionIDFunc,
	wsidFunc WSIDFunc,
	secretReader isecrets.ISecretReader,
	cudFunc CUDFunc,
	principalsFunc PrincipalsFunc,
	tokenFunc TokenFunc,
	intentsLimit int,
	cmdResultBuilderFunc ObjectBuilderFunc,
	execCmdArgsFunc CommandPrepareArgsFunc,
	argFunc ArgFunc,
	unloggedArgFunc UnloggedArgFunc,
	wlogOffsetFunc WLogOffsetFunc,
	options ...StateOptFunc) IHostState {

	opts := &stateOpts{}
	for _, optFunc := range options {
		optFunc(opts)
	}

	state := &commandProcessorState{
		hostState:          newHostState("CommandProcessor", intentsLimit, appStructsFunc),
		commandPrepareArgs: execCmdArgsFunc,
	}

	state.addStorage(View, newViewRecordsStorage(ctx, appStructsFunc, wsidFunc, nil), S_GET|S_GET_BATCH)
	state.addStorage(Record, newRecordsStorage(appStructsFunc, wsidFunc, cudFunc), S_GET|S_GET_BATCH|S_INSERT|S_UPDATE)

	state.addStorage(WLog, &wLogStorage{
		ctx:        ctx,
		eventsFunc: func() istructs.IEvents { return appStructsFunc().Events() },
		wsidFunc:   wsidFunc,
	}, S_GET)

	state.addStorage(AppSecret, &appSecretsStorage{secretReader: secretReader}, S_GET)

	state.addStorage(RequestSubject, &subjectStorage{
		principalsFunc: principalsFunc,
		tokenFunc:      tokenFunc,
	}, S_GET)

	state.addStorage(Result, newCmdResultStorage(cmdResultBuilderFunc), S_INSERT)

	state.addStorage(Uniq, newUniquesStorage(appStructsFunc, wsidFunc, opts.uniquesHandler), S_GET)

	state.addStorage(Response, &cmdResponseStorage{}, S_INSERT)

	state.addStorage(CommandContext, &commandContextStorage{
		argFunc:         argFunc,
		unloggedArgFunc: unloggedArgFunc,
		wsidFunc:        wsidFunc,
		wlogOffsetFunc:  wlogOffsetFunc,
	}, S_GET)

	return state
}
