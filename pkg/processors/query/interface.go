/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 *
 * * @author Michael Saigachenko
 */

package queryprocessor

import (
	"context"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/utils/federation"
)

// RowsProcessorFactory is the function for building pipeline from query params and row meta
// The pipeline is used to process data fetched by QueryHandler
// TODO In my opinion we have to remove it from export
type RowsProcessorFactory func(ctx context.Context, appDef appdef.IAppDef, state istructs.IState,
	params IQueryParams, resultMeta appdef.IType, rs IResultSenderClosable, metrics IMetrics) pipeline.IAsyncPipeline

type ServiceFactory func(serviceChannel iprocbus.ServiceChannel, resultSenderClosableFactory ResultSenderClosableFactory,
	appParts appparts.IAppPartitions, maxPrepareQueries int, metrics imetrics.IMetrics, vvm string,
	authn iauthnz.IAuthenticator, authz iauthnz.IAuthorizer, itokens itokens.ITokens, federation federation.IFederation,
	statelessResources istructsmem.IStatelessResources) pipeline.IService
