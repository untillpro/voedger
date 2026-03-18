/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package router

import (
	"fmt"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
	"golang.org/x/crypto/acme/autocert"

	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
)

// port == 443 -> httpsService + ACMEService, otherwise -> HTTPService only, ACMEService is nil
// where is VVM RequestHandler? bus.RequestHandler
func Provide(rp RouterParams, broker in10n.IN10nBroker, blobRequestHandler blobprocessor.IRequestHandler, autocertCache autocert.Cache,
	requestSender bus.IRequestSender, numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, iTokens itokens.ITokens,
	federation federation.IFederation, appTokensFactory payloads.IAppTokensFactory) (httpSrv IHTTPService, acmeSrv IACMEService, adminSrv IAdminService) {
	httpServ := getRouterService("sys._HTTPServer", httpu.ListenAddr(rp.Port), rp, broker, blobRequestHandler,
		requestSender, numsAppsWorkspaces, iTokens, federation, appTokensFactory)
	adminEndpoint := fmt.Sprintf("%s:%d", httpu.LocalhostIP, rp.AdminPort)
	adminSrv = getRouterService("sys._AdminHTTPServer", adminEndpoint, RouterParams{
		HTTPServerParams: HTTPServerParams{
			WriteTimeout:     rp.WriteTimeout,
			ReadTimeout:      rp.ReadTimeout,
			ConnectionsLimit: rp.ConnectionsLimit,
		},
	}, broker, nil, requestSender, numsAppsWorkspaces, iTokens, federation, appTokensFactory)

	if rp.Port != HTTPSPort {
		return httpServ, nil, adminSrv
	}
	crtMgr := &autocert.Manager{
		/*
			If we need to test issuance of big amount of certficates for different domains then need to use test perimeter of the enterprise.
			Need to redefine DirectoryURL in Manager at:
			https://acme-staging-v02.api.letsencrypt.org/directory :
			Client: &acme.Client{
				DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
			},
			that is because there are quotas for certificate issuace: per domains amount, per amount of certificates per time etc
		*/
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(rp.HTTP01ChallengeHosts...),
		Cache:      autocertCache,
	}
	if crtMgr.Cache == nil {
		crtMgr.Cache = autocert.DirCache(rp.CertDir)
	}
	httpServ.name = "sys._HTTPSServer"
	httpsService := &httpsService{
		routerService: httpServ,
		crtMgr:        crtMgr,
	}

	// handle Lets Encrypt callback over 80 port - only port 80 allowed
	acmeService := &acmeService{
		httpServer: getHTTPServer("sys._ACMEServer", ":80", HTTPServerParams{
			WriteTimeout: int(DefaultACMEServerWriteTimeout.Seconds()),
			ReadTimeout:  int(DefaultACMEServerReadTimeout.Seconds()),
		}),
		handler: crtMgr.HTTPHandler(nil),
	}
	return httpsService, acmeService, adminSrv
}

func getRouterService(name string, listenAddress string, rp RouterParams, broker in10n.IN10nBroker,
	blobRequestHandler blobprocessor.IRequestHandler, requestSender bus.IRequestSender,
	numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces, iTokens itokens.ITokens,
	federation federation.IFederation, appTokensFactory payloads.IAppTokensFactory) *routerService {
	return &routerService{
		httpServer:         getHTTPServer(name, listenAddress, rp.HTTPServerParams),
		routeDefault:       rp.RouteDefault,
		routes:             rp.Routes,
		routesRewrite:      rp.RoutesRewrite,
		routeDomains:       rp.RouteDomains,
		n10n:               broker,
		requestSender:      requestSender,
		numsAppsWorkspaces: numsAppsWorkspaces,
		blobRequestHandler: blobRequestHandler,
		iTokens:            iTokens,
		federation:         federation,
		appTokensFactory:   appTokensFactory,
	}
}

func getHTTPServer(name string, listenAddress string, params HTTPServerParams) httpServer {
	return httpServer{
		HTTPServerParams: params,
		listenAddress:    listenAddress,
		name:             name,
	}
}
