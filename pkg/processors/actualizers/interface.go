/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package actualizers

import (
	"context"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/state"
)

type TimeAfterFunc func(d time.Duration) <-chan time.Time

type BasicAsyncActualizerConfig struct {
	VvmName string

	SecretReader isecrets.ISecretReader
	Tokens       itokens.ITokens
	Metrics      imetrics.IMetrics
	Broker       in10n.IN10nBroker
	Federation   federation.IFederation

	StateOpts  state.StateOpts
	HTTPClient httpu.IHTTPClient

	// Optional.
	AAMetrics AsyncActualizerMetrics
	//IntentsLimit top limit per event, optional, default value is 100
	IntentsLimit int
	//BundlesLimit top limit when bundle size is greater than this value, actualizer flushes changes to underlying storage, optional, default value is 100
	BundlesLimit int
	//FlushInterval specifies how often the current actualizer flushes changes to underlying storage, optional, default value is 100 milliseconds
	FlushInterval time.Duration
	// FlushPositionInterval specifies how often actualizer must save it's position, even when no events has been processed by actualizer. Default is 1 minute
	FlushPositionInterval time.Duration

	EmailSender state.IEmailSender
}

type AsyncActualizerConf struct {
	BasicAsyncActualizerConfig

	AppQName    appdef.AppQName
	PartitionID istructs.PartitionID

	channel in10n.ChannelID
}

type AsyncActualizerMetrics interface {
	Increase(metricName string, partition istructs.PartitionID, projection appdef.QName, valueDelta float64)
	Set(metricName string, partition istructs.PartitionID, projection appdef.QName, value float64)
}

type SyncActualizerConf struct {
	Ctx          context.Context
	SecretReader isecrets.ISecretReader
	Partition    istructs.PartitionID
	//IntentsLimit top limit per event, default value is 100
	IntentsLimit int
	N10nFunc     state.N10nFunc
}

type ViewTypeBuilder func(builder appdef.IViewBuilder)

// SyncActualizerFactory returns the Operator<SyncActualizer>
// Workpiece is ...?
type SyncActualizerFactory func(conf SyncActualizerConf, projectors istructs.Projectors) pipeline.ISyncOperator
