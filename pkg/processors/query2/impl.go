/*
 * Copyright (c) 2025-present unTill Software Development Group B.V.
 * @author Michael Saigachenko
 */
package query2

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
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
	queryprocessor "github.com/voedger/voedger/pkg/processors/query"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/state/stateprovide"
	"github.com/voedger/voedger/pkg/sys/authnz"
)

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
				qpm.Increase(queryprocessor.Metric_QueriesTotal, 1.0)
				qwork := newQueryWork(msg, appParts, maxPrepareQueries, qpm, secretReader, federation)
				func() { // borrowed application partition should be guaranteed to be freed
					defer qwork.Release()
					if p == nil {
						p = newQueryProcessorPipeline(ctx, authn, itokens, federation, statelessResources, stateOpts, httpClient)
					}
					err := p.SendSync(qwork)
					if err != nil {
						qpm.Increase(queryprocessor.Metric_ErrorsTotal, 1.0)
						p.Close()
						p = nil
					} else {
						now := time.Now()
						if qwork.apiPathHandler.exec != nil {
							err = qwork.apiPathHandler.exec(ctx, qwork)
						}
						qwork.metrics.Increase(queryprocessor.Metric_ExecSeconds, time.Since(now).Seconds())
						if err == nil {
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
					if qwork.apiPathHandler.isArrayResult {
						if qwork.responseWriterGetter == nil || qwork.responseWriterGetter() == nil {
							// have an error before 200ok is sent -> send the status from the actual error
							respWriter = msg.Responder().StreamJSON(statusCode)
						} else {
							respWriter = qwork.responseWriterGetter()
						}
						respWriter.Close(err)
					} else if err != nil {
						respondErr := qwork.msg.Responder().Respond(bus.ResponseMeta{ContentType: httpu.ContentType_ApplicationJSON, StatusCode: statusCode}, err)
						if respondErr != nil { // notest
						}
					}
				}()
				metrics.IncreaseApp(queryprocessor.Metric_QueriesSeconds, vvm, msg.AppQName(), time.Since(now).Seconds())
			case <-ctx.Done():
			}
		}
		if p != nil {
			p.Close()
		}
	})
}

// IStatelessResources need only for determine the exact result type of ANY
func newQueryProcessorPipeline(requestCtx context.Context, authn iauthnz.IAuthenticator,
	itokens itokens.ITokens, federation federation.IFederation, statelessResources istructsmem.IStatelessResources,
	stateOpts state.StateOpts, httpClient httpu.IHTTPClient) pipeline.ISyncPipeline {
	ops := []*pipeline.WiredOperator{
		operator("get api path handler", func(ctx context.Context, qw *queryWork) (err error) {
			switch qw.msg.APIPath() {
			case processors.APIPath_Queries:
				qw.apiPathHandler = queryHandler()
			case processors.APIPath_Views:
				qw.apiPathHandler = viewHandler()
			case processors.APIPath_Docs:
				// [~server.apiv2.docs/cmp.provideDocsHandler~impl]
				qw.apiPathHandler = docsHandler()
			case processors.APIPaths_Schema:
				qw.apiPathHandler = schemasHandler()
			case processors.APIPath_Schemas_WorkspaceRoles:
				qw.apiPathHandler = schemasRolesHandler()
			case processors.APIPath_Schemas_WorkspaceRole:
				// [~server.apiv2.role/cmp.provideSchemasRoleHandler~impl]
				qw.apiPathHandler = schemasRoleHandler()
			case processors.APIPath_CDocs:
				// [~server.apiv2.docs/cmp.provideCDocsHandler~impl]
				qw.apiPathHandler = cdocsHandler()
			case processors.APIPath_Auth_Login:
				// [~server.authnz/cmp.provideAuthLoginHandler~impl]
				qw.apiPathHandler = authLoginHandler()
			case processors.APIPath_Auth_Refresh:
				// [~server.authnz/cmp.provideAuthRefreshHandler~impl]
				qw.apiPathHandler = authRefreshHandler()
			default:
				return coreutils.NewHTTPErrorf(http.StatusBadRequest, fmt.Sprintf("unsupported api path %v", qw.msg.APIPath()))
			}
			return nil
		}),
		operator("borrowAppPart", borrowAppPart),
		operator("check rate limit", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.apiPathHandler.checkRateLimit == nil {
				return nil
			}
			return qw.apiPathHandler.checkRateLimit(ctx, qw)
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
			qw.principals, qw.profileWSID, err = authn.Authenticate(qw.msg.RequestCtx(), qw.appStructs, qw.appStructs.AppTokens(), req)
			return coreutils.WrapSysError(err, http.StatusUnauthorized)
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
		operator("set request type (view, query etc)", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.apiPathHandler.setRequestType == nil {
				return nil
			}
			return qw.apiPathHandler.setRequestType(ctx, qw)
		}),
		operator("authorize query request", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.apiPathHandler.requestOpKind == appdef.OperationKind_null {
				return nil
			}
			ok, err := qw.appPart.IsOperationAllowed(qw.iWorkspace, qw.apiPathHandler.requestOpKind, qw.msg.QName(), nil, qw.roles)
			if err != nil {
				return err
			}
			if !ok {
				return coreutils.WrapSysError(errors.New(""), http.StatusForbidden)
			}
			return nil
		}),
		operator("validate: get exec query args", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.msg.APIPath() == processors.APIPath_Queries {
				qw.execQueryArgs, err = newExecQueryArgs(qw.msg.WSID(), qw)
			}
			return coreutils.WrapSysError(err, http.StatusBadRequest)
		}),
		operator("create callback func", func(ctx context.Context, qw *queryWork) (err error) {
			qw.callbackFunc = func(o istructs.IObject) (err error) {
				if _, ok := o.(objectBackedByMap); !ok {
					o = objectBackedByMap{
						data: coreutils.FieldsToMap(queryResultWrapper{
							IObject: o,
							qName:   qw.appStructs.AppDef().Type(qw.iQuery.QName()).(appdef.IQuery).Result().QName(),
						},
							qw.appStructs.AppDef(),
							coreutils.WithAllFields(), // we do not know which fields are specified because `o` is different on each query -> read all fields of the result
							coreutils.Filter(func(name string, kind appdef.DataKind) bool {
								// system fields like sys.QName, sys.Container are emitted due of WithAllFields
								// skip them in the query result
								return !appdef.IsSysField(name)
							}),
						),
					}
				}
				return qw.rowsProcessor.SendAsync(o.(pipeline.IWorkpiece))
			}
			return
		}),
		operator("create state", func(ctx context.Context, qw *queryWork) (err error) {
			qw.state = stateprovide.ProvideQueryProcessorStateFactory()(
				qw.msg.RequestCtx(),
				func() istructs.IAppStructs { return qw.appStructs },
				state.SimplePartitionIDFunc(qw.msg.PartitionID()),
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
			if qw.apiPathHandler.setResultType == nil {
				return nil
			}
			return qw.apiPathHandler.setResultType(ctx, qw, statelessResources)
		}),
		operator("authorize actual sys.Any result", func(ctx context.Context, qw *queryWork) (err error) {
			if qw.apiPathHandler.authorizeResult == nil {
				return nil
			}
			err = qw.apiPathHandler.authorizeResult(ctx, qw)
			if errors.Is(err, appdef.ErrNotFoundError) {
				// TODO: temporary solution. Such case must not happen after https://github.com/voedger/voedger/issues/3522
				// to be eliminated in https://github.com/voedger/voedger/issues/4003
				err = coreutils.WrapSysError(err, http.StatusBadRequest)
			}
			return err
		}),
		operator("build rows processor", func(ctx context.Context, qw *queryWork) error {
			now := time.Now()
			defer func() {
				qw.metrics.Increase(queryprocessor.Metric_BuildSeconds, time.Since(now).Seconds())
			}()
			if qw.apiPathHandler.rowsProcessor == nil {
				return nil
			}
			return qw.apiPathHandler.rowsProcessor(ctx, qw)
		}),
	}
	return pipeline.NewSyncPipeline(requestCtx, "Query Processor", ops[0], ops[1:]...)
}

func newExecQueryArgs(wsid istructs.WSID, qw *queryWork) (execQueryArgs istructs.ExecQueryArgs, err error) {
	argsType := qw.iQuery.Param()
	requestArgs := istructs.NewNullObject()
	if argsType != nil {
		requestArgsBuilder := qw.appStructs.ObjectBuilder(argsType.QName())
		requestArgsBuilder.FillFromJSON(qw.msg.QueryParams().Argument)
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
func getCombinations(arrays [][]interface{}) [][]interface{} {
	if len(arrays) == 0 {
		return [][]interface{}{}
	}
	return combine(arrays, 0)
}

func combine(arrays [][]interface{}, index int) [][]interface{} {
	if index == len(arrays) {
		return [][]interface{}{{}}
	}
	subCombinations := combine(arrays, index+1)
	var result [][]interface{}
	for _, elem := range arrays[index] {
		for _, subComb := range subCombinations {
			newComb := append([]interface{}{elem}, subComb...)
			result = append(result, newComb)
		}
	}
	return result
}

func splitPath(path string) (parts []string) {
	var current []rune
	inQuotes := false
	for _, r := range path {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case '.':
			if inQuotes {
				current = append(current, r)
			} else {
				parts = append(parts, string(current))
				current = current[:0]
			}
		default:
			current = append(current, r)
		}
	}
	parts = append(parts, string(current))
	return parts
}
