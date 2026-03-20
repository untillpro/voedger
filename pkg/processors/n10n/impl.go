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
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/pipeline"
)

func (p *implIN10NProc) Handle(requestCtx context.Context, args N10NProcArgs) {
	var pipeline pipeline.ISyncPipeline
	switch args.Method {
	case http.MethodPost:
		pipeline = subscribeAndWatchPipeline(requestCtx, p)
	case http.MethodPut:
		pipeline = subscribeExtraPipeline(requestCtx, p)
	case http.MethodDelete:
		pipeline = unsubscribePipeline(requestCtx, p)
	default:
		// notest: excluded by router rule
		panic("unexpected method " + args.Method)
	}
	defer pipeline.Close()
	n10nWP := &n10nWorkpiece{
		host:          args.Host,
		body:          args.Body,
		requestCtx:    requestCtx,
		logCtx:        requestCtx,
		responder:     args.Responder,
		token:         args.Token,
		appQName:      args.AppQName,
		entityFromURL: args.EntityFromURL,
		wsidFromURL:   args.WSID,
		channelID:     in10n.ChannelID(args.ChannelIDFromURL),
	}
	err := pipeline.SendSync(n10nWP)
	if err != nil {
		err = wrapToSysError(err)
		reportError(n10nWP, err)
	}
}

func wrapToSysError(err error) error {
	resultCode := http.StatusBadRequest
	switch {
	case errors.Is(err, in10n.ErrChannelDoesNotExist):
		resultCode = http.StatusNotFound
	case errors.Is(err, in10n.ErrQuotaExceeded_Subscriptions), errors.Is(err, in10n.ErrQuotaExceeded_SubscriptionsPerSubject),
		errors.Is(err, in10n.ErrQuotaExceeded_Channels), errors.Is(err, in10n.ErrQuotaExceeded_ChannelsPerSubject):
		resultCode = http.StatusTooManyRequests
	}
	return coreutils.WrapSysError(err, resultCode)
}

func reportError(n10nWP *n10nWorkpiece, err error) {
	logger.ErrorCtx(n10nWP.logCtx, "n10n.error", err)
	if n10nWP.responseWriter == nil {
		bus.ReplyErr(n10nWP.responder, err)
		return
	}
	n10nWP.responseWriter.Close(err)
}

func (m *n10nWorkpiece) Release() {}
