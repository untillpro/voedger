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
	"sync/atomic"
	"time"

	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/in10n"
	istructs "github.com/voedger/voedger/pkg/istructs"
)

type N10nBroker struct {
	sync.RWMutex
	projections      map[in10n.ProjectionKey]*projection
	channels         map[in10n.ChannelID]*channel
	channelsWG       sync.WaitGroup
	quotas           in10n.Quotas
	metricBySubject  map[istructs.SubjectLogin]*metricType
	numSubscriptions int
	time             timeu.ITime
	events           chan event
	logCtx           context.Context
}

type event struct {
	prj *projection
}

type projection struct {
	sync.Mutex

	offsetPointer *istructs.Offset

	toSubscribe map[in10n.ChannelID]*channel

	// merged by pnotifier using toSubscribe, toUnsubscribe
	subscribedChannels map[in10n.ChannelID]*channel
}

type subscription struct {
	deliveredOffset istructs.Offset
	currentOffset   *istructs.Offset
}

type channel struct {
	subject         istructs.SubjectLogin
	subscriptions   map[in10n.ProjectionKey]*subscription
	channelDuration time.Duration
	createTime      time.Time
	cchan           chan struct{}
	terminated      bool
	watching        atomic.Bool
}

type metricType struct {
	numChannelsPerSubject      int
	numSubscriptionsPerSubject int
}

type UpdateUnit struct {
	Projection in10n.ProjectionKey
	Offset     istructs.Offset
}

type CreateChannelParamsType struct {
	SubjectLogin  istructs.SubjectLogin
	ProjectionKey []in10n.ProjectionKey
}
