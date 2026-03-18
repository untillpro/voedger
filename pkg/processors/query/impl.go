/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * * @author Michael Saigachenko
 */

package queryprocessor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/processors/oldacl"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/state/stateprovide"
	"github.com/voedger/voedger/pkg/sys/authnz"

	"github.com/voedger/voedger/pkg/coreutils"
)

func implRowsProcessorFactory(ctx context.Context, appDef appdef.IAppDef, state istructs.IState, params IQueryParams,
	resultMeta appdef.IType, responder bus.IResponder, metrics IMetrics, errCh chan<- error) (rowsProcessor pipeline.IAsyncPipeline, iResponseWriterGetter func() bus.IResponseWriter) {
	operators := make([]*pipeline.WiredOperator, 0)
	if resultMeta == nil {
		// happens when the query has no result, e.g. q.air.UpdateSubscriptionDetails
		operators = append(operators, pipeline.WireAsyncOperator("noop, no result", &pipeline.AsyncNOOP{}))
	} else if resultMeta.QName() == istructs.QNameRaw {
		operators = append(operators, pipeline.WireAsyncOperator("Raw result", &RawResultOperator{
			metrics: metrics,
		}))
	} else {
		fieldsDefs := &fieldsDefs{
			appDef: appDef,
			fields: make(map[appdef.QName]FieldsKinds),
		}
		rootFields := newFieldsKinds(resultMeta)
		operators = append(operators, pipeline.WireAsyncOperator("Result fields", &ResultFieldsOperator{
			elements:   params.Elements(),
			rootFields: rootFields,
			fieldsDefs: fieldsDefs,
			metrics:    metrics,
		}))
		operators = append(operators, pipeline.WireAsyncOperator("Enrichment", &EnrichmentOperator{
			state:      state,
			elements:   params.Elements(),
			fieldsDefs: fieldsDefs,
			metrics:    metrics,
		}))
		if len(params.Filters()) != 0 {
			operators = append(operators, pipeline.WireAsyncOperator("Filter", &FilterOperator{
				filters:    params.Filters(),
				rootFields: rootFields,
				metrics:    metrics,
			}))
		}
		if len(params.OrderBy()) != 0 {
			operators = append(operators, pipeline.WireAsyncOperator("Order", newOrderOperator(params.OrderBy(), metrics)))
		}
		if params.StartFrom() != 0 || params.Count() != 0 {
			operators = append(operators, pipeline.WireAsyncOperator("Counter", newCounterOperator(
				params.StartFrom(),
				params.Count(),
				metrics)))
		}
	}
	sendToBusOp := &SendToBusOperator{
		responder: responder,
		metrics:   metrics,
		errCh:     errCh,
	}
	operators = append(operators, pipeline.WireAsyncOperator("Send to bus", sendToBusOp))
	return pipeline.NewAsyncPipeline(ctx, "Rows processor", operators[0], operators[1:]...), func() bus.IResponseWriter {
		return sendToBusOp.responseWriter
	}
}

func implServiceFactory(serviceChannel iprocbus.ServiceChannel,
	appParts appparts.IAppPartitions, maxPrepareQueries int, metrics imetrics.IMetrics, vvm string,
	authn iauthnz.IAuthenticator, itokens itokens.ITokens, federation federation.IFederation,
	statelessResources istructsmem.IStatelessResources, secretReader isecrets.ISecretReader,
	stateOpts state.StateOpts, httpClient httpu.IHTTPClient) pipeline.IService {
	return pipeline.NewService(func(ctx context.Context) {
		var p pipeline.ISyncPipeline
		for ctx.Err() == nil {
			select {
			case intf := <-serviceChannel:
				now := time.Now()
				msg := intf.(IQueryMessage)
				qpm := &queryProcessorMetrics{
					vvm:     vvm,
					app:     msg.AppQName(),
					metrics: metrics,
				}
				qpm.Increase(Metric_QueriesTotal, 1.0)
				qwork := newQueryWork(msg, appParts, maxPrepareQueries, qpm, secretReader)
				func() { // borrowed application partition should be guaranteed to be freed
					defer qwork.Release()
					if p == nil {
						p = newQueryProcessorPipeline(ctx, authn, itokens, federation, statelessResources, stateOpts, httpClient)
					}
					err := p.SendSync(qwork)
					if err != nil {
						qpm.Increase(Metric_ErrorsTotal, 1.0)
						p.Close()
						p = nil
					} else {
						if err = execQuery(ctx, qwork); err == nil {
							logger.VerboseCtx(qwork.msg.RequestCtx(), "qp.success")
							if err = processors.CheckResponseIntent(qwork.state); err == nil {
								err = qwork.state.ApplyIntents()
							}
						}
					}
					if qwork.rowsProcessor != nil {
						// wait until all rows are sent
						qwork.rowsProcessor.Close()
					}
					select {
					case rowsProcErr := <-qwork.rowsProcessorErrCh:
						if err == nil {
							err = rowsProcErr
						}
					default:
					}
					err = coreutils.WrapSysError(err, http.StatusInternalServerError)
					var respWriter bus.IResponseWriter
					statusCode := http.StatusOK
					if err != nil {
						statusCode = err.(coreutils.SysError).HTTPStatus // nolint:errorlint
						logger.ErrorCtx(qwork.msg.RequestCtx(), "qp.error", err)
					}
					if qwork.responseWriterGetter == nil || qwork.responseWriterGetter() == nil {
						// have an error before 200ok is sent -> send the status from the actual error
						respWriter = msg.Responder().StreamJSON(statusCode)
					} else {
						respWriter = qwork.responseWriterGetter()
					}
					respWriter.Close(err)
				}()
				metrics.IncreaseApp(Metric_QueriesSeconds, vvm, msg.AppQName(), time.Since(now).Seconds())
			case <-ctx.Done():
			}
		}
		if p != nil {
			p.Close()
		}
	})
}

func execQuery(ctx context.Context, qw *queryWork) (err error) {
	now := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			err = fmt.Errorf("%v\n%s", r, stack)
		}
		qw.metrics.Increase(Metric_ExecSeconds, time.Since(now).Seconds())
	}()
	return qw.appPart.Invoke(ctx, qw.msg.QName(), qw.state, qw.state)
}

// IStatelessResources need only for determine the exact result type of ANY
func newQueryProcessorPipeline(requestCtx context.Context, authn iauthnz.IAuthenticator,
	itokens itokens.ITokens, federation federation.IFederation, statelessResources istructsmem.IStatelessResources,
	stateOpts state.StateOpts, httpClient httpu.IHTTPClient) pipeline.ISyncPipeline {
	ops := []*pipeline.WiredOperator{
		operator("borrowAppPart", borrowAppPart),
		operator("check function call rate", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.appStructs.IsFunctionRateLimitsExceeded(qw.msg.QName(), qw.msg.WSID()) {
				return coreutils.NewSysError(http.StatusTooManyRequests)
			}
			return nil
		}),
		operator("authenticate query request", func(ctx context.Context, qw *queryWork) (err error) {
			if processors.SetPrincipalsForAnonymousOnlyFunc(qw.appStructs.AppDef(), qw.msg.QName(), qw.msg.WSID(), qw) {
				// grant to anonymous -> set token == "" to avoid validating an expired token accidentally kept in cookies
				return nil
			}
			req := iauthnz.AuthnRequest{
				Host:        qw.msg.Host(),
				RequestWSID: qw.msg.WSID(),
				Token:       qw.msg.Token(),
			}
			if qw.principals, _, err = authn.Authenticate(qw.msg.RequestCtx(), qw.appStructs, qw.appStructs.AppTokens(), req); err != nil {
				return coreutils.WrapSysError(err, http.StatusUnauthorized)
			}
			return
		}),
		operator("get workspace descriptor", func(ctx context.Context, qw *queryWork) (err error) {
			qw.wsDesc, err = processors.GetWSDesc(qw.msg.WSID(), qw.appStructs)
			return err
		}),
		operator("get principals roles", func(ctx context.Context, qw *queryWork) (err error) {
			qw.roles = processors.GetRoles(qw.principals)
			return nil
		}),
		operator("check workspace active", func(ctx context.Context, qw *queryWork) (err error) {
			for _, prn := range qw.principals {
				if prn.Kind == iauthnz.PrincipalKind_Role && prn.QName == iauthnz.QNameRoleSystem && prn.WSID == qw.msg.WSID() {
					// system -> allow to work in any case
					return nil
				}
			}
			if qw.wsDesc.QName() == appdef.NullQName {
				// TODO: query prcessor currently does not check the workspace active state
				return nil
			}
			if qw.wsDesc.AsInt32(authnz.Field_Status) != int32(authnz.WorkspaceStatus_Active) {
				return processors.ErrWSInactive
			}
			return nil
		}),

		operator("get IWorkspace", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.iWorkspace = qw.appStructs.AppDef().WorkspaceByDescriptor(qw.wsDesc.AsQName(authnz.Field_WSKind)); qw.iWorkspace == nil {
				return coreutils.NewHTTPErrorf(http.StatusInternalServerError, fmt.Sprintf("workspace is not found in AppDef by cdoc.sys.WorkspaceDescriptor.WSKind %s",
					qw.wsDesc.AsQName(authnz.Field_WSKind)))
			}
			return nil
		}),

		operator("get IQuery", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.iQuery = appdef.Query(qw.iWorkspace.Type, qw.msg.QName()); qw.iQuery == nil {
				return coreutils.NewHTTPErrorf(http.StatusBadRequest, fmt.Sprintf("query %s does not exist in %v", qw.msg.QName(), qw.iWorkspace))
			}
			return nil
		}),

		operator("authorize query request", func(ctx context.Context, qw *queryWork) (err error) {
			ws := qw.iWorkspace

			newACLOk, err := qw.appPart.IsOperationAllowed(ws, appdef.OperationKind_Execute, qw.msg.QName(), nil, qw.roles)
			if err != nil {
				return err
			}
			// TODO: temporary solution. To be eliminated after implementing ACL in VSQL for Air
			oldACLOk := oldacl.IsOperationAllowed(appdef.OperationKind_Execute, qw.msg.QName(), nil, oldacl.EnrichPrincipals(qw.principals, qw.msg.WSID()))
			if !newACLOk && !oldACLOk {
				return coreutils.WrapSysError(errors.New(""), http.StatusForbidden)
			}
			if !newACLOk && oldACLOk {
				logger.Verbose("newACL not ok, but oldACL ok.", appdef.OperationKind_Execute, qw.msg.QName(), qw.roles)
			}
			return nil
		}),
		operator("unmarshal request", func(ctx context.Context, qw *queryWork) (err error) {
			parsType := qw.iQuery.Param()
			if parsType != nil && parsType.QName() == istructs.QNameRaw {
				qw.requestData["args"] = map[string]interface{}{
					processors.Field_RawObject_Body: string(qw.msg.Body()),
				}
				return nil
			}
			err = coreutils.JSONUnmarshal(qw.msg.Body(), &qw.requestData)
			return coreutils.WrapSysError(err, http.StatusBadRequest)
		}),
		operator("validate: get exec query args", func(ctx context.Context, qw *queryWork) (err error) {
			qw.execQueryArgs, err = newExecQueryArgs(qw.requestData, qw.msg.WSID(), qw)
			return coreutils.WrapSysError(err, http.StatusBadRequest)
		}),
		operator("create callback func", func(ctx context.Context, qw *queryWork) (err error) {
			qw.callbackFunc = func(object istructs.IObject) error {
				pathToIdx := make(map[string]int)
				if qw.resultType.QName() == istructs.QNameRaw {
					pathToIdx[processors.Field_RawObject_Body] = 0
				} else {
					for i, element := range qw.queryParams.Elements() {
						pathToIdx[element.Path().Name()] = i
					}
				}
				return qw.rowsProcessor.SendAsync(rowsWorkpiece{
					object: object,
					outputRow: &outputRow{
						keyToIdx: pathToIdx,
						values:   make([]interface{}, len(pathToIdx)),
					},
					enrichedRootFieldsKinds: make(map[string]appdef.DataKind),
				})
			}
			return nil
		}),
		operator("create state", func(ctx context.Context, qw *queryWork) (err error) {
			qw.state = stateprovide.ProvideQueryProcessorStateFactory()(
				qw.msg.RequestCtx(),
				func() istructs.IAppStructs { return qw.appStructs },
				state.SimplePartitionIDFunc(qw.msg.Partition()),
				state.SimpleWSIDFunc(qw.msg.WSID()),
				qw.secretReader,
				func() []iauthnz.Principal { return qw.principals },
				func() string { return qw.msg.Token() },
				itokens,
				func() istructs.PrepareArgs { return qw.execQueryArgs.PrepareArgs },
				func() istructs.IObject { return qw.execQueryArgs.ArgumentObject },
				func() istructs.IObjectBuilder {
					return qw.appStructs.ObjectBuilder(qw.resultType.QName())
				},
				federation,
				func() istructs.ExecQueryCallback {
					return qw.callbackFunc
				},
				stateOpts,
				httpClient,
			)
			qw.execQueryArgs.State = qw.state
			qw.execQueryArgs.Intents = qw.state
			return
		}),
		operator("validate: get result type", func(ctx context.Context, qw *queryWork) (err error) {
			qw.resultType = qw.iQuery.Result()
			if qw.resultType == nil || qw.resultType.QName() != appdef.QNameANY {
				return nil
			}
			// ANY -> exact type according to PrepareArgs
			iResource := qw.appStructs.Resources().QueryResource(qw.msg.QName())
			var iQueryFunc istructs.IQueryFunction
			if iResource.Kind() != istructs.ResourceKind_null {
				iQueryFunc = iResource.(istructs.IQueryFunction)
			} else {
				for _, qry := range statelessResources.Queries {
					if qry.QName() == qw.msg.QName() {
						iQueryFunc = qry
						break
					}
				}
			}
			qNameResultType := iQueryFunc.ResultType(qw.execQueryArgs.PrepareArgs)

			ws := qw.iWorkspace
			qw.resultType = ws.Type(qNameResultType)
			if qw.resultType.Kind() == appdef.TypeKind_null {
				return coreutils.NewHTTPError(http.StatusBadRequest, fmt.Errorf("%s query result type %s does not exist in %v", qw.iQuery.QName(), qNameResultType, ws))
			}
			return nil
		}),
		operator("validate: get query params", func(ctx context.Context, qw *queryWork) (err error) {
			qw.queryParams, err = newQueryParams(qw.requestData, NewElement, NewFilter, NewOrderBy, newFieldsKinds(qw.resultType), qw.resultType)
			return coreutils.WrapSysError(err, http.StatusBadRequest)
		}),
		operator("authorize actual sys.Any result", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.iQuery.Result() != appdef.AnyType {
				// will authorize result only if result is sys.Any
				// otherwise each field is considered as allowed if EXECUTE ON QUERY is allowed
				return nil
			}
			ws := qw.iWorkspace
			for _, elem := range qw.queryParams.Elements() {
				nestedPath := elem.Path().AsArray()
				nestedType := qw.resultType
				for _, nestedName := range nestedPath {
					if len(nestedName) == 0 {
						// root
						continue
					}
					// incorrectness is excluded already on validation stage in [queryParams.validate]
					containersOfNested := nestedType.(appdef.IWithContainers)
					// container presence is checked already on validation stage in [queryParams.validate]
					nestedContainer := containersOfNested.Container(nestedName)
					nestedType = nestedContainer.Type()
				}
				requestedfields := []string{}
				for _, resultField := range elem.ResultFields() {
					requestedfields = append(requestedfields, resultField.Field())
				}

				// TODO: temporary do not check if allowed or not. Deny -> just verbose log, then collect denies,
				// investigate and implement according grants in vsql
				// see https://github.com/voedger/voedger/issues/3223
				_, err := qw.appPart.IsOperationAllowed(ws, appdef.OperationKind_Select, nestedType.QName(), requestedfields, qw.roles)
				if err != nil {
					return err
				}
			}
			return nil
		}),

		operator("build rows processor", func(ctx context.Context, qw *queryWork) error {
			now := time.Now()
			defer func() {
				qw.metrics.Increase(Metric_BuildSeconds, time.Since(now).Seconds())
			}()
			qw.rowsProcessor, qw.responseWriterGetter = ProvideRowsProcessorFactory()(qw.msg.RequestCtx(), qw.appStructs.AppDef(),
				qw.state, qw.queryParams, qw.resultType, qw.msg.Responder(), qw.metrics, qw.rowsProcessorErrCh)
			return nil
		}),
	}
	return pipeline.NewSyncPipeline(requestCtx, "Query Processor", ops[0], ops[1:]...)
}

type queryWork struct {
	// input
	msg      IQueryMessage
	appParts appparts.IAppPartitions
	// work
	requestData          map[string]interface{}
	state                state.IHostState
	queryParams          IQueryParams
	appPart              appparts.IAppPartition
	appStructs           istructs.IAppStructs
	resultType           appdef.IType
	execQueryArgs        istructs.ExecQueryArgs
	maxPrepareQueries    int
	rowsProcessor        pipeline.IAsyncPipeline
	rowsProcessorErrCh   chan error // will contain the first error from rowProcessor if any. The rest of errors in rowsProcessor will be just logged
	metrics              IMetrics
	principals           []iauthnz.Principal
	roles                []appdef.QName
	secretReader         isecrets.ISecretReader
	iWorkspace           appdef.IWorkspace
	iQuery               appdef.IQuery
	wsDesc               istructs.IRecord
	callbackFunc         istructs.ExecQueryCallback
	responseWriterGetter func() bus.IResponseWriter
}

func newQueryWork(msg IQueryMessage, appParts appparts.IAppPartitions,
	maxPrepareQueries int, metrics *queryProcessorMetrics, secretReader isecrets.ISecretReader) *queryWork {
	return &queryWork{
		msg:                msg,
		appParts:           appParts,
		requestData:        make(map[string]interface{}),
		maxPrepareQueries:  maxPrepareQueries,
		metrics:            metrics,
		secretReader:       secretReader,
		rowsProcessorErrCh: make(chan error, 1),
	}
}

// need for q.sys.EnrichPrincipalToken
func (qw *queryWork) GetPrincipals() []iauthnz.Principal {
	return qw.principals
}

// need for various funcs of sys package
func (qw *queryWork) GetAppStructs() istructs.IAppStructs {
	return qw.appStructs
}

// borrows app partition for query
func (qw *queryWork) borrow() (err error) {
	if qw.appPart, err = qw.appParts.Borrow(qw.msg.AppQName(), qw.msg.Partition(), appparts.ProcessorKind_Query); err != nil {
		return err
	}
	qw.appStructs = qw.appPart.AppStructs()
	return nil
}

// releases borrowed app partition
func (qw *queryWork) Release() {
	if ap := qw.appPart; ap != nil {
		qw.appStructs = nil
		qw.appPart = nil
		ap.Release()
	}
}

// need for q.sys.EnrichPrincipalToken
func (qw *queryWork) AppQName() appdef.AppQName {
	return qw.msg.AppQName()
}

// need for sqlquery
func (qw *queryWork) AppPartitions() appparts.IAppPartitions {
	return qw.appParts
}

// need for q.sys.SqlQuery to authnz the result
func (qw *queryWork) AppPartition() appparts.IAppPartition {
	return qw.appPart
}

// need for q.sys.SqlQuery to authnz the result
func (qw *queryWork) Roles() []appdef.QName {
	return qw.roles
}

func (qw *queryWork) SetPrincipals(prns []iauthnz.Principal) {
	qw.principals = prns
}

func borrowAppPart(_ context.Context, qw *queryWork) error {
	switch err := qw.borrow(); {
	case err == nil:
		return nil
	case errors.Is(err, appparts.ErrNotAvailableEngines), errors.Is(err, appparts.ErrNotFound): // partition is not deployed yet -> ErrNotFound
		return coreutils.WrapSysError(err, http.StatusServiceUnavailable)
	default:
		return coreutils.WrapSysError(err, http.StatusBadRequest)
	}
}

func operator(name string, doSync func(ctx context.Context, qw *queryWork) (err error)) *pipeline.WiredOperator {
	return pipeline.WireFunc(name, doSync)
}

type queryMessage struct {
	requestCtx context.Context
	appQName   appdef.AppQName
	wsid       istructs.WSID
	partition  istructs.PartitionID
	responder  bus.IResponder
	body       []byte
	qName      appdef.QName
	host       string
	token      string
}

func (m queryMessage) AppQName() appdef.AppQName { return m.appQName }
func (m queryMessage) WSID() istructs.WSID       { return m.wsid }
func (m queryMessage) Responder() bus.IResponder {
	return m.responder
}
func (m queryMessage) RequestCtx() context.Context     { return m.requestCtx }
func (m queryMessage) QName() appdef.QName             { return m.qName }
func (m queryMessage) Host() string                    { return m.host }
func (m queryMessage) Token() string                   { return m.token }
func (m queryMessage) Partition() istructs.PartitionID { return m.partition }
func (m queryMessage) Body() []byte {
	if len(m.body) != 0 {
		return m.body
	}
	return []byte("{}")
}

func NewQueryMessage(requestCtx context.Context, appQName appdef.AppQName, partID istructs.PartitionID, wsid istructs.WSID,
	responder bus.IResponder, body []byte, qName appdef.QName, host string, token string) IQueryMessage {
	return queryMessage{
		appQName:   appQName,
		wsid:       wsid,
		partition:  partID,
		responder:  responder,
		body:       body,
		requestCtx: requestCtx,
		qName:      qName,
		host:       host,
		token:      token,
	}
}

type rowsWorkpiece struct {
	pipeline.IWorkpiece
	object                  istructs.IObject
	outputRow               IOutputRow
	enrichedRootFieldsKinds FieldsKinds
}

func (w rowsWorkpiece) Object() istructs.IObject             { return w.object }
func (w rowsWorkpiece) OutputRow() IOutputRow                { return w.outputRow }
func (w rowsWorkpiece) EnrichedRootFieldsKinds() FieldsKinds { return w.enrichedRootFieldsKinds }
func (w rowsWorkpiece) PutEnrichedRootFieldKind(name string, kind appdef.DataKind) {
	w.enrichedRootFieldsKinds[name] = kind
}
func (w rowsWorkpiece) Release() {
	// TODO implement it someday
	// Release goes here
}

type outputRow struct {
	keyToIdx map[string]int
	values   []interface{}
}

func (r *outputRow) Set(alias string, value interface{}) {
	r.values[r.keyToIdx[alias]] = value
}
func (r *outputRow) Values() []interface{}          { return r.values }
func (r *outputRow) Value(alias string) interface{} { return r.values[r.keyToIdx[alias]] }
func (r *outputRow) MarshalJSON() ([]byte, error)   { return json.Marshal(r.values) }

func newExecQueryArgs(data coreutils.MapObject, wsid istructs.WSID, qw *queryWork) (execQueryArgs istructs.ExecQueryArgs, err error) {
	args, _, err := data.AsObject("args")
	if err != nil {
		return execQueryArgs, err
	}
	argsType := qw.iQuery.Param()
	requestArgs := istructs.NewNullObject()
	if argsType != nil {
		requestArgsBuilder := qw.appStructs.ObjectBuilder(argsType.QName())
		requestArgsBuilder.FillFromJSON(args)
		requestArgs, err = requestArgsBuilder.Build()
		if err != nil {
			return execQueryArgs, err
		}
	}
	return istructs.ExecQueryArgs{
		PrepareArgs: istructs.PrepareArgs{
			ArgumentObject: requestArgs,
			WSID:           wsid,
			Workpiece:      qw,
			Workspace:      qw.iWorkspace,
		},
	}, nil
}

type path []string

func (p path) IsRoot() bool      { return p[0] == rootDocument }
func (p path) Name() string      { return strings.Join(p, "/") }
func (p path) AsArray() []string { return p }

type element struct {
	path   path
	fields []IResultField
	refs   []IRefField
}

func (e element) NewOutputRow() IOutputRow {
	fields := make([]string, 0, len(e.fields)+len(e.refs))
	for _, field := range e.fields {
		fields = append(fields, field.Field())
	}
	for _, field := range e.refs {
		fields = append(fields, field.Key())
	}
	fieldToIdx := make(map[string]int)
	for j, field := range fields {
		fieldToIdx[field] = j
	}
	return &outputRow{
		keyToIdx: fieldToIdx,
		values:   make([]interface{}, len(fieldToIdx)),
	}
}

func (e element) Path() IPath                  { return e.path }
func (e element) ResultFields() []IResultField { return e.fields }
func (e element) RefFields() []IRefField       { return e.refs }

type fieldsDefs struct {
	appDef appdef.IAppDef
	fields map[appdef.QName]FieldsKinds
	lock   sync.Mutex
}

func newFieldsDefs(appDef appdef.IAppDef) *fieldsDefs {
	return &fieldsDefs{
		appDef: appDef,
		fields: make(map[appdef.QName]FieldsKinds),
	}
}

func (c *fieldsDefs) get(name appdef.QName) FieldsKinds {
	c.lock.Lock()
	defer c.lock.Unlock()
	fd, ok := c.fields[name]
	if !ok {
		fd = newFieldsKinds(c.appDef.Type(name))
		c.fields[name] = fd
	}
	return fd
}

type queryProcessorMetrics struct {
	vvm     string
	app     appdef.AppQName
	metrics imetrics.IMetrics
}

func (m *queryProcessorMetrics) Increase(metricName string, valueDelta float64) {
	m.metrics.IncreaseApp(metricName, m.vvm, m.app, valueDelta)
}

func newFieldsKinds(t appdef.IType) FieldsKinds {
	res := FieldsKinds{}
	if fields, ok := t.(appdef.IWithFields); ok {
		for _, f := range fields.Fields() {
			res[f.Name()] = f.DataKind()
		}
	}
	return res
}
