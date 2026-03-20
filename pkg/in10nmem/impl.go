/*
 * Copyright (c) 2023-present unTill Pro, Ltd.
 * @author Maxim Geraskin
 * Deep refactoring, no timers
 *
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * Aleksei Ponomarev
 * Initial implementation
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 */

package in10nmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/in10n"
	istructs "github.com/voedger/voedger/pkg/istructs"
	"golang.org/x/exp/maps"
)

// NewChannel @ConcurrentAccess
// Create new channel.
// On timeout channel will be closed. channelDuration determines time during with it will be open.
func (nb *N10nBroker) NewChannel(subject istructs.SubjectLogin, channelDuration time.Duration) (channelID in10n.ChannelID, channelCleanup func(), err error) {
	nb.Lock()
	defer nb.Unlock()
	var metric *metricType
	if len(nb.channels) >= nb.quotas.Channels {
		return "", nil, in10n.ErrQuotaExceeded_Channels
	}
	metric = nb.metricBySubject[subject]
	if metric != nil {
		if metric.numChannelsPerSubject >= nb.quotas.ChannelsPerSubject {
			return "", nil, in10n.ErrQuotaExceeded_ChannelsPerSubject
		}
	} else {
		metric = new(metricType)
		nb.metricBySubject[subject] = metric
	}
	metric.numChannelsPerSubject++
	channelID = in10n.ChannelID(uuid.New().String())
	channel := channel{
		subject:         subject,
		subscriptions:   make(map[in10n.ProjectionKey]*subscription),
		channelDuration: channelDuration,
		createTime:      nb.time.Now(),
		cchan:           make(chan struct{}, 1),
	}
	nb.channels[channelID] = &channel
	channelCleanup = func() { nb.cleanupChannel(&channel, channelID, metric) }
	nb.channelsWG.Add(1)
	return channelID, channelCleanup, err
}

// Implementation of in10n.IN10nBroker
// Errors: ErrChannelDoesNotExist, ErrQuotaExceeded_Subscriptions*
//
// [~server.n10n.heartbeats/freq.ZeroKey~doc]:
// - If Subscribe is called for QNameHeartbeat30:
//   - ProjectionKey.WSID is set 0
//   - ProjectionKey.AppQName is set to {"", ""}
//
// [~server.n10n.heartbeats/freq.Interval30Seconds~doc]
// - Implementation generates a heartbeat every 30 seconds for all channels that are subscribed on QNameHeartbeat30
func (nb *N10nBroker) Subscribe(channelID in10n.ChannelID, projectionKey in10n.ProjectionKey) (err error) {

	var prj *projection
	var channel *channel
	var channelOK bool

	// Modify broker structures
	err = func() error {
		nb.Lock()
		defer nb.Unlock()

		channel, channelOK = nb.channels[channelID]

		if !channelOK {
			return in10n.ErrChannelDoesNotExist
		}

		// We cannot subscribe to a channel that is already terminated
		if channel.terminated {
			return in10n.ErrChannelTerminated
		}

		metric, metricOK := nb.metricBySubject[channel.subject]
		if !metricOK {
			return ErrMetricDoesNotExists
		}

		if nb.numSubscriptions >= nb.quotas.Subscriptions {
			return in10n.ErrQuotaExceeded_Subscriptions
		}
		if metric.numSubscriptionsPerSubject >= nb.quotas.SubscriptionsPerSubject {
			return in10n.ErrQuotaExceeded_SubscriptionsPerSubject
		}

		// [~server.n10n.heartbeats/freq.ZeroKey~impl]
		// [~server.n10n.heartbeats/freq.SingleNotification~impl]
		if projectionKey.Projection == in10n.QNameHeartbeat30 {
			projectionKey = in10n.Heartbeat30ProjectionKey
		}

		currentOffset := guaranteeProjection(nb.projections, projectionKey)
		if _, ok := channel.subscriptions[projectionKey]; !ok {
			// do not affect metrics and internal maps on Subscribe again
			subscription := subscription{
				deliveredOffset: istructs.Offset(0),
				currentOffset:   currentOffset,
			}
			channel.subscriptions[projectionKey] = &subscription
			metric.numSubscriptionsPerSubject++
			nb.numSubscriptions++
		}

		// Must exist because we create it in guaranteeProjection
		prj = nb.projections[projectionKey]

		return nil
	}()

	if err != nil {
		return err
	}

	// Trigger notifier
	{
		prj.Lock()
		prj.toSubscribe[channelID] = channel
		prj.Unlock()
		e := event{prj: prj}
		nb.events <- e
	}

	return err
}

func (nb *N10nBroker) Unsubscribe(channelID in10n.ChannelID, projectionKey in10n.ProjectionKey) (err error) {

	var prj *projection
	var channel *channel
	var cOK bool

	// Modify broker structures
	err = func() error {

		nb.Lock()
		defer nb.Unlock()

		channel, cOK = nb.channels[channelID]
		if !cOK {
			return in10n.ErrChannelDoesNotExist
		}

		// if channel.terminated {
		// Ok we can unsubscribe from terminated channel

		if _, ok := channel.subscriptions[projectionKey]; ok {
			metric, mOK := nb.metricBySubject[channel.subject]
			if !mOK {
				return ErrMetricDoesNotExists
			}
			delete(channel.subscriptions, projectionKey)
			metric.numSubscriptionsPerSubject--
			nb.numSubscriptions--
		}

		prj = nb.projections[projectionKey]
		return nil
	}()

	if err != nil {
		return err
	}

	// Trigger notifier
	if prj != nil {
		prj.Lock()
		prj.toSubscribe[channelID] = nil
		prj.Unlock()
		e := event{prj: prj}
		nb.events <- e
	}

	return err
}

// Implementation of the in10n.IN10nBroker
// watchCtx normally is normally request+VVM ctx
func (nb *N10nBroker) WatchChannel(watchCtx context.Context, channelID in10n.ChannelID, notifySubscriber func(projection in10n.ProjectionKey, offset istructs.Offset)) {
	// check that the channelID with the given ChannelID exists
	channel := func() *channel {
		nb.RLock()
		defer nb.RUnlock()
		channel, channelOK := nb.channels[channelID]
		if !channelOK {
			panic(fmt.Errorf("channel with channelID: %s must exist %w", channelID, in10n.ErrChannelDoesNotExist))
		}
		_, metricOK := nb.metricBySubject[channel.subject]
		if !metricOK {
			panic(fmt.Errorf("metric for channel with channelID: %s must exist", channelID))
		}
		return channel
	}()

	if !channel.watching.CompareAndSwap(false, true) {
		panic(fmt.Errorf("%w: %s", in10n.ErrChannelAlreadyBeingWatched, channelID))
	}
	defer channel.watching.Store(false)

	updateUnits := make([]UpdateUnit, 0)

	// cycle for channel.cchan and ctx
	for watchCtx.Err() == nil {
		select {
		case <-watchCtx.Done():
			return
		case <-channel.cchan:
			if watchCtx.Err() != nil {
				return
			}

			err := nb.validateChannel(channel)
			if err != nil {
				logger.ErrorCtx(nb.logCtx, "n10n.channel.expired", channel.subject)
				return
			}

			// find projection for update and collect
			nb.Lock()
			for projection, channelOffsets := range channel.subscriptions {
				if *channelOffsets.currentOffset > channelOffsets.deliveredOffset {
					updateUnits = append(updateUnits,
						UpdateUnit{
							Projection: projection,
							Offset:     *channelOffsets.currentOffset,
						})
					channelOffsets.deliveredOffset = *channelOffsets.currentOffset
				}
			}
			nb.Unlock()
			for _, unit := range updateUnits {
				notifySubscriber(unit.Projection, unit.Offset)
			}
			updateUnits = updateUnits[:0]
		}

	}
}

func (nb *N10nBroker) cleanupChannel(channel *channel, channelID in10n.ChannelID, metric *metricType) {

	// Mark channel as terminated and unsubscribe from all projections
	{
		if channel.terminated {
			panic(in10n.ErrChannelTerminated)
		}

		var clonedSubs map[in10n.ProjectionKey]*subscription
		{
			nb.Lock()
			// Copy channel.subscriptions to a temporary map
			// to avoid concurrent map access issues when removing subscriptions
			clonedSubs = maps.Clone(channel.subscriptions)
			channel.terminated = true
			nb.Unlock()
		}

		// Unsubscribe from all subscriptions
		for projectionKey := range clonedSubs {
			err := nb.Unsubscribe(channelID, projectionKey)
			if err != nil {
				errCtx := logger.WithContextAttrs(nb.logCtx, map[string]any{
					"channelid":     string(channelID),
					"projectionkey": in10n.ProjectionKeysToJSON([]in10n.ProjectionKey{projectionKey}),
				})
				logger.ErrorCtx(errCtx, "n10n.cleanup.error", err.Error())
			}
		}
	}

	nb.Lock()
	metric.numChannelsPerSubject--
	delete(nb.channels, channelID)
	nb.Unlock()
	nb.channelsWG.Done()
}

func notifier(brokerCtx context.Context, logCtx context.Context, wg *sync.WaitGroup, events chan event) {
	defer func() {
		logger.InfoCtx(logCtx, "n10n.notifier.stop")
		wg.Done()
	}()

	logger.InfoCtx(logCtx, "n10n.notifier.start")

	for brokerCtx.Err() == nil {
		select {
		case <-brokerCtx.Done():
			return
		case eve := <-events:
			prj := eve.prj

			// Actualize subscriptions
			{
				prj.Lock()
				for channelID, channel := range prj.toSubscribe {
					if channel != nil {
						prj.subscribedChannels[channelID] = channel
					} else {
						delete(prj.subscribedChannels, channelID)
					}
				}
				maps.Clear(prj.toSubscribe)
				prj.Unlock()
			}

			// Notify subscribers
			for _, ch := range prj.subscribedChannels {
				// note: ch could be closed\terminated here but still in prj.subscribedChannels
				// just fail to write to cchan in this case, no problem
				select {
				case ch.cchan <- struct{}{}:
				default:
					// normally happens when the <-cchan is triggered in WatchChannel but next receiving from cchan is not started yet
					// i.e. WatchChannel does not read from cchan in the current instant
					// the event will not be lost because cchan has a value and the according offset is already updated
				}
			}
		}
	}
}

func guaranteeProjection(projections map[in10n.ProjectionKey]*projection, projectionKey in10n.ProjectionKey) (offsetPointer *istructs.Offset) {
	prj := projections[projectionKey]
	if prj == nil {
		prj = &projection{
			subscribedChannels: make(map[in10n.ChannelID]*channel),
			offsetPointer:      new(istructs.Offset),
			toSubscribe:        make(map[in10n.ChannelID]*channel),
		}
		projections[projectionKey] = prj

	}
	return prj.offsetPointer
}

// Update @ConcurrentAccess
// Update projections map with new offset
func (nb *N10nBroker) Update(projection in10n.ProjectionKey, offset istructs.Offset) {
	nb.Lock()
	*guaranteeProjection(nb.projections, projection) = offset
	prj := nb.projections[projection]
	nb.Unlock()

	e := event{prj: prj}
	nb.events <- e
}

// MetricNumChannels @ConcurrentAccess
// return channels count
func (nb *N10nBroker) MetricNumChannels() int {
	nb.RLock()
	defer nb.RUnlock()
	return len(nb.channels)
}

func (nb *N10nBroker) MetricNumSubscriptions() int {
	nb.RLock()
	defer nb.RUnlock()
	return nb.numSubscriptions
}

func (nb *N10nBroker) MetricSubject(ctx context.Context, cb func(subject istructs.SubjectLogin, numChannels int, numSubscriptions int)) {
	postMetric := func(subject istructs.SubjectLogin, metric *metricType) (err error) {
		nb.RLock()
		defer nb.RUnlock()
		cb(subject, metric.numChannelsPerSubject, metric.numSubscriptionsPerSubject)
		return err
	}
	for subject, subjectMetric := range nb.metricBySubject {
		if ctx.Err() != nil {
			return
		}
		err := postMetric(subject, subjectMetric)
		if err != nil {
			return
		}
	}
}

func (nb *N10nBroker) MetricNumProjectionSubscriptions(projection in10n.ProjectionKey) int {
	nb.RLock()
	defer nb.RUnlock()
	prj := nb.projections[projection]
	if prj == nil {
		return 0
	}
	prj.Lock()
	defer prj.Unlock()
	return len(prj.subscribedChannels)
}

// Call Update() every 30 seconds for i10n.Heartbeat30ProjectionKey
func (nb *N10nBroker) heartbeat30(brokerCtx context.Context, wg *sync.WaitGroup) {
	defer func() {
		logger.InfoCtx(nb.logCtx, "n10n.heartbeat.stop")
		wg.Done()
	}()

	// [~server.n10n.heartbeats/freq.Interval30Seconds~impl]
	ticker := nb.time.NewTimerChan(in10n.Heartbeat30Duration)
	logger.InfoCtx(nb.logCtx, "n10n.heartbeat.start", "Heartbeat30Duration: ", in10n.Heartbeat30Duration)

	offset := istructs.Offset(1)

	for {
		select {
		case <-brokerCtx.Done():
			return
		case <-ticker:
			nb.Update(in10n.Heartbeat30ProjectionKey, offset)
			offset++
			ticker = nb.time.NewTimerChan(in10n.Heartbeat30Duration)
		}
	}
}

func (nb *N10nBroker) validateChannel(channel *channel) error {
	// if channel lifetime > channelDuration defined in NewChannel when create channel - must exit
	if nb.time.Now().Sub(channel.createTime) > channel.channelDuration {
		return ErrChannelExpired
	}
	return nil
}
