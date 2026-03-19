/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 */

package commandprocessor

import (
	"context"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/processors/actualizers"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/state/stateprovide"
)

type ServiceFactory func(commandsChannel CommandChannel) pipeline.IService
type CommandChannel iprocbus.ServiceChannel
type OperatorSyncActualizer pipeline.ISyncOperator
type SyncActualizerFactory func(vvmCtx context.Context, partitionID istructs.PartitionID) pipeline.ISyncOperator

type ValidateFunc func(ctx context.Context, appStructs istructs.IAppStructs, cudRow istructs.ICUDRow, wsid istructs.WSID) (err error)

type ICommandMessage interface {
	Body() []byte
	AppQName() appdef.AppQName
	WSID() istructs.WSID // url WSID
	Responder() bus.IResponder
	PartitionID() istructs.PartitionID
	RequestCtx() context.Context
	QName() appdef.QName // APIv1 -> cmd QName, APIv2 -> cmdQName or DocQName
	Token() string
	Host() string
	APIPath() processors.APIPath
	DocID() istructs.RecordID
	Method() string
	Origin() string
}

type xPath string

type commandProcessorMetrics struct {
	vvmName string
	app     appdef.AppQName
	metrics imetrics.IMetrics
}

func (m *commandProcessorMetrics) increase(metricName string, valueDelta float64) {
	m.metrics.IncreaseApp(metricName, m.vvmName, m.app, valueDelta)
}

type cmdWorkpiece struct {
	appParts                     appparts.IAppPartitions
	appPart                      appparts.IAppPartition
	appStructs                   istructs.IAppStructs
	requestData                  coreutils.MapObject
	cmdMes                       ICommandMessage
	argsObject                   istructs.IObject
	unloggedArgsObject           istructs.IObject
	reb                          istructs.IRawEventBuilder
	rawEvent                     istructs.IRawEvent
	pLogEvent                    istructs.IPLogEvent
	appPartition                 *appPartition
	workspace                    *workspace
	idGeneratorReporter          *implIDGeneratorReporter
	eca                          istructs.ExecCommandArgs
	metrics                      commandProcessorMetrics
	syncProjectorsStart          time.Time
	principals                   []iauthnz.Principal
	roles                        []appdef.QName
	parsedCUDs                   []parsedCUD
	wsDesc                       istructs.IRecord
	hostStateProvider            *hostStateProvider
	wsInitialized                bool
	cmdResultBuilder             istructs.IObjectBuilder
	cmdResult                    istructs.IObject
	iCommand                     appdef.ICommand
	iWorkspace                   appdef.IWorkspace
	appPartitionRestartScheduled bool
	cmdQName                     appdef.QName
	statusCodeOfSuccess          int
	reapplier                    istructs.IEventReapplier
	commandCtxStorage            istructs.IStateValue
	cmdResToLog                  string
	pLogOffset                   istructs.Offset // need for logging
	logCtx                       context.Context // enriched log ctx from logEventAndCUDs (woffset, poffset, evqname), used by sync projectors
}

type implIDGeneratorReporter struct {
	istructs.IIDGenerator
	generatedIDs map[istructs.RecordID]istructs.RecordID
}

type parsedCUD struct {
	opKind         appdef.OperationKind // update can not be activate\deactivate because IsActive modified -> other fields update is not allowed, see
	existingRecord istructs.IRecord     // create -> nil
	id             int64
	qName          appdef.QName
	fields         coreutils.MapObject
	xPath          xPath
}

type implICommandMessage struct {
	body        []byte
	appQName    appdef.AppQName // need to determine where to send c.sys.Init request on create a new workspace
	wsid        istructs.WSID
	responder   bus.IResponder
	partitionID istructs.PartitionID
	requestCtx  context.Context
	qName       appdef.QName // APIv1 -> cmd QName, APIv2 -> cmdQName or DocQName
	token       string
	host        string
	apiPath     processors.APIPath
	docID       istructs.RecordID
	method      string
	origin      string
}

type wrongArgsCatcher struct {
	pipeline.NOOP
}

type hostStateProvider struct {
	as               istructs.IAppStructs
	cud              istructs.ICUD
	wsid             istructs.WSID
	principals       []iauthnz.Principal
	state            state.IHostState
	token            string
	cmdResultBuilder istructs.IObjectBuilder
	cmdPrepareArgs   istructs.CommandPrepareArgs
	wlogOffset       istructs.Offset
	origin           string
	args             istructs.IObject
	unloggedArgs     istructs.IObject
	partitionID      istructs.PartitionID
}

func newHostStateProvider(ctx context.Context, secretReader isecrets.ISecretReader) *hostStateProvider {
	p := &hostStateProvider{}
	p.state = stateprovide.ProvideCommandProcessorStateFactory()(ctx, p.getAppStructs, p.getPartititonID,
		p.getWSID, secretReader, p.getCUD, p.getPrincipals, p.getToken, actualizers.DefaultIntentsLimit,
		p.getCmdResultBuilder, p.getCmdPrepareArgs, p.getArgs, p.getUnloggedArgs, p.getWLogOffset, state.NullOpts, p.getOrigin)
	return p
}

func (p *hostStateProvider) getAppStructs() istructs.IAppStructs { return p.as }
func (p *hostStateProvider) getWSID() istructs.WSID              { return p.wsid }
func (p *hostStateProvider) getCUD() istructs.ICUD               { return p.cud }
func (p *hostStateProvider) getPrincipals() []iauthnz.Principal {
	return p.principals
}
func (p *hostStateProvider) getToken() string                               { return p.token }
func (p *hostStateProvider) getCmdResultBuilder() istructs.IObjectBuilder   { return p.cmdResultBuilder }
func (p *hostStateProvider) getCmdPrepareArgs() istructs.CommandPrepareArgs { return p.cmdPrepareArgs }
func (p *hostStateProvider) getWLogOffset() istructs.Offset                 { return p.wlogOffset }
func (p *hostStateProvider) getOrigin() string                              { return p.origin }
func (p *hostStateProvider) getArgs() istructs.IObject                      { return p.args }
func (p *hostStateProvider) getUnloggedArgs() istructs.IObject              { return p.unloggedArgs }
func (p *hostStateProvider) getPartititonID() istructs.PartitionID          { return p.partitionID }
func (p *hostStateProvider) get(appStructs istructs.IAppStructs, wsid istructs.WSID, cud istructs.ICUD, principals []iauthnz.Principal, token string,
	cmdResultBuilder istructs.IObjectBuilder, cmdPrepareArgs istructs.CommandPrepareArgs, wlogOffset istructs.Offset, args istructs.IObject,
	unloggedArgs istructs.IObject, partitionID istructs.PartitionID, origin string) state.IHostState {
	p.as = appStructs
	p.wsid = wsid
	p.cud = cud
	p.principals = principals
	p.token = token
	p.cmdResultBuilder = cmdResultBuilder
	p.cmdPrepareArgs = cmdPrepareArgs
	p.wlogOffset = wlogOffset
	p.args = args
	p.unloggedArgs = unloggedArgs
	p.partitionID = partitionID
	p.origin = origin
	return p.state
}
