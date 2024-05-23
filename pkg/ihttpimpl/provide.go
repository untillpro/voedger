/*
 * Copyright (c) 2022-present Sigma-Soft, Ltd.
 * @author Aleksei Ponomarev
 */

package ihttpimpl

import (
	"net/http"
	"sync"

	"github.com/voedger/voedger/pkg/ihttp"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
	coreutils "github.com/voedger/voedger/pkg/utils"
	dbcertcache "github.com/voedger/voedger/pkg/vvm/db_cert_cache"
	"github.com/voedger/voedger/staging/src/github.com/untillpro/ibusmem"
)

func NewProcessor(params ihttp.CLIParams, routerStorage ihttp.IRouterStorage) (server ihttp.IHTTPProcessor, cleanup func()) {
	r := newRouter()
	routerAppStorage := istorage.IAppStorage(routerStorage)
	httpProcessor := &httpProcessor{
		params:      params,
		router:      r,
		certCache:   dbcertcache.ProvideDbCache(&routerAppStorage),
		acmeDomains: &sync.Map{},
		server: &http.Server{
			Addr:              coreutils.ServerAddress(params.Port),
			Handler:           r,
			ReadHeaderTimeout: defaultReadHeaderTimeout,
		},
		apps:               make(map[istructs.AppQName]*appInfo),
		numsAppsWorkspaces: make(map[istructs.AppQName]istructs.NumAppWorkspaces),
	}
	httpProcessor.bus = ibusmem.Provide(httpProcessor.requestHandler)
	if len(params.AcmeDomains) > 0 {
		for _, domain := range params.AcmeDomains {
			httpProcessor.AddAcmeDomain(domain)
		}
	}
	return httpProcessor, httpProcessor.cleanup
}
