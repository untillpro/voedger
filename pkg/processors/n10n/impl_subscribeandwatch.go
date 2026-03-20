/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package n10n

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appdef/acl"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/sys/authnz"
)

func subscribeAndWatchPipeline(requestCtx context.Context, p *implIN10NProc) pipeline.ISyncPipeline {
	return pipeline.NewSyncPipeline(requestCtx, "Subscribe and Watch Processor",
		pipeline.WireFunc("validateToken", p.validateToken),
		pipeline.WireFunc("getSubjectLogin", p.getSubjectLogin),
		pipeline.WireFunc("getAppStructs", p.getAppStructs),
		pipeline.WireFunc("parseSubscribeAndWatchArgs", parseSubscribeAndWatchArgs),
		pipeline.WireFunc("authnzEntities", p.authnzEntities),
		pipeline.WireFunc("newChannel", p.newChannel),
		pipeline.WireFunc("subscribe", p.subscribe),
		pipeline.WireFunc("logSubscribeAndWatchSuccess", logSubscribeAndWatchSuccess),
		pipeline.WireFunc("initResponse", initResponse),
		pipeline.WireFunc("sendChannelIDSSEEvent", sendChannelIDSSEEvent),
		pipeline.WireSyncOperator("channelCleanupOnErr", &channelCleanupOnErr{n10nBroker: p.n10nBroker}),
		pipeline.WireFunc("go WatchChannel", p.watchChannel),
	)
}

func (p *implIN10NProc) validateToken(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	n10nWP.appTokens = p.appTokensFactory.New(n10nWP.appQName)
	_, err = n10nWP.appTokens.ValidateToken(n10nWP.token, &n10nWP.principalPayload)

	// [~server.n10n/err.routerCreateChannelInvalidToken~impl]
	// [~server.n10n/err.routerAddSubscriptionInvalidToken~impl]
	// [~server.n10n/err.routerUnsubscribeInvalidToken~impl]
	return coreutils.WrapSysError(err, http.StatusUnauthorized)
}

func (p *implIN10NProc) getSubjectLogin(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	n10nWP.subjectLogin = istructs.SubjectLogin(n10nWP.principalPayload.Login)
	return nil
}

func parseSubscribeAndWatchArgs(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	subscribeAndWatchArgs := n10nArgs{}
	if err := coreutils.JSONUnmarshalDisallowUnknownFields(n10nWP.body, &subscribeAndWatchArgs); err != nil {
		return fmt.Errorf("failed to unmarshal request body: %w", err)
	}
	if subscribeAndWatchArgs.ExpiresInSeconds == 0 {
		subscribeAndWatchArgs.ExpiresInSeconds = defaultN10NExpiresInSeconds
	} else if subscribeAndWatchArgs.ExpiresInSeconds < 0 {
		return fmt.Errorf("invalid expiresIn value %d", subscribeAndWatchArgs.ExpiresInSeconds)
	}
	n10nWP.expiresIn = time.Duration(subscribeAndWatchArgs.ExpiresInSeconds) * time.Second
	if len(subscribeAndWatchArgs.Subscriptions) == 0 {
		return errors.New("no subscriptions provided")
	}
	for i, subscr := range subscribeAndWatchArgs.Subscriptions {
		if len(subscr.Entity) == 0 || len(subscr.WSIDNumber.String()) == 0 {
			return fmt.Errorf("subscriptions[%d]: entity and/or wsid is not provided", i)
		}
		wsid, err := coreutils.ClarifyJSONWSID(subscr.WSIDNumber)
		if err != nil {
			return fmt.Errorf("subscriptions[%d]: failed to parse wsid %s: %w", i, subscr.WSIDNumber, err)
		}
		entity, err := appdef.ParseQName(subscr.Entity)
		if err != nil {
			return fmt.Errorf("subscriptions[%d]: failed to parse entity %s as a QName: %w", i, subscr.Entity, err)
		}
		n10nWP.subscriptions = append(n10nWP.subscriptions, subscription{
			entity: entity,
			wsid:   wsid,
		})
	}
	return nil
}

func (p *implIN10NProc) getAppStructs(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	n10nWP.appStructs, err = p.appStructsProvider.BuiltIn(n10nWP.appQName)
	return err
}

func (p *implIN10NProc) authnzEntities(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	for _, s := range n10nWP.subscriptions {
		wsDesc, err := processors.GetWSDesc(s.wsid, n10nWP.appStructs)
		if err != nil {
			return fmt.Errorf("%d: %w", s.wsid, err)
		}
		iWorkspace := n10nWP.appStructs.AppDef().WorkspaceByDescriptor(wsDesc.AsQName(authnz.Field_WSKind))
		authnzReq := iauthnz.AuthnRequest{
			Host:        n10nWP.host,
			RequestWSID: s.wsid,
			Token:       n10nWP.token,
		}
		principals, _, err := p.authenticator.Authenticate(ctx, n10nWP.appStructs, n10nWP.appTokens, authnzReq)
		if err != nil {
			// [~server.n10n/err.routerCreateChannelInvalidToken~impl]
			// [~server.n10n/err.routerAddSubscriptionInvalidToken~impl]
			// [~server.n10n/err.routerUnsubscribeInvalidToken~impl]
			// notest: token is validated already, error could happen on e.g. subjects read failure
			return coreutils.NewHTTPError(http.StatusUnauthorized, err)
		}
		roles := processors.GetRoles(principals)
		ok, err := acl.IsOperationAllowed(iWorkspace, appdef.OperationKind_Select, s.entity, nil, roles)
		if err != nil {
			return err
		}
		if !ok {
			// [~server.n10n/err.routerCreateChannelNoPermissions~impl]
			// [~server.n10n/err.routerAddSubscriptionNoPermissions~impl]
			return coreutils.NewHTTPErrorf(http.StatusForbidden)
		}
	}
	return nil
}

func (p *implIN10NProc) newChannel(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	n10nWP.channelID, n10nWP.channelCleanup, err = p.n10nBroker.NewChannel(n10nWP.subjectLogin, n10nWP.expiresIn)
	if err == nil {
		n10nWP.logCtx = logger.WithContextAttrs(n10nWP.logCtx, map[string]any{
			logAttr_ChannelID: string(n10nWP.channelID),
		})
	}
	return err
}

func initResponse(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	n10nWP.responseWriter = n10nWP.responder.StreamEvents()
	return nil
}

func sendChannelIDSSEEvent(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	return n10nWP.responseWriter.Write(fmt.Sprintf("event: channelId\ndata: %s\n\n", n10nWP.channelID))
}

func (p *implIN10NProc) subscribe(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	for _, sub := range n10nWP.subscriptions {
		projectionKey := in10n.ProjectionKey{
			App:        n10nWP.appQName,
			Projection: sub.entity,
			WS:         sub.wsid,
		}
		if err = p.n10nBroker.Subscribe(n10nWP.channelID, projectionKey); err != nil {
			return fmt.Errorf("subscribe failed: %w", err)
		}
		n10nWP.subscribedProjectionKeys = append(n10nWP.subscribedProjectionKeys, projectionKey)
	}
	if len(n10nWP.subscribedProjectionKeys) > 0 {
		n10nWP.logCtx = logger.WithContextAttrs(n10nWP.logCtx, map[string]any{
			logAttr_ProjectionKey: in10n.ProjectionKeysToJSON(n10nWP.subscribedProjectionKeys),
		})
	}
	return nil
}

func logSubscribeAndWatchSuccess(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	logger.VerboseCtx(n10nWP.logCtx, "n10n.subscribe&watch.success")
	return nil
}

func (p *implIN10NProc) watchChannel(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	p.goroutinesWG.Add(1)
	watchChannelCtx, cancel := context.WithCancel(n10nWP.requestCtx)
	go func() {
		defer p.goroutinesWG.Done()
		defer cancel()
		// unsubscribe and channel cleanup is done within WatchChannel
		p.n10nBroker.WatchChannel(watchChannelCtx, n10nWP.channelID, func(projection in10n.ProjectionKey, offset istructs.Offset) {
			sseMessage := fmt.Sprintf("event: %s\ndata: %d\n\n", projection.ToJSON(), offset)
			if err := n10nWP.responseWriter.Write(sseMessage); err != nil {
				logger.ErrorCtx(n10nWP.logCtx, "n10n.watch.sse_error", err)
				cancel()
			}
			if logger.IsVerbose() {
				logger.VerboseCtx(n10nWP.logCtx, "n10n.sse_sent", strings.ReplaceAll(sseMessage, "\n", " "))
			}
		})
		logger.VerboseCtx(n10nWP.logCtx, "n10n.watch.done")
		n10nWP.channelCleanup()
		n10nWP.responseWriter.Close(nil)
	}()
	return nil
}

func (u *channelCleanupOnErr) OnErr(err error, work interface{}, _ pipeline.IWorkpieceContext) (newErr error) {
	n10nWP := work.(*n10nWorkpiece)
	if n10nWP.channelCleanup != nil {
		n10nWP.channelCleanup()
	}
	return err
}
