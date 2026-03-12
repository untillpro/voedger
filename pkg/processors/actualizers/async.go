/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package actualizers

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/voedger/voedger/pkg/goutils/logger"
	retrier "github.com/voedger/voedger/pkg/goutils/retry"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/state/stateprovide"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/sys/authnz"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/state"
)

type workpiece struct {
	event      istructs.IPLogEvent
	pLogOffset istructs.Offset
}

func (w *workpiece) Release() {
	w.event.Release()
}

type (
	plogEvent struct {
		istructs.Offset
		istructs.IPLogEvent
	}
	plogBatch []plogEvent

	// Should return:
	//	- events in batch and no error, if events succussfully read
	//	- no events (empty batch) and no error, if all events are read
	//	- no events (empty  batch) and error, if error occurs while reading PLog.
	// Should not return simultaneously any events (non empty batch) and error
	readPLogBatch func(*plogBatch) error
)

// 1 asyncActualizer per each projector per each partition
type asyncActualizer struct {
	conf           AsyncActualizerConf
	projectorQName appdef.QName
	pipeline       pipeline.IAsyncPipeline
	offset         istructs.Offset
	name           string
	readCtx        *asyncActualizerContextState
	projErrState   int32 // 0 - no error, 1 - error
	plogBatch            // [50]plogEvent
	appParts       appparts.IAppPartitions
	retrierCfg     retrier.Config
	channelCleanup func()
}

func (a *asyncActualizer) Prepare(vvmCtx context.Context) {
	if a.conf.IntentsLimit == 0 {
		a.conf.IntentsLimit = defaultIntentsLimit
	}

	if a.conf.BundlesLimit == 0 {
		a.conf.BundlesLimit = defaultBundlesLimit
	}

	if a.conf.FlushInterval == 0 {
		a.conf.FlushInterval = defaultFlushInterval
	}
	if a.conf.FlushPositionInterval == 0 {
		a.conf.FlushPositionInterval = defaultFlushPositionInterval
	}

	a.retrierCfg.OnError = func(_ int, _ time.Duration, opErr error) (retry bool, err error) {
		a.logError(opErr)
		return true, nil
	}
}

func (a *asyncActualizer) logError(err error) {
	var errCtx errWithCtx
	if errors.As(err, &errCtx) {
		logger.ErrorCtx(errCtx.ctx, errCtx.error)
	} else {
		logger.ErrorCtx(a.readCtx.vvmCtx, err)
	}
}

func (a *asyncActualizer) Run(vvmCtx context.Context) {
	for vvmCtx.Err() == nil {
		_ = retrier.RetryNoResult(vvmCtx, a.retrierCfg, func() error {
			err := a.init(vvmCtx)
			if err == nil {
				err = a.keepReading()
			}
			a.finit() // execute even if a.init() has failed

			// avoiding panic on close the closed pipeline. Case:
			// init, error, finit, init again but error before pipeline creation, then 2nd finit -> panic on closing the closed channel within the pipeline
			// i.e. 2nd finit will panic if an error is returned from 2nd init before the place where the pipeline is re-created
			// see https://untill.atlassian.net/browse/AIR-2302
			a.pipeline = nil
			return err
		})
	}
}

func (a *asyncActualizer) cancelChannel(e error) {
	a.readCtx.cancelWithError(e)
	a.conf.Broker.WatchChannel(a.readCtx.vvmCtx, a.conf.channel, func(projection in10n.ProjectionKey, offset istructs.Offset) {})
}

func (a *asyncActualizer) init(vvmCtx context.Context) (err error) {
	a.plogBatch = make(plogBatch, 0, plogReadBatchSize)

	a.readCtx = &asyncActualizerContextState{}

	a.readCtx.vvmCtx, a.readCtx.cancel = context.WithCancel(vvmCtx)

	appDef, err := a.appParts.AppDef(a.conf.AppQName)
	if err != nil {
		return err
	}
	prjType := appdef.Projector(appDef.Type, a.projectorQName)
	if prjType == nil {
		return fmt.Errorf("async projector %s is not defined in AppDef", a.projectorQName)
	}

	// returns true if there are custom storages except «sys.View» and «sys.Record»
	customStorages := func(ss appdef.IStorages) bool {
		for _, n := range ss.Names() {
			if n != sys.Storage_View && n != sys.Storage_Record {
				return true
			}
		}
		return false
	}

	// https://github.com/voedger/voedger/issues/1048
	hasIntentsExceptViewAndRecord := customStorages(prjType.Intents())

	// https://github.com/voedger/voedger/issues/1092
	hasStatesExceptViewAndRecord := customStorages(prjType.States())

	nonBuffered := hasIntentsExceptViewAndRecord || hasStatesExceptViewAndRecord
	p := &asyncProjector{
		partitionID:           a.conf.PartitionID,
		aametrics:             a.conf.AAMetrics,
		flushPositionInterval: a.conf.FlushPositionInterval,
		lastSave:              time.Now(),
		projErrState:          &a.projErrState,
		metrics:               a.conf.Metrics,
		vvmName:               a.conf.VvmName,
		appQName:              a.conf.AppQName,
		name:                  a.projectorQName,
		iProjector:            prjType,
		nonBuffered:           nonBuffered,
		appParts:              a.appParts,
	}

	if p.metrics != nil {
		p.projInErrAddr = p.metrics.AppMetricAddr(ProjectorsInError, a.conf.VvmName, a.conf.AppQName)
	}

	a.name = fmt.Sprintf("%v [%d]", p.name, a.conf.PartitionID)

	err = a.readOffset(p.name)
	if err != nil {
		logger.ErrorCtx(vvmCtx, a.name, err)
		return err
	}

	p.state = stateprovide.ProvideAsyncActualizerStateFactory()(
		vvmCtx,
		p.borrowedAppStructs,
		state.SimplePartitionIDFunc(a.conf.PartitionID),
		p.WSIDProvider,
		func(view appdef.QName, wsid istructs.WSID, offset istructs.Offset) {
			a.conf.Broker.Update(in10n.ProjectionKey{
				App:        a.conf.AppQName,
				Projection: view,
				WS:         wsid,
			}, offset)
		},
		a.conf.SecretReader,
		p.EventProvider,
		a.conf.Tokens,
		a.conf.Federation,
		a.conf.IntentsLimit,
		a.conf.BundlesLimit,
		a.conf.StateOpts,
		a.conf.EmailSender,
		a.conf.HTTPClient,
	)

	projectorOp := pipeline.WireAsyncOperator("Projector", p, a.conf.FlushInterval)

	errHandler := &asyncErrorHandler{
		readCtx:      a.readCtx,
		projErrState: &a.projErrState,
		metrics:      a.conf.Metrics,
		vvmName:      a.conf.VvmName,
		appQName:     a.conf.AppQName,
	}
	errorHandlerOp := pipeline.WireAsyncOperator("ErrorHandler", errHandler)

	a.pipeline = pipeline.NewAsyncPipeline(vvmCtx, a.name, projectorOp, errorHandlerOp)

	if a.conf.channel, a.channelCleanup, err = a.conf.Broker.NewChannel(istructs.SubjectLogin(a.name), n10nChannelDuration); err != nil {
		return err
	}
	return a.conf.Broker.Subscribe(a.conf.channel, in10n.ProjectionKey{
		App:        a.conf.AppQName,
		Projection: PLogUpdatesQName,
		WS:         istructs.WSID(a.conf.PartitionID),
	})
}

func (a *asyncActualizer) finit() {
	if a.pipeline != nil {
		a.pipeline.Close()
	}
	if a.channelCleanup != nil {
		a.channelCleanup()
	}
}

func (a *asyncActualizer) keepReading() (err error) {
	err = a.readPlogToTheEnd()
	if err != nil {
		a.cancelChannel(err)
		return
	}
	a.conf.Broker.WatchChannel(a.readCtx.vvmCtx, a.conf.channel, func(projection in10n.ProjectionKey, offset istructs.Offset) {
		if logger.IsVerbose() {
			logger.VerboseCtx(a.readCtx.vvmCtx, fmt.Sprintf("received n10n: offset %d, last handled: %d", offset, a.offset))
		}
		if a.offset < offset {
			err = a.readPlogToOffset(a.readCtx.vvmCtx, offset)
			if err != nil {
				logger.ErrorCtx(a.readCtx.vvmCtx, a.name, err)
				a.readCtx.cancelWithError(err)
			}
		}
	})
	return a.readCtx.error()
}

func (a *asyncActualizer) handleEvent(pLogOffset istructs.Offset, event istructs.IPLogEvent) (err error) {
	work := &workpiece{
		event:      event,
		pLogOffset: pLogOffset,
	}

	err = a.pipeline.SendAsync(work)
	if err != nil {
		a.logError(err)
		return err
	}

	a.offset = pLogOffset

	return nil
}

func (a *asyncActualizer) readPlogByBatches(readBatch readPLogBatch) error {
	for a.readCtx.vvmCtx.Err() == nil {
		if err := readBatch(&a.plogBatch); err != nil {
			return err
		}
		if len(a.plogBatch) == 0 {
			break
		}
		for _, e := range a.plogBatch {
			if err := a.handleEvent(e.Offset, e.IPLogEvent); err != nil {
				return err
			}
			if a.readCtx.vvmCtx.Err() != nil {
				return nil // canceled
			}
		}
	}
	return nil
}

func (a *asyncActualizer) borrowAppPart(ctx context.Context) (ap appparts.IAppPartition, err error) {
	return a.appParts.WaitForBorrow(ctx, a.conf.AppQName, a.conf.PartitionID, appparts.ProcessorKind_Actualizer)
}

func (a *asyncActualizer) readPlogToTheEnd() error {
	return a.readPlogByBatches(func(batch *plogBatch) (err error) {
		*batch = (*batch)[:0]

		ap, err := a.borrowAppPart(a.readCtx.vvmCtx)
		if err != nil {
			return err
		}

		defer ap.Release()

		err = ap.AppStructs().Events().ReadPLog(a.readCtx.vvmCtx, a.conf.PartitionID, a.offset+1, istructs.ReadToTheEnd,
			func(ofs istructs.Offset, event istructs.IPLogEvent) error {
				if *batch = append(*batch, plogEvent{ofs, event}); len(*batch) == cap(*batch) {
					return errBatchFull
				}
				return nil
			})
		if len(*batch) > 0 {
			//nolint suppress error if at least one event was read
			return nil
		}
		return err
	})
}

func (a *asyncActualizer) readPlogToOffset(ctx context.Context, tillOffset istructs.Offset) error {
	return a.readPlogByBatches(func(batch *plogBatch) (err error) {
		*batch = (*batch)[:0]

		ap, err := a.borrowAppPart(ctx)
		if err != nil {
			return err
		}

		defer ap.Release()

		plog := ap.AppStructs().Events()
		for readOffset := a.offset + 1; readOffset <= tillOffset; readOffset++ {
			if err = plog.ReadPLog(a.readCtx.vvmCtx, a.conf.PartitionID, readOffset, 1,
				func(ofs istructs.Offset, event istructs.IPLogEvent) error {
					if *batch = append(*batch, plogEvent{ofs, event}); len(*batch) == cap(*batch) {
						return errBatchFull
					}
					return nil
				}); err != nil {
				break
			}
		}
		if len(*batch) > 0 {
			//nolint suppress error if at least one event was read
			return nil
		}
		return err
	})
}

func (a *asyncActualizer) readOffset(projectorName appdef.QName) error {
	ap, err := a.borrowAppPart(a.readCtx.vvmCtx)
	if err != nil {
		return err
	}

	defer ap.Release()

	a.offset, err = ActualizerOffset(ap.AppStructs(), a.conf.PartitionID, projectorName)
	return err
}

type asyncProjector struct {
	pipeline.AsyncNOOP
	state                 state.IBundledHostState
	partitionID           istructs.PartitionID
	event                 istructs.IPLogEvent
	name                  appdef.QName
	pLogOffset            istructs.Offset
	aametrics             AsyncActualizerMetrics
	metrics               imetrics.IMetrics
	projInErrAddr         *imetrics.MetricValue
	flushPositionInterval time.Duration
	acceptedSinceSave     bool
	lastSave              time.Time
	projErrState          *int32
	vvmName               string
	appQName              appdef.AppQName
	iProjector            appdef.IProjector
	nonBuffered           bool
	appParts              appparts.IAppPartitions
	borrowedPartition     appparts.IAppPartition
}

// DoAsync is executed for every event of a given partition.
// It determines whether:
//   - the projector is defined in vsql in the workspace of the event, and
//   - the projector is triggered by the event.
func (p *asyncProjector) DoAsync(ctx context.Context, work pipeline.IWorkpiece) (pipeline.IWorkpiece, error) {
	defer work.Release()
	w := work.(*workpiece)

	p.event = w.event
	p.pLogOffset = w.pLogOffset
	if p.aametrics != nil {
		p.aametrics.Set(aaCurrentOffset, p.partitionID, p.name, float64(w.pLogOffset))
	}

	triggeredByQName := ProjectorEvent(p.iProjector, w.event)
	if triggeredByQName == appdef.NullQName {
		return nil, nil
	}

	// ctx here contains vapp and extension attribs already
	logCtx := logger.WithContextAttrs(ctx, map[string]any{
		logger.LogAttr_WSID: w.event.Workspace(),
	})

	wrapErr := func(err error) error {
		return errWithCtx{err, logCtx}
	}

	if err := p.borrowAppPart(ctx); err != nil {
		return nil, wrapErr(err)
	}

	defer p.releaseAppPart()

	ok, err := p.isProjectorDefined()
	if err != nil {
		return nil, wrapErr(err)
	}
	if !ok {
		// projector is not defined in the workspace of the event -> skip
		return nil, nil
	}

	if logCtx, err = logEventAndCUDs(logCtx, w.event, w.pLogOffset,
		p.borrowedPartition.AppStructs().AppDef(), triggeredByQName); err != nil {
		// notest
		return nil, wrapErr(err)
	}

	if err := p.borrowedPartition.Invoke(logCtx, p.name, p.state, p.state); err != nil {
		return nil, wrapErr(err)
	}

	p.acceptedSinceSave = true

	readyToFlushBundle, err := p.state.ApplyIntents()
	if err != nil {
		return nil, wrapErr(err)
	}

	if readyToFlushBundle || p.nonBuffered {
		if err := p.flush(); err != nil {
			return nil, wrapErr(err)
		}
	}
	if logger.IsVerbose() {
		logger.VerboseCtx(logCtx, "success")
	}

	return nil, nil
}

func logEventAndCUDs(logCtx context.Context, event istructs.IPLogEvent,
	pLogOffset istructs.Offset, appDef appdef.IAppDef, triggeredByQName appdef.QName) (context.Context, error) {
	if !logger.IsVerbose() {
		return logCtx, nil
	}
	triggeredByKind := appDef.Type(triggeredByQName).Kind()
	triggeredByFunc := appdef.TypeKind_Functions.Contains(triggeredByKind)
	triggeredByODoc := triggeredByKind == appdef.TypeKind_ODoc || triggeredByKind == appdef.TypeKind_ORecord
	return processors.LogEventAndCUDs(logCtx, event, pLogOffset, appDef, 2,
		func(cud istructs.ICUDRow) (bool, string, error) {
			return triggeredByFunc || triggeredByODoc || cud.QName() == triggeredByQName, "", nil
		},
		"triggeredby="+triggeredByQName.String(),
	)
}

func (p *asyncProjector) isProjectorDefined() (bool, error) {
	skbCDocWorkspaceDescriptor, err := p.state.KeyBuilder(sys.Storage_Record, authnz.QNameCDocWorkspaceDescriptor)
	if err != nil {
		// notest
		return false, err
	}
	skbCDocWorkspaceDescriptor.PutQName(state.Field_Singleton, authnz.QNameCDocWorkspaceDescriptor)
	skbCDocWorkspaceDescriptor.PutInt64(state.Field_WSID, int64(p.event.Workspace())) // nolint G115
	svCDocWorkspaceDescriptor, err := p.state.MustExist(skbCDocWorkspaceDescriptor)
	if err != nil {
		// notest
		return false, err
	}
	iws := p.borrowedPartition.AppStructs().AppDef().WorkspaceByDescriptor(svCDocWorkspaceDescriptor.AsQName(authnz.Field_WSKind))
	return iws.Type(p.name).Kind() != appdef.TypeKind_null, nil
}

func (p *asyncProjector) Flush(pipeline.OpFuncFlush) error {
	if err := p.borrowAppPart(context.Background()); err != nil {
		return err
	}

	defer p.releaseAppPart()

	return p.flush()
}

func (p *asyncProjector) WSIDProvider() istructs.WSID        { return p.event.Workspace() }
func (p *asyncProjector) EventProvider() istructs.IPLogEvent { return p.event }

func (p *asyncProjector) borrowAppPart(ctx context.Context) (err error) {
	if p.borrowedPartition, err = p.appParts.WaitForBorrow(ctx, p.appQName, p.partitionID, appparts.ProcessorKind_Actualizer); err != nil {
		return err
	}
	return nil
}

func (p *asyncProjector) borrowedAppStructs() istructs.IAppStructs {
	if ap := p.borrowedPartition; ap != nil {
		return ap.AppStructs()
	}
	panic(errNoBorrowedPartition)
}

func (p *asyncProjector) releaseAppPart() {
	if ap := p.borrowedPartition; ap != nil {
		p.borrowedPartition = nil
		ap.Release()
	}
}

func (p *asyncProjector) savePosition() error {
	defer func() {
		p.acceptedSinceSave = false
		p.lastSave = time.Now()
	}()
	key, e := p.state.KeyBuilder(sys.Storage_View, qnameProjectionOffsets)
	if e != nil {
		return e
	}
	key.PutInt64(sys.Storage_View_Field_WSID, int64(istructs.NullWSID))
	key.PutInt32(partitionFld, int32(p.partitionID))
	key.PutQName(projectorNameFld, p.name)
	value, e := p.state.NewValue(key)
	if e != nil {
		return e
	}
	value.PutInt64(offsetFld, int64(p.pLogOffset)) // nolint G115
	return nil
}
func (p *asyncProjector) flush() (err error) {
	if p.pLogOffset == istructs.NullOffset {
		return
	}
	defer func() {
		p.pLogOffset = istructs.NullOffset
	}()

	timeToSavePosition := time.Since(p.lastSave) >= p.flushPositionInterval
	if p.acceptedSinceSave || timeToSavePosition {
		if err = p.savePosition(); err != nil {
			return
		}
	}
	_, err = p.state.ApplyIntents()
	if err != nil {
		return err
	}
	if p.aametrics != nil {
		p.aametrics.Increase(aaFlushesTotal, p.partitionID, p.name, 1)
		p.aametrics.Set(aaStoredOffset, p.partitionID, p.name, float64(p.pLogOffset))
	}
	err = p.state.FlushBundles()
	if err == nil && p.projInErrAddr != nil {
		if atomic.CompareAndSwapInt32(p.projErrState, 1, 0) {
			p.projInErrAddr.Increase(-1)
		} else { // state not changed
			p.projInErrAddr.Increase(0)
		}
	}
	return err
}

type asyncErrorHandler struct {
	pipeline.AsyncNOOP
	readCtx      *asyncActualizerContextState
	metrics      imetrics.IMetrics
	vvmName      string
	appQName     appdef.AppQName
	projErrState *int32
}

func (h *asyncErrorHandler) OnError(_ context.Context, err error) {
	if atomic.CompareAndSwapInt32(h.projErrState, 0, 1) {
		if h.metrics != nil {
			h.metrics.IncreaseApp(ProjectorsInError, h.vvmName, h.appQName, 1)
		}
	}
	h.readCtx.cancelWithError(err)
}

func ActualizerOffset(appStructs istructs.IAppStructs, partition istructs.PartitionID, projectorName appdef.QName) (offset istructs.Offset, err error) {
	key := appStructs.ViewRecords().KeyBuilder(qnameProjectionOffsets)
	key.PutInt32(partitionFld, int32(partition))
	key.PutQName(projectorNameFld, projectorName)
	value, err := appStructs.ViewRecords().Get(istructs.NullWSID, key)
	if errors.Is(err, istructs.ErrRecordNotFound) {
		return istructs.NullOffset, nil
	}
	if err != nil {
		return istructs.NullOffset, err
	}
	return istructs.Offset(value.AsInt64(offsetFld)), err // nolint G115
}
