/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package n10n

import (
	"context"
	"net/http"

	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/pipeline"
)

func unsubscribePipeline(requestCtx context.Context, p *implIN10NProc) pipeline.ISyncPipeline {
	return pipeline.NewSyncPipeline(requestCtx, "Unsubscribe Processor",
		pipeline.WireFunc("validateToken", p.validateToken),
		pipeline.WireFunc("denyBody", denyBody),
		pipeline.WireFunc("unsubscribe", p.unsubscribe),
		pipeline.WireFunc("logUnsubscribeSuccess", logUnsubscribeSuccess),
		pipeline.WireFunc("reply204NoContent", reply204NoContent),
	)
}

func (p *implIN10NProc) unsubscribe(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	projectionKey := in10n.ProjectionKey{
		App:        n10nWP.appQName,
		Projection: n10nWP.entityFromURL,
		WS:         n10nWP.wsidFromURL,
	}
	n10nWP.logCtx = logger.WithContextAttrs(n10nWP.logCtx, map[string]any{
		logAttr_ProjectionKey: in10n.ProjectionKeysToJSON([]in10n.ProjectionKey{projectionKey}),
	})
	return p.n10nBroker.Unsubscribe(n10nWP.channelID, projectionKey)
}

func logUnsubscribeSuccess(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	logger.VerboseCtx(n10nWP.logCtx, "n10n.unsubscribe.success")
	return nil
}

func reply204NoContent(ctx context.Context, n10nWP *n10nWorkpiece) (err error) {
	return n10nWP.responder.Respond(bus.ResponseMeta{StatusCode: http.StatusNoContent}, nil)
}
