/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * @author Michael Saigachenko
 */

package projectors

import (
	"context"
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/state"
)

func syncActualizerFactory(conf SyncActualizerConf, projectors istructs.Projectors) pipeline.ISyncOperator {
	if conf.IntentsLimit == 0 {
		conf.IntentsLimit = defaultIntentsLimit
	}
	if conf.WorkToEvent == nil {
		conf.WorkToEvent = func(work interface{}) istructs.IPLogEvent { return work.(istructs.IPLogEvent) }
	}
	service := &eventService{}
	ss := make([]state.IHostState, 0, len(projectors))
	bb := make([]pipeline.ForkOperatorOptionFunc, 0, len(projectors))
	for _, p := range projectors {
		b, s := newSyncBranch(conf, p, service)
		ss = append(ss, s)
		bb = append(bb, b)
	}
	h := &syncErrorHandler{ss: ss}
	return pipeline.NewSyncPipeline(conf.Ctx, "PartitionSyncActualizer",
		pipeline.WireFunc("Update event", func(_ context.Context, work interface{}) (err error) {
			service.event = conf.WorkToEvent(work)
			return err
		}),
		pipeline.WireSyncOperator("SyncActualizer", pipeline.ForkOperator(pipeline.ForkSame, bb[0], bb[1:]...)),
		pipeline.WireFunc("IntentsApplier", func(_ context.Context, _ interface{}) (err error) {
			for _, st := range ss {
				err = st.ApplyIntents()
				if err != nil {
					return
				}
			}
			return
		}),
		pipeline.WireSyncOperator("ErrorHandler", h))
}

func newSyncBranch(conf SyncActualizerConf, projector istructs.Projector, service *eventService) (fn pipeline.ForkOperatorOptionFunc, s state.IHostState) {
	pipelineName := fmt.Sprintf("[%d] %s", conf.Partition, projector.Name)
	s = state.ProvideSyncActualizerStateFactory()(
		conf.Ctx,
		conf.AppStructs,
		state.SimplePartitionIDFunc(conf.Partition),
		service.getWSID,
		conf.N10nFunc,
		conf.SecretReader,
		service.getEvent,
		conf.IntentsLimit)
	fn = pipeline.ForkBranch(pipeline.NewSyncPipeline(conf.Ctx, pipelineName,
		pipeline.WireFunc("Projector",
			func(ctx context.Context, work interface{}) error {
				appPart := work.(interface{ AppPartition() appparts.IAppPartition }).AppPartition()
				appDef := appPart.AppStructs().AppDef()
				prj := appDef.Projector(projector.Name)
				event := s.PLogEvent()
				if !isAcceptable(event, prj.WantErrors(), prj.Events().Map(), appDef) {
					return nil
				}
				return appPart.Invoke(ctx, projector.Name, s, s)
			}),
		pipeline.WireFunc("IntentsValidator", func(_ context.Context, _ interface{}) (err error) {
			return s.ValidateIntents()
		})))
	return
}

type syncErrorHandler struct {
	pipeline.NOOP
	ss  []state.IHostState
	err error
}

func (h *syncErrorHandler) DoSync(_ context.Context, _ interface{}) (err error) {
	if h.err != nil {
		for _, s := range h.ss {
			s.ClearIntents()
		}
		err = h.err
		h.err = nil
	}
	return
}

func (h *syncErrorHandler) OnErr(err error, _ interface{}, _ pipeline.IWorkpieceContext) error {
	h.err = err
	return nil
}

type eventService struct {
	event istructs.IPLogEvent
}

func (s *eventService) getWSID() istructs.WSID { return s.event.Workspace() }

func (s *eventService) getEvent() istructs.IPLogEvent { return s.event }

func provideViewDefImpl(appDef appdef.IAppDefBuilder, qname appdef.QName, buildFunc ViewTypeBuilder) {
	builder := appDef.AddView(qname)
	if buildFunc != nil {
		buildFunc(builder)
	}
}
