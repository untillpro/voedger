/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package n10n

import (
	"context"
	"errors"
	"net/http"

	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/pipeline"
)

func subscribeExtraPipeline(requestCtx context.Context, p *implIN10NProc) pipeline.ISyncPipeline {
	return pipeline.NewSyncPipeline(requestCtx, "Subscribe on Extra View Processor",
		pipeline.WireFunc("validateToken", p.validateToken),
		pipeline.WireFunc("denyBody", denyBody),
		pipeline.WireFunc("getAppStructs", p.getAppStructs),
		pipeline.WireFunc("addProjectionKeyFromURL", addProjectionKeyFromURL),
		pipeline.WireFunc("authnzEntities", p.authnzEntities),
		pipeline.WireFunc("subscribe", p.subscribe),
		pipeline.WireFunc("logSubscribeSuccess", logSubscribeSuccess),
		pipeline.WireFunc("replyOK", p.replyOK),
	)
}

func denyBody(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	if len(n10nWP.body) > 0 {
		return errors.New("unexpected body")
	}
	return nil
}

func addProjectionKeyFromURL(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	n10nWP.subscriptions = append(n10nWP.subscriptions, subscription{
		entity: n10nWP.entityFromURL,
		wsid:   n10nWP.wsidFromURL,
	})
	projKey := in10n.ProjectionKey{
		App:        n10nWP.appQName,
		Projection: n10nWP.entityFromURL,
		WS:         n10nWP.wsidFromURL,
	}
	n10nWP.logCtx = logger.WithContextAttrs(n10nWP.logCtx, map[string]any{
		logAttr_ProjectionKey: in10n.ProjectionKeysToJSON([]in10n.ProjectionKey{projKey}),
	})
	return nil
}

func logSubscribeSuccess(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	logger.VerboseCtx(n10nWP.logCtx, "n10n.subscribe.success")
	return nil
}

func (p *implIN10NProc) replyOK(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	return n10nWP.responder.Respond(bus.ResponseMeta{StatusCode: http.StatusOK}, nil)
}
