/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * Aleksei Ponomarev
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package in10nmem

import (
	"context"
	"sync"

	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/in10n"
	istructs "github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/sys"
)

func NewN10nBroker(quotas in10n.Quotas, time timeu.ITime) (nb in10n.IN10nBroker, cleanup func()) {
	logCtx := logger.WithContextAttrs(context.Background(), map[string]any{
		logger.LogAttr_VApp:      sys.VApp_SysVoedger,
		logger.LogAttr_Extension: "sys._N10NBroker",
	})
	broker := N10nBroker{
		projections:     make(map[in10n.ProjectionKey]*projection),
		channels:        make(map[in10n.ChannelID]*channel),
		channelsWG:      sync.WaitGroup{},
		metricBySubject: make(map[istructs.SubjectLogin]*metricType),
		quotas:          quotas,
		time:            time,
		events:          make(chan event, eventsChannelSize),
		logCtx:          logCtx,
	}
	brokerCtx, cancel := context.WithCancel(context.Background())
	wg := sync.WaitGroup{}
	cleanup = func() {
		cancel()
		wg.Wait()
		broker.channelsWG.Wait()
	}

	wg.Add(1)
	go notifier(brokerCtx, logCtx, &wg, broker.events)
	wg.Add(1)
	go broker.heartbeat30(brokerCtx, &wg)

	return &broker, cleanup
}
