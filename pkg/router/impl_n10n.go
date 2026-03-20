/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/strconvu"

	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/in10nmem"
	"github.com/voedger/voedger/pkg/istructs"
)

/*
curl -G --data-urlencode "payload={\"SubjectLogin\": \"paa\", \"ProjectionKey\":[{\"App\":\"Application\",\"Projection\":\"paa.price\",\"WS\":1}, {\"App\":\"Application\",\"Projection\":\"paa.wine_price\",\"WS\":1}]}" https://alpha2.dev.untill.ru/n10n/channel -H "Content-Type: application/json"
*/

// Why that is duplicated in n10n processor? Because the processor handles apiv2 only.
func (s *routerService) subscribeAndWatchHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		logCtx := withLogAttribs(req.Context(), validatedData{}, bus.Request{Resource: "sys._N10N_SubscribeAndWatch"}, req)
		var (
			urlParams      in10nmem.CreateChannelParamsType
			channel        in10n.ChannelID
			channelCleanup func()
			flusher        http.Flusher
			err            error
		)
		rw.Header().Set("Content-Type", "text/event-stream")
		rw.Header().Set("Cache-Control", "no-cache")
		rw.Header().Set("Connection", "keep-alive")
		jsonParam, ok := req.URL.Query()["payload"]
		if !ok || len(jsonParam[0]) < 1 {
			errMsg := "query parameter with payload (SubjectLogin id and ProjectionKey) is missing"
			logger.ErrorCtx(logCtx, "n10n.error", errMsg)
			WriteTextResponse(rw, errMsg, http.StatusBadRequest)
			return
		}
		if err = json.Unmarshal([]byte(jsonParam[0]), &urlParams); err != nil {
			err = fmt.Errorf("cannot unmarshal input payload %w", err)
			logger.ErrorCtx(logCtx, "n10n.error", err)
			WriteTextResponse(rw, err.Error(), http.StatusBadRequest)
			return
		}
		flusher, ok = rw.(http.Flusher)
		if !ok {
			// notest
			WriteTextResponse(rw, "streaming unsupported!", http.StatusInternalServerError)
			return
		}
		channel, channelCleanup, err = s.n10n.NewChannel(urlParams.SubjectLogin, hours24)
		if err != nil {
			logger.ErrorCtx(logCtx, "n10n.error", err)
			WriteTextResponse(rw, "create new channel failed: "+err.Error(), n10nErrorToStatusCode(err))
			return
		}
		logCtx = logger.WithContextAttrs(logCtx, map[string]any{
			logAttrib_ChannelID: string(channel),
		})
		defer channelCleanup()
		if _, err = fmt.Fprintf(rw, "event: channelId\ndata: %s\n\n", channel); err != nil {
			logger.ErrorCtx(logCtx, "n10n.error", "failed to write created channel id:", err)
			return
		}
		for _, projection := range urlParams.ProjectionKey {
			if err = s.n10n.Subscribe(channel, projection); err != nil {
				logger.ErrorCtx(logCtx, "n10n.error", err)
				WriteTextResponse(rw, "subscribe failed: "+err.Error(), n10nErrorToStatusCode(err))
				return
			}
		}
		if len(urlParams.ProjectionKey) > 0 {
			logCtx = logger.WithContextAttrs(logCtx, map[string]any{
				logAttrib_ProjectionKey: in10n.ProjectionKeysToJSON(urlParams.ProjectionKey),
			})
		}
		flusher.Flush()
		serveN10NChannel(logCtx, rw, flusher, channel, s.n10n)
	}
}

// finishes when logCtx is closed or on SSE message sending failure
func serveN10NChannel(logCtx context.Context, rw http.ResponseWriter, flusher http.Flusher, channel in10n.ChannelID, n10n in10n.IN10nBroker) {
	ch := make(chan in10nmem.UpdateUnit)
	watchChannelCtx, watchChannelCtxCancel := context.WithCancel(logCtx)
	go func() {
		defer close(ch)
		n10n.WatchChannel(watchChannelCtx, channel, func(projection in10n.ProjectionKey, offset istructs.Offset) {
			ch <- in10nmem.UpdateUnit{
				Projection: projection,
				Offset:     offset,
			}
		})
	}()
	logger.VerboseCtx(logCtx, "n10n.subscribe&watch.success")
	for logCtx.Err() == nil {
		result, ok := <-ch
		if !ok {
			break
		}
		sseMessage := fmt.Sprintf("event: %s\ndata: %s\n\n", result.Projection.ToJSON(), strconvu.UintToString(result.Offset))
		if _, err := fmt.Fprint(rw, sseMessage); err != nil {
			logger.ErrorCtx(logCtx, "n10n.watch.sse_error", err)
			break
		}
		flusher.Flush()
		if logger.IsVerbose() {
			logger.VerboseCtx(logCtx, "n10n.sse_sent", strings.ReplaceAll(sseMessage, "\n", " "))
		}
	}
	// graceful client disconnect -> req.Context() closed
	// failed to write sse message -> need to close watchChannelContext
	watchChannelCtxCancel()
	for range ch {
	}
	logger.VerboseCtx(logCtx, "n10n.watch.done")
}

func n10nErrorToStatusCode(err error) int {
	switch {
	case errors.Is(err, in10n.ErrChannelDoesNotExist), errors.Is(err, in10nmem.ErrMetricDoesNotExists):
		return http.StatusBadRequest
	case errors.Is(err, in10n.ErrQuotaExceeded_Subscriptions), errors.Is(err, in10n.ErrQuotaExceeded_SubscriptionsPerSubject),
		errors.Is(err, in10n.ErrQuotaExceeded_Channels), errors.Is(err, in10n.ErrQuotaExceeded_ChannelsPerSubject):
		return http.StatusTooManyRequests
	}
	return http.StatusInternalServerError
}

/*
curl -G --data-urlencode "payload={\"Channel\": \"a23b2050-b90c-4ed1-adb7-1ecc4f346f2b\", \"ProjectionKey\":[{\"App\":\"Application\",\"Projection\":\"paa.wine_price\",\"WS\":1}]}" https://alpha2.dev.untill.ru/n10n/subscribe -H "Content-Type: application/json"
*/
func (s *routerService) subscribeHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var parameters subscriberParamsType
		err := getJSONPayload(req, &parameters)
		extension := ""
		if len(parameters.ProjectionKey) > 0 {
			extension = parameters.ProjectionKey[0].Projection.String()
		}
		logCtx := withLogAttribs(req.Context(), validatedData{}, bus.Request{Resource: extension}, req)
		if err != nil {
			logger.ErrorCtx(logCtx, "n10n.error", err)
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if len(parameters.ProjectionKey) > 0 {
			logCtx = logger.WithContextAttrs(logCtx, map[string]any{
				logAttrib_ProjectionKey: in10n.ProjectionKeysToJSON(parameters.ProjectionKey),
			})
		}
		for _, projection := range parameters.ProjectionKey {
			if err = s.n10n.Subscribe(parameters.Channel, projection); err != nil {
				logger.ErrorCtx(logCtx, "n10n.error", err)
				http.Error(rw, "subscribe failed: "+err.Error(), n10nErrorToStatusCode(err))
				return
			}
		}
		logger.VerboseCtx(logCtx, "n10n.subscribe.success")
	}
}

/*
curl -G --data-urlencode "payload={\"Channel\": \"a23b2050-b90c-4ed1-adb7-1ecc4f346f2b\", \"ProjectionKey\":[{\"App\":\"Application\",\"Projection\":\"paa.wine_price\",\"WS\":1}]}" https://alpha2.dev.untill.ru/n10n/unsubscribe -H "Content-Type: application/json"
*/
func (s *routerService) unSubscribeHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		var parameters subscriberParamsType
		err := getJSONPayload(req, &parameters)
		extension := ""
		if len(parameters.ProjectionKey) > 0 {
			extension = parameters.ProjectionKey[0].Projection.String()
		}
		logCtx := withLogAttribs(req.Context(), validatedData{}, bus.Request{Resource: extension}, req)
		if err != nil {
			logger.ErrorCtx(logCtx, "n10n.error", err)
			http.Error(rw, err.Error(), http.StatusBadRequest)
			return
		}
		if len(parameters.ProjectionKey) > 0 {
			logCtx = logger.WithContextAttrs(logCtx, map[string]any{
				logAttrib_ProjectionKey: in10n.ProjectionKeysToJSON(parameters.ProjectionKey),
			})
		}
		for _, projection := range parameters.ProjectionKey {
			if err = s.n10n.Unsubscribe(parameters.Channel, projection); err != nil {
				logger.ErrorCtx(logCtx, "n10n.error", err)
				http.Error(rw, err.Error(), n10nErrorToStatusCode(err))
				return
			}
		}
		logger.VerboseCtx(logCtx, "n10n.unsubscribe.success")
	}
}

// curl -X POST "http://localhost:3001/n10n/update" -H "Content-Type: application/json" -d "{\"App\":\"Application\",\"Projection\":\"paa.price\",\"WS\":1}"
// TODO: eliminate after airs-bp3 integration tests implementation
func (s *routerService) updateHandler() http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		var p in10n.ProjectionKey
		body, err := io.ReadAll(req.Body)
		if err != nil {
			logger.Error(err)
			http.Error(resp, "Error when read request body", http.StatusInternalServerError)
			return
		}
		err = json.Unmarshal(body, &p)
		if err != nil {
			logger.Error(err)
			http.Error(resp, "Error when parse request body", http.StatusBadRequest)
			return
		}

		params := mux.Vars(req)
		offset := params["offset"]
		if off, err := strconvu.ParseUint64(offset); err == nil {
			s.n10n.Update(p, istructs.Offset(off))
		}
	}
}

func getJSONPayload(req *http.Request, payload *subscriberParamsType) error {
	jsonParam, ok := req.URL.Query()["payload"]
	if !ok || len(jsonParam[0]) < 1 {
		return errors.New("url parameter with payload (channel id and projection key) is missing")
	}
	if err := json.Unmarshal([]byte(jsonParam[0]), payload); err != nil {
		return fmt.Errorf("cannot unmarshal input payload %w", err)
	}
	return nil
}
