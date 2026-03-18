/*
 * Copyright (c) 2020-present unTill Pro, Ltd.
 * @author Denis Gribanov
 */

package router

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme/autocert"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
)

type HTTPServerParams struct {
	Port             int
	WriteTimeout     int
	ReadTimeout      int
	ConnectionsLimit int
}

type RouterParams struct {
	HTTPServerParams
	AdminPort            int
	HTTP01ChallengeHosts []string
	CertDir              string
	RouteDefault         string            // http://10.0.0.3:3000/not-found : https://alpha.dev.untill.ru/unknown/foo -> http://10.0.0.3:3000/not-found/unknown/foo
	Routes               map[string]string // /grafana=http://10.0.0.3:3000 : https://alpha.dev.untill.ru/grafana/foo -> http://10.0.0.3:3000/grafana/foo
	RoutesRewrite        map[string]string // /grafana-rewrite=http://10.0.0.3:3000/rewritten : https://alpha.dev.untill.ru/grafana-rewrite/foo -> http://10.0.0.3:3000/rewritten/foo
	RouteDomains         map[string]string // resellerportal.dev.untill.ru=http://resellerportal : https://resellerportal.dev.untill.ru/foo -> http://resellerportal/foo
}

type httpServer struct {
	HTTPServerParams
	listenAddress string
	server        *http.Server
	listener      net.Listener
	name          string
	listeningPort atomic.Uint32
	rootLogCtx    context.Context // initialized on Run()
}

type routerService struct {
	httpServer
	routeDefault       string
	routes             map[string]string
	routesRewrite      map[string]string
	routeDomains       map[string]string
	router             *mux.Router
	n10n               in10n.IN10nBroker
	requestSender      bus.IRequestSender
	numsAppsWorkspaces map[appdef.AppQName]istructs.NumAppWorkspaces
	blobRequestHandler blobprocessor.IRequestHandler
	iTokens            itokens.ITokens
	federation         federation.IFederation
	appTokensFactory   payloads.IAppTokensFactory
}

type httpsService struct {
	*routerService
	crtMgr *autocert.Manager
}

type acmeService struct {
	httpServer
	handler http.Handler
}

type route struct {
	targetURL  *url.URL
	isRewrite  bool
	fromDomain string
}

type subscriberParamsType struct {
	Channel       in10n.ChannelID
	ProjectionKey []in10n.ProjectionKey
}

type SubscriptionJSON struct {
	Entity     string      `json:"entity"`
	WSIDNumber json.Number `json:"wsid"`
}

type N10nArgs struct {
	Subscriptions    []SubscriptionJSON `json:"subscriptions"`
	ExpiresInSeconds int64              `json:"expiresIn"`
}

// validatedData contains validated data from HTTP request
type validatedData struct {
	vars     map[string]string
	wsid     istructs.WSID
	appQName appdef.AppQName
	header   map[string]string
	body     []byte
}

type validatorFunc func(validateData validatedData, req *http.Request) (validatedData, error)
