// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package main

import (
	"github.com/voedger/voedger/pkg/apps"
	"github.com/voedger/voedger/pkg/ihttp"
	"github.com/voedger/voedger/pkg/ihttpctl"
	"github.com/voedger/voedger/pkg/ihttpimpl"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/provider"
)

import (
	_ "embed"
)

// Injectors from wire.go:

func wireServer(httpCliParams ihttp.CLIParams, appsCliParams apps.CLIParams) (WiredServer, func(), error) {
	iAppStorageFactory, err := apps.NewAppStorageFactory(appsCliParams)
	if err != nil {
		return WiredServer{}, nil, err
	}
	iAppStorageInitializer := provideAppStorageInitializer(iAppStorageFactory)
	iRouterStorage, err := ihttp.NewIRouterStorage(iAppStorageInitializer)
	if err != nil {
		return WiredServer{}, nil, err
	}
	ihttpProcessor, cleanup := ihttpimpl.NewProcessor(httpCliParams, iRouterStorage)
	v := apps.NewStaticEmbeddedResources()
	redirectRoutes := apps.NewRedirectionRoutes()
	defaultRedirectRoute := apps.NewDefaultRedirectionRoute()
	acmeDomains := httpCliParams.AcmeDomains
	appRequestHandlers := apps.NewAppRequestHandlers()
	ihttpProcessorController := ihttpctl.NewHTTPProcessorController(ihttpProcessor, v, redirectRoutes, defaultRedirectRoute, acmeDomains, appRequestHandlers)
	wiredServer := WiredServer{
		IHTTPProcessor:           ihttpProcessor,
		IHTTPProcessorController: ihttpProcessorController,
	}
	return wiredServer, func() {
		cleanup()
	}, nil
}

// wire.go:

// provideAppStorageInitializer is intended to be used by wire instead of istorage/provider.Provide, because wire can not handle variadic arguments
func provideAppStorageInitializer(appStorageFactory istorage.IAppStorageFactory) istorage.IAppStorageInitializer {
	return provider.Provide(appStorageFactory)
}
