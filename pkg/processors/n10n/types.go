/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package n10n

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/pipeline"
)

type IN10NProc interface {
	Handle(requestCtx context.Context, args N10NProcArgs)
}

type implIN10NProc struct {
	n10nBroker         in10n.IN10nBroker
	authenticator      iauthnz.IAuthenticator
	appTokensFactory   payloads.IAppTokensFactory
	goroutinesWG       sync.WaitGroup
	appStructsProvider istructs.IAppStructsProvider
}

type n10nWorkpiece struct {
	host                     string
	body                     []byte
	requestCtx               context.Context
	logCtx                   context.Context
	responder                bus.IResponder
	channelID                in10n.ChannelID
	channelCleanup           func()
	subscriptions            []subscription
	expiresIn                time.Duration
	subscribedProjectionKeys []in10n.ProjectionKey
	responseWriter           bus.IResponseWriter
	token                    string
	subjectLogin             istructs.SubjectLogin
	appQName                 appdef.AppQName
	principalPayload         payloads.PrincipalPayload
	entityFromURL            appdef.QName
	wsidFromURL              istructs.WSID
	appStructs               istructs.IAppStructs
	appTokens                istructs.IAppTokens
}

type n10nArgs struct {
	Subscriptions    []subscriptionJSON `json:"subscriptions"`
	ExpiresInSeconds int64              `json:"expiresIn"`
}

type subscriptionJSON struct {
	Entity     string      `json:"entity"`
	WSIDNumber json.Number `json:"wsid"`
}

type subscription struct {
	entity appdef.QName
	wsid   istructs.WSID
}

type N10NProcArgs struct {
	Host             string
	Body             []byte
	Token            string
	Method           string
	EntityFromURL    appdef.QName
	WSID             istructs.WSID
	Responder        bus.IResponder
	AppQName         appdef.AppQName
	ChannelIDFromURL string
}

type channelCleanupOnErr struct {
	pipeline.NOOP
	n10nBroker in10n.IN10nBroker
}
