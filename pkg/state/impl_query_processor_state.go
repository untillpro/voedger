/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package state

import (
	"context"

	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/utils/federation"
)

type queryProcessorState struct {
	*hostState
	queryArgs     PrepareArgsFunc
	queryCallback ExecQueryCallbackFunc
}

func (s queryProcessorState) QueryPrepareArgs() istructs.PrepareArgs {
	return s.queryArgs()
}

func (s queryProcessorState) QueryCallback() istructs.ExecQueryCallback {
	return s.queryCallback()
}

func implProvideQueryProcessorState(
	ctx context.Context,
	appStructsFunc AppStructsFunc,
	partitionIDFunc PartitionIDFunc,
	wsidFunc WSIDFunc,
	secretReader isecrets.ISecretReader,
	principalsFunc PrincipalsFunc,
	tokenFunc TokenFunc,
	itokens itokens.ITokens,
	execQueryArgsFunc PrepareArgsFunc,
	argFunc ArgFunc,
	resultBuilderFunc ObjectBuilderFunc,
	federation federation.IFederation,
	queryCallbackFunc ExecQueryCallbackFunc,
	options ...StateOptFunc) IHostState {

	opts := &stateOpts{}
	for _, optFunc := range options {
		optFunc(opts)
	}

	state := &queryProcessorState{
		hostState:     newHostState("QueryProcessor", queryProcessorStateMaxIntents, appStructsFunc),
		queryArgs:     execQueryArgsFunc,
		queryCallback: queryCallbackFunc,
	}

	state.addStorage(View, newViewRecordsStorage(ctx, appStructsFunc, wsidFunc, nil), S_GET|S_GET_BATCH|S_READ)
	state.addStorage(Record, newRecordsStorage(appStructsFunc, wsidFunc, nil), S_GET|S_GET_BATCH)

	state.addStorage(WLog, &wLogStorage{
		ctx:        ctx,
		eventsFunc: func() istructs.IEvents { return appStructsFunc().Events() },
		wsidFunc:   wsidFunc,
	}, S_GET|S_READ)

	state.addStorage(Http, &httpStorage{
		customClient: opts.customHttpClient,
	}, S_READ)

	state.addStorage(FederationCommand, &federationCommandStorage{
		appStructs: appStructsFunc,
		wsid:       wsidFunc,
		emulation:  opts.federationCommandHandler,
		federation: federation,
		tokens:     itokens,
	}, S_GET)

	state.addStorage(AppSecret, &appSecretsStorage{secretReader: secretReader}, S_GET)

	state.addStorage(RequestSubject, &subjectStorage{
		principalsFunc: principalsFunc,
		tokenFunc:      tokenFunc,
	}, S_GET)

	state.addStorage(QueryContext, &queryContextStorage{
		argFunc:  argFunc,
		wsidFunc: wsidFunc,
	}, S_GET)

	state.addStorage(Response, &cmdResponseStorage{}, S_INSERT)

	state.addStorage(Result, newQueryResultStorage(appStructsFunc, resultBuilderFunc, queryCallbackFunc), S_INSERT)

	state.addStorage(Uniq, newUniquesStorage(appStructsFunc, wsidFunc, opts.uniquesHandler), S_GET)

	return state
}
