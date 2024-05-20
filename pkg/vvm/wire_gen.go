// Code generated by Wire. DO NOT EDIT.

//go:generate go run github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package vvm

import (
	"context"
	"errors"
	"fmt"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/apppartsctl"
	"github.com/voedger/voedger/pkg/apps"
	"github.com/voedger/voedger/pkg/btstrp"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iauthnz"
	"github.com/voedger/voedger/pkg/iauthnzimpl"
	"github.com/voedger/voedger/pkg/iblobstoragestg"
	"github.com/voedger/voedger/pkg/in10n"
	"github.com/voedger/voedger/pkg/in10nmem"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/iprocbusmem"
	"github.com/voedger/voedger/pkg/irates"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/isecrets"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/provider"
	"github.com/voedger/voedger/pkg/istoragecache"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	"github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/processors/command"
	"github.com/voedger/voedger/pkg/processors/query"
	"github.com/voedger/voedger/pkg/projectors"
	"github.com/voedger/voedger/pkg/router"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/invite"
	"github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/utils/federation"
	"github.com/voedger/voedger/pkg/vvm/db_cert_cache"
	"github.com/voedger/voedger/pkg/vvm/engines"
	"github.com/voedger/voedger/pkg/vvm/metrics"
	"github.com/voedger/voedger/staging/src/github.com/untillpro/airs-ibus"
	"golang.org/x/crypto/acme/autocert"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Injectors from provide.go:

// vvmCtx must be cancelled by the caller right before vvm.ServicePipeline.Close()
func ProvideCluster(vvmCtx context.Context, vvmConfig *VVMConfig, vvmIdx VVMIdxType) (*VVM, func(), error) {
	numCommandProcessors := vvmConfig.NumCommandProcessors
	v := provideChannelGroups(vvmConfig)
	iProcBus := iprocbusmem.Provide(v)
	serviceChannelFactory := provideServiceChannelFactory(vvmConfig, iProcBus)
	commandChannelFactory := provideCommandChannelFactory(serviceChannelFactory)
	appConfigsTypeEmpty := provideAppConfigsTypeEmpty()
	timeFunc := vvmConfig.TimeFunc
	bucketsFactoryType := provideBucketsFactory(timeFunc)
	iSecretReader := vvmConfig.SecretsReader
	secretKeyType, err := provideSecretKeyJWT(iSecretReader)
	if err != nil {
		return nil, nil, err
	}
	iTokens := itokensjwt.ProvideITokens(secretKeyType, timeFunc)
	iAppTokensFactory := payloads.ProvideIAppTokensFactory(iTokens)
	storageCacheSizeType := vvmConfig.StorageCacheSize
	iMetrics := imetrics.Provide()
	vvmName := vvmConfig.Name
	iAppStorageFactory, err := provideStorageFactory(vvmConfig)
	if err != nil {
		return nil, nil, err
	}
	iAppStorageUncachingProviderFactory := provideIAppStorageUncachingProviderFactory(iAppStorageFactory, vvmConfig)
	iAppStorageProvider := provideCachingAppStorageProvider(storageCacheSizeType, iMetrics, vvmName, iAppStorageUncachingProviderFactory)
	iAppStructsProvider := provideIAppStructsProvider(appConfigsTypeEmpty, bucketsFactoryType, iAppTokensFactory, iAppStorageProvider)
	syncActualizerFactory := projectors.ProvideSyncActualizerFactory()
	quotas := vvmConfig.Quotas
	in10nBroker, cleanup := in10nmem.ProvideEx2(quotas, timeFunc)
	v2 := projectors.NewSyncActualizerFactoryFactory(syncActualizerFactory, iSecretReader, in10nBroker)
	vvmPortSource := provideVVMPortSource()
	iFederation, cleanup2 := provideIFederation(vvmConfig, vvmPortSource)
	apIs := apps.APIs{
		ITokens:             iTokens,
		IAppStructsProvider: iAppStructsProvider,
		IAppStorageProvider: iAppStorageProvider,
		IAppTokensFactory:   iAppTokensFactory,
		IFederation:         iFederation,
		TimeFunc:            timeFunc,
	}
	appsArtefacts, err := provideBuiltInAppsArtefacts(vvmConfig, apIs, appConfigsTypeEmpty)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	iAppPartitions, cleanup3, err := provideAppPartitions(iAppStructsProvider, v2, appsArtefacts)
	if err != nil {
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	v3 := provideSubjectGetterFunc()
	isDeviceAllowedFuncs := provideIsDeviceAllowedFunc(appsArtefacts)
	iAuthenticator := iauthnzimpl.NewDefaultAuthenticator(v3, isDeviceAllowedFuncs)
	iAuthorizer := iauthnzimpl.NewDefaultAuthorizer()
	serviceFactory := commandprocessor.ProvideServiceFactory(iAppPartitions, timeFunc, in10nBroker, iMetrics, vvmName, iAuthenticator, iAuthorizer, iSecretReader)
	operatorCommandProcessors := provideCommandProcessors(numCommandProcessors, commandChannelFactory, serviceFactory)
	numQueryProcessors := vvmConfig.NumQueryProcessors
	queryChannel := provideQueryChannel(serviceChannelFactory)
	queryprocessorServiceFactory := queryprocessor.ProvideServiceFactory()
	maxPrepareQueriesType := vvmConfig.MaxPrepareQueries
	operatorQueryProcessors := provideQueryProcessors(numQueryProcessors, queryChannel, iAppPartitions, queryprocessorServiceFactory, iMetrics, vvmName, maxPrepareQueriesType, iAuthenticator, iAuthorizer, iTokens, iFederation)
	asyncActualizerFactory := projectors.ProvideAsyncActualizerFactory()
	asyncActualizersFactory := provideAsyncActualizersFactory(iAppPartitions, iAppStructsProvider, in10nBroker, asyncActualizerFactory, iSecretReader, iMetrics)
	v4 := vvmConfig.ActualizerStateOpts
	appPartitionFactory := provideAppPartitionFactory(asyncActualizersFactory, v4)
	appServiceFactory := provideAppServiceFactory(appPartitionFactory)
	operatorAppServicesFactory := provideOperatorAppServices(appServiceFactory, appsArtefacts, iAppStructsProvider)
	iAppPartitionsController, cleanup4, err := apppartsctl.New(iAppPartitions)
	if err != nil {
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	iAppPartsCtlPipelineService := provideAppPartsCtlPipelineService(iAppPartitionsController)
	v5 := provideBuiltInApps(appsArtefacts)
	blobAppStoragePtr := provideBlobAppStoragePtr(iAppStorageProvider)
	routerAppStoragePtr := provideRouterAppStoragePtr(iAppStorageProvider)
	bootstrapOperator, err := provideBootstrapOperator(iFederation, iAppStructsProvider, timeFunc, iAppPartitions, v5, iTokens, iAppStorageProvider, blobAppStoragePtr, routerAppStoragePtr)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	vvmPortType := vvmConfig.VVMPort
	routerParams := provideRouterParams(vvmConfig, vvmPortType, vvmIdx)
	busTimeout := vvmConfig.BusTimeout
	blobberServiceChannels := vvmConfig.BlobberServiceChannels
	blobMaxSizeType := vvmConfig.BLOBMaxSize
	blobStorage := provideBlobStorage(blobAppStoragePtr, timeFunc)
	cache := dbcertcache.ProvideDbCache(routerAppStoragePtr)
	commandProcessorsChannelGroupIdxType := provideProcessorChannelGroupIdxCommand(vvmConfig)
	queryProcessorsChannelGroupIdxType := provideProcessorChannelGroupIdxQuery(vvmConfig)
	vvmApps := provideVVMApps(v5)
	iBus := provideIBus(iAppPartitions, iProcBus, commandProcessorsChannelGroupIdxType, queryProcessorsChannelGroupIdxType, numCommandProcessors, vvmApps)
	v6, err := provideNumsAppsWorkspaces(vvmApps, iAppStructsProvider)
	if err != nil {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
		return nil, nil, err
	}
	routerServices := provideRouterServices(vvmCtx, routerParams, busTimeout, in10nBroker, quotas, timeFunc, blobberServiceChannels, blobMaxSizeType, blobStorage, cache, iBus, vvmPortSource, v6)
	adminEndpointServiceOperator := provideAdminEndpointServiceOperator(routerServices)
	metricsServicePortInitial := vvmConfig.MetricsServicePort
	metricsServicePort := provideMetricsServicePort(metricsServicePortInitial, vvmIdx)
	metricsService := metrics.ProvideMetricsService(vvmCtx, metricsServicePort, iMetrics)
	metricsServiceOperator := provideMetricsServiceOperator(metricsService)
	publicEndpointServiceOperator := providePublicEndpointServiceOperator(routerServices, metricsServiceOperator)
	servicePipeline := provideServicePipeline(vvmCtx, operatorCommandProcessors, operatorQueryProcessors, operatorAppServicesFactory, iAppPartsCtlPipelineService, bootstrapOperator, adminEndpointServiceOperator, publicEndpointServiceOperator)
	v7 := provideExtensionPoints(appsArtefacts)
	v8 := provideMetricsServicePortGetter(metricsService)
	v9 := provideBuiltInAppPackages(appsArtefacts)
	vvm := &VVM{
		ServicePipeline:     servicePipeline,
		APIs:                apIs,
		IAppPartitions:      iAppPartitions,
		AppsExtensionPoints: v7,
		MetricsServicePort:  v8,
		BuiltInAppsPackages: v9,
	}
	return vvm, func() {
		cleanup4()
		cleanup3()
		cleanup2()
		cleanup()
	}, nil
}

// provide.go:

func ProvideVVM(vvmCfg *VVMConfig, vvmIdx VVMIdxType) (voedgerVM *VoedgerVM, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	voedgerVM = &VoedgerVM{vvmCtxCancel: cancel}
	vvmCfg.addProcessorChannel(iprocbusmem.ChannelGroup{
		NumChannels:       int(vvmCfg.NumCommandProcessors),
		ChannelBufferSize: int(DefaultNumCommandProcessors),
	}, ProcessorChannel_Command,
	)

	vvmCfg.addProcessorChannel(iprocbusmem.ChannelGroup{
		NumChannels:       1,
		ChannelBufferSize: 0,
	}, ProcessorChannel_Query,
	)
	vvmCfg.Quotas = in10n.Quotas{
		Channels:                int(DefaultQuotasChannelsFactor * vvmCfg.NumCommandProcessors),
		ChannelsPerSubject:      DefaultQuotasChannelsPerSubject,
		Subscriptions:           int(DefaultQuotasSubscriptionsFactor * vvmCfg.NumCommandProcessors),
		SubscriptionsPerSubject: DefaultQuotasSubscriptionsPerSubject,
	}
	voedgerVM.VVM, voedgerVM.vvmCleanup, err = ProvideCluster(ctx, vvmCfg, vvmIdx)
	if err != nil {
		return nil, err
	}
	return voedgerVM, nil
}

func (vvm *VoedgerVM) Shutdown() {
	vvm.vvmCtxCancel()
	vvm.ServicePipeline.Close()
	vvm.vvmCleanup()
}

func (vvm *VoedgerVM) Launch() error {
	ignition := struct{}{}
	err := vvm.ServicePipeline.SendSync(ignition)
	if err != nil {
		err = errors.Join(err, ErrVVMLaunchFailure)
		logger.Error(err)
	}
	return err
}

func provideBootstrapOperator(federation2 federation.IFederation, asp istructs.IAppStructsProvider, timeFunc coreutils.TimeFunc, apppar appparts.IAppPartitions,
	builtinApps []appparts.BuiltInApp, itokens2 itokens.ITokens, storageProvider istorage.IAppStorageProvider, blobberAppStoragePtr iblobstoragestg.BlobAppStoragePtr,
	routerAppStoragePtr dbcertcache.RouterAppStoragePtr) (BootstrapOperator, error) {
	var clusterBuiltinApp btstrp.ClusterBuiltInApp
	otherApps := make([]appparts.BuiltInApp, 0, len(builtinApps)-1)
	for _, app := range builtinApps {
		if app.Name == istructs.AppQName_sys_cluster {
			clusterBuiltinApp = btstrp.ClusterBuiltInApp(app)
		} else {
			otherApps = append(otherApps, app)
		}
	}
	if clusterBuiltinApp.Name == istructs.NullAppQName {
		return nil, fmt.Errorf("%s app should be added to VVM builtin apps", istructs.AppQName_sys_cluster)
	}
	return pipeline.NewSyncOp(func(ctx context.Context, work interface{}) (err error) {
		return btstrp.Bootstrap(federation2, asp, timeFunc, apppar, clusterBuiltinApp, otherApps, itokens2, storageProvider, blobberAppStoragePtr, routerAppStoragePtr)
	}), nil
}

func provideExtensionPoints(appsArtefacts AppsArtefacts) map[istructs.AppQName]extensionpoints.IExtensionPoint {
	return appsArtefacts.appEPs
}

func provideBuiltInAppPackages(appsArtefacts AppsArtefacts) []BuiltInAppPackages {
	return appsArtefacts.builtInAppPackages
}

func provideAppConfigsTypeEmpty() AppConfigsTypeEmpty {
	return AppConfigsTypeEmpty(istructsmem.AppConfigsType{})
}

// AppConfigsTypeEmpty is provided here despite it looks senceless. But ok: it is a map that will be filled later, on BuildAppsArtefacts(), and used after filling only
// provide appsArtefacts.AppConfigsType here -> wire cycle: BuildappsArtefacts requires APIs requires IAppStructsProvider requires AppConfigsType obtained from BuildappsArtefacts
// The same approach does not work for IAppPartitions implementation, because the appparts.NewWithActualizerWithExtEnginesFactories() accepts
// iextengine.ExtensionEngineFactories that must be initialized with the already filled AppConfigsType
func provideIAppStructsProvider(cfgs AppConfigsTypeEmpty, bucketsFactory irates.BucketsFactoryType, appTokensFactory payloads.IAppTokensFactory,
	storageProvider istorage.IAppStorageProvider) istructs.IAppStructsProvider {
	return istructsmem.Provide(istructsmem.AppConfigsType(cfgs), bucketsFactory, appTokensFactory, storageProvider)
}

func provideAppPartitions(
	asp istructs.IAppStructsProvider,
	actualizer appparts.SyncActualizerFactory,
	appsArtefacts AppsArtefacts,
) (ap appparts.IAppPartitions, cleanup func(), err error) {

	eef := engines.ProvideExtEngineFactories(engines.ExtEngineFactoriesConfig{
		AppConfigs:  appsArtefacts.AppConfigsType,
		WASMCompile: false,
	})

	return appparts.NewWithActualizerWithExtEnginesFactories(asp, actualizer, eef)
}

func provideIsDeviceAllowedFunc(appsArtefacts AppsArtefacts) iauthnzimpl.IsDeviceAllowedFuncs {
	res := iauthnzimpl.IsDeviceAllowedFuncs{}
	for appQName, appEP := range appsArtefacts.appEPs {
		val, ok := appEP.Find(apps.EPIsDeviceAllowedFunc)
		if !ok {
			res[appQName] = func(as istructs.IAppStructs, requestWSID istructs.WSID, deviceProfileWSID istructs.WSID) (ok bool, err error) {
				return true, nil
			}
		} else {
			res[appQName] = val.(iauthnzimpl.IsDeviceAllowedFunc)
		}
	}
	return res
}

func provideBuiltInApps(appsArtefacts AppsArtefacts) []appparts.BuiltInApp {
	res := make([]appparts.BuiltInApp, len(appsArtefacts.builtInAppPackages))
	for i, pkg := range appsArtefacts.builtInAppPackages {
		res[i] = pkg.BuiltInApp
	}
	return res
}

func provideAppPartsCtlPipelineService(ctl apppartsctl.IAppPartitionsController) IAppPartsCtlPipelineService {
	return &AppPartsCtlPipelineService{IAppPartitionsController: ctl}
}

func provideIAppStorageUncachingProviderFactory(factory istorage.IAppStorageFactory, vvmCfg *VVMConfig) IAppStorageUncachingProviderFactory {
	return func() istorage.IAppStorageProvider {
		return provider.Provide(factory, vvmCfg.KeyspaceNameSuffix)
	}
}

func provideStorageFactory(vvmConfig *VVMConfig) (provider2 istorage.IAppStorageFactory, err error) {
	return vvmConfig.StorageFactory()
}

func provideSubjectGetterFunc() iauthnzimpl.SubjectGetterFunc {
	return func(requestContext context.Context, name string, as istructs.IAppStructs, wsid istructs.WSID) ([]appdef.QName, error) {
		kb := as.ViewRecords().KeyBuilder(invite.QNameViewSubjectsIdx)
		kb.PutInt64(invite.Field_LoginHash, coreutils.HashBytes([]byte(name)))
		kb.PutString(invite.Field_Login, name)
		subjectsIdx, err := as.ViewRecords().Get(wsid, kb)
		if err == istructsmem.ErrRecordNotFound {
			return nil, nil
		}
		if err != nil {

			return nil, err
		}
		res := []appdef.QName{}
		subjectID := subjectsIdx.AsRecordID(invite.Field_SubjectID)
		cdocSubject, err := as.Records().Get(wsid, true, istructs.RecordID(subjectID))
		if err != nil {

			return nil, err
		}
		roles := strings.Split(cdocSubject.AsString(invite.Field_Roles), ",")
		for _, role := range roles {
			roleQName, err := appdef.ParseQName(role)
			if err != nil {

				return nil, err
			}
			res = append(res, roleQName)
		}
		return res, nil
	}
}

func provideBucketsFactory(timeFunc coreutils.TimeFunc) irates.BucketsFactoryType {
	return func() irates.IBuckets {
		return iratesce.Provide(timeFunc)
	}
}

func provideSecretKeyJWT(sr isecrets.ISecretReader) (itokensjwt.SecretKeyType, error) {
	return sr.ReadSecret(itokensjwt.SecretKeyJWTName)
}

func provideNumsAppsWorkspaces(vvmApps VVMApps, asp istructs.IAppStructsProvider) (map[istructs.AppQName]istructs.NumAppWorkspaces, error) {
	res := map[istructs.AppQName]istructs.NumAppWorkspaces{}
	for _, appQName := range vvmApps {
		as, err := asp.AppStructs(appQName)
		if err != nil {

			return nil, err
		}
		res[appQName] = as.NumAppWorkspaces()
	}
	return res, nil
}

func provideMetricsServicePort(msp MetricsServicePortInitial, vvmIdx VVMIdxType) metrics.MetricsServicePort {
	if msp != 0 {
		return metrics.MetricsServicePort(msp) + metrics.MetricsServicePort(vvmIdx)
	}
	return metrics.MetricsServicePort(msp)
}

// VVMPort could be dynamic -> need a source to get the actual port later
// just calling RouterService.GetPort() causes wire cycle: RouterService requires IBus->VVMApps->FederationURL->VVMPort->RouterService
// so we need something in the middle of FederationURL and RouterService: FederationURL reads VVMPortSource, RouterService writes it.
func provideVVMPortSource() *VVMPortSource {
	return &VVMPortSource{}
}

func provideMetricsServiceOperator(ms metrics.MetricsService) MetricsServiceOperator {
	return pipeline.ServiceOperator(ms)
}

// TODO: consider vvmIdx
func provideIFederation(cfg *VVMConfig, vvmPortSource *VVMPortSource) (federation.IFederation, func()) {
	return federation.New(func() *url.URL {
		if cfg.FederationURL != nil {
			return cfg.FederationURL
		}
		resultFU, err := url.Parse(LocalHost + ":" + strconv.Itoa(int(vvmPortSource.getter())))
		if err != nil {

			panic(err)
		}
		return resultFU
	}, func() int { return vvmPortSource.adminGetter() })
}

// Metrics service port could be dynamic -> need a func that will return the actual port
func provideMetricsServicePortGetter(ms metrics.MetricsService) func() metrics.MetricsServicePort {
	return func() metrics.MetricsServicePort {
		return metrics.MetricsServicePort(ms.(interface{ GetPort() int }).GetPort())
	}
}

func provideRouterParams(cfg *VVMConfig, port VVMPortType, vvmIdx VVMIdxType) router.RouterParams {
	res := router.RouterParams{
		WriteTimeout:         cfg.RouterWriteTimeout,
		ReadTimeout:          cfg.RouterReadTimeout,
		ConnectionsLimit:     cfg.RouterConnectionsLimit,
		HTTP01ChallengeHosts: cfg.RouterHTTP01ChallengeHosts,
		RouteDefault:         cfg.RouteDefault,
		Routes:               cfg.Routes,
		RoutesRewrite:        cfg.RoutesRewrite,
		RouteDomains:         cfg.RouteDomains,
	}
	if port != 0 {
		res.Port = int(port) + int(vvmIdx)
	}
	return res
}

func provideVVMApps(builtInApps []appparts.BuiltInApp) (vvmApps VVMApps) {
	for _, builtInApp := range builtInApps {
		vvmApps = append(vvmApps, builtInApp.Name)
	}
	return vvmApps
}

func provideBuiltInAppsArtefacts(vvmConfig *VVMConfig, apis apps.APIs, cfgs AppConfigsTypeEmpty) (AppsArtefacts, error) {
	return vvmConfig.VVMAppsBuilder.BuildAppsArtefacts(apis, cfgs)
}

func provideServiceChannelFactory(vvmConfig *VVMConfig, procbus iprocbus.IProcBus) ServiceChannelFactory {
	return vvmConfig.ProvideServiceChannelFactory(procbus)
}

func provideProcessorChannelGroupIdxCommand(vvmCfg *VVMConfig) CommandProcessorsChannelGroupIdxType {
	return CommandProcessorsChannelGroupIdxType(getChannelGroupIdx(vvmCfg, ProcessorChannel_Command))
}

func provideProcessorChannelGroupIdxQuery(vvmCfg *VVMConfig) QueryProcessorsChannelGroupIdxType {
	return QueryProcessorsChannelGroupIdxType(getChannelGroupIdx(vvmCfg, ProcessorChannel_Query))
}

func getChannelGroupIdx(vvmCfg *VVMConfig, channelType ProcessorChannelType) int {
	for channelGroup, pc := range vvmCfg.processorsChannels {
		if pc.ChannelType == channelType {
			return channelGroup
		}
	}
	panic("wrong processor channel group config")
}

func provideChannelGroups(cfg *VVMConfig) (res []iprocbusmem.ChannelGroup) {
	for _, pc := range cfg.processorsChannels {
		res = append(res, pc.ChannelGroup)
	}
	return
}

func provideCachingAppStorageProvider(storageCacheSize StorageCacheSizeType, metrics2 imetrics.IMetrics,
	vvmName commandprocessor.VVMName, uncachingProvider IAppStorageUncachingProviderFactory) istorage.IAppStorageProvider {
	aspNonCaching := uncachingProvider()
	return istoragecache.Provide(int(storageCacheSize), aspNonCaching, metrics2, string(vvmName))
}

// синхронный актуализатор один на приложение из-за storages, которые у каждого приложения свои
// сделаем так, чтобы в командный процессор подавался свитч по appName, который выберет нужный актуализатор с нужным набором проекторов
type switchByAppName struct {
}

func (s *switchByAppName) Switch(work interface{}) (branchName string, err error) {
	return work.(interface{ AppQName() istructs.AppQName }).AppQName().String(), nil
}

func provideBlobAppStoragePtr(astp istorage.IAppStorageProvider) iblobstoragestg.BlobAppStoragePtr {
	return new(istorage.IAppStorage)
}

func provideBlobStorage(bas iblobstoragestg.BlobAppStoragePtr, nowFunc coreutils.TimeFunc) BlobStorage {
	return iblobstoragestg.Provide(bas, nowFunc)
}

func provideRouterAppStoragePtr(astp istorage.IAppStorageProvider) dbcertcache.RouterAppStoragePtr {
	return new(istorage.IAppStorage)
}

// port 80 -> [0] is http server, port 443 -> [0] is https server, [1] is acme server
func provideRouterServices(vvmCtx context.Context, rp router.RouterParams, busTimeout BusTimeout, broker in10n.IN10nBroker, quotas in10n.Quotas,
	nowFunc coreutils.TimeFunc, bsc router.BlobberServiceChannels, bms router.BLOBMaxSizeType, blobStorage BlobStorage,
	autocertCache autocert.Cache, bus ibus.IBus, vvmPortSource *VVMPortSource, numsAppsWorkspaces map[istructs.AppQName]istructs.NumAppWorkspaces) RouterServices {
	bp := &router.BlobberParams{
		ServiceChannels:        bsc,
		BLOBStorage:            blobStorage,
		BLOBWorkersNum:         DefaultBLOBWorkersNum,
		RetryAfterSecondsOn503: DefaultRetryAfterSecondsOn503,
		BLOBMaxSize:            bms,
	}
	httpSrv, acmeSrv, adminSrv := router.Provide(vvmCtx, rp, time.Duration(busTimeout), broker, bp, autocertCache, bus, numsAppsWorkspaces)
	vvmPortSource.getter = func() VVMPortType {
		return VVMPortType(httpSrv.GetPort())
	}
	vvmPortSource.adminGetter = func() int {
		return adminSrv.GetPort()
	}
	return RouterServices{
		httpSrv, acmeSrv, adminSrv,
	}
}

func provideAdminEndpointServiceOperator(rs RouterServices) AdminEndpointServiceOperator {
	return pipeline.ServiceOperator(rs.IAdminService)
}

func providePublicEndpointServiceOperator(rs RouterServices, metricsServiceOp MetricsServiceOperator) PublicEndpointServiceOperator {
	funcs := make([]pipeline.ForkOperatorOptionFunc, 2, 3)
	funcs[0] = pipeline.ForkBranch(pipeline.ServiceOperator(rs.IHTTPService))
	funcs[1] = pipeline.ForkBranch(metricsServiceOp)
	if rs.IACMEService != nil {
		funcs = append(funcs, pipeline.ForkBranch(pipeline.ServiceOperator(rs.IACMEService)))
	}
	return pipeline.ForkOperator(pipeline.ForkSame, funcs[0], funcs[1:]...)
}

func provideQueryChannel(sch ServiceChannelFactory) QueryChannel {
	return QueryChannel(sch(ProcessorChannel_Query, 0))
}

func provideCommandChannelFactory(sch ServiceChannelFactory) CommandChannelFactory {
	return func(channelIdx int) commandprocessor.CommandChannel {
		return commandprocessor.CommandChannel(sch(ProcessorChannel_Command, channelIdx))
	}
}

func provideQueryProcessors(qpCount istructs.NumQueryProcessors, qc QueryChannel, appParts appparts.IAppPartitions, qpFactory queryprocessor.ServiceFactory, imetrics2 imetrics.IMetrics,
	vvm commandprocessor.VVMName, mpq MaxPrepareQueriesType, authn iauthnz.IAuthenticator, authz iauthnz.IAuthorizer,
	tokens itokens.ITokens, federation2 federation.IFederation) OperatorQueryProcessors {
	forks := make([]pipeline.ForkOperatorOptionFunc, qpCount)
	resultSenderFactory := func(ctx context.Context, sender ibus.ISender) queryprocessor.IResultSenderClosable {
		return &resultSenderErrorFirst{
			ctx:    ctx,
			sender: sender,
		}
	}
	for i := 0; i < int(qpCount); i++ {
		forks[i] = pipeline.ForkBranch(pipeline.ServiceOperator(qpFactory(iprocbus.ServiceChannel(qc), resultSenderFactory, appParts, int(mpq), imetrics2, string(vvm), authn, authz, tokens, federation2)))
	}
	return pipeline.ForkOperator(pipeline.ForkSame, forks[0], forks[1:]...)
}

func provideCommandProcessors(cpCount istructs.NumCommandProcessors, ccf CommandChannelFactory, cpFactory commandprocessor.ServiceFactory) OperatorCommandProcessors {
	forks := make([]pipeline.ForkOperatorOptionFunc, cpCount)
	for i := 0; i < int(cpCount); i++ {
		forks[i] = pipeline.ForkBranch(pipeline.ServiceOperator(cpFactory(ccf(i), istructs.PartitionID(i))))
	}
	return pipeline.ForkOperator(pipeline.ForkSame, forks[0], forks[1:]...)
}

func provideAsyncActualizersFactory(appParts appparts.IAppPartitions, appStructsProvider istructs.IAppStructsProvider, n10nBroker in10n.IN10nBroker, asyncActualizerFactory projectors.AsyncActualizerFactory, secretReader isecrets.ISecretReader, metrics2 imetrics.IMetrics) AsyncActualizersFactory {
	return func(vvmCtx context.Context, appQName istructs.AppQName, asyncProjectors istructs.Projectors, partitionID istructs.PartitionID, opts []state.ActualizerStateOptFunc) pipeline.ISyncOperator {
		appStructs, err := appStructsProvider.AppStructs(appQName)
		if err != nil {
			panic(err)
		}

		conf := projectors.AsyncActualizerConf{
			Ctx:           vvmCtx,
			AppQName:      appQName,
			AppPartitions: appParts,
			AppStructs:    func() istructs.IAppStructs { return appStructs },
			SecretReader:  secretReader,
			Partition:     partitionID,
			Broker:        n10nBroker,
			Opts:          opts,
			IntentsLimit:  projectors.DefaultIntentsLimit,
			FlushInterval: actualizerFlushInterval,
			Metrics:       metrics2,
		}

		forkOps := make([]pipeline.ForkOperatorOptionFunc, 0, len(asyncProjectors))
		for _, prj := range asyncProjectors {
			asyncActualizer, err := asyncActualizerFactory(conf, prj)
			if err != nil {
				panic(err)
			}
			forkOps = append(forkOps, pipeline.ForkBranch(asyncActualizer))
		}

		return pipeline.ForkOperator(func(work interface{}, branchNumber int) (fork interface{}, err error) { return struct{}{}, nil }, forkOps[0], forkOps[1:]...)
	}
}

func provideAppPartitionFactory(aaf AsyncActualizersFactory, opts []state.ActualizerStateOptFunc) AppPartitionFactory {
	return func(vvmCtx context.Context, appQName istructs.AppQName, asyncProjectors istructs.Projectors, partitionID istructs.PartitionID) pipeline.ISyncOperator {
		return aaf(vvmCtx, appQName, asyncProjectors, partitionID, opts)
	}
}

// forks appPartition(just async actualizers for now) of one app by amount of partitions of the app
func provideAppServiceFactory(apf AppPartitionFactory) AppServiceFactory {
	return func(vvmCtx context.Context, appQName istructs.AppQName, asyncProjectors istructs.Projectors, appPartsCount istructs.NumAppPartitions) pipeline.ISyncOperator {
		forks := make([]pipeline.ForkOperatorOptionFunc, appPartsCount)
		for i := 0; i < int(appPartsCount); i++ {
			forks[i] = pipeline.ForkBranch(apf(vvmCtx, appQName, asyncProjectors, istructs.PartitionID(i)))
		}
		return pipeline.ForkOperator(pipeline.ForkSame, forks[0], forks[1:]...)
	}
}

// forks appServices per apps
// [appsAmount]appServices
func provideOperatorAppServices(apf AppServiceFactory, appsArtefacts AppsArtefacts, asp istructs.IAppStructsProvider) OperatorAppServicesFactory {
	return func(vvmCtx context.Context) pipeline.ISyncOperator {
		var branches []pipeline.ForkOperatorOptionFunc
		for _, builtInAppPackages := range appsArtefacts.builtInAppPackages {
			as, err := asp.AppStructs(builtInAppPackages.Name)
			if err != nil {
				panic(err)
			}
			if len(as.AsyncProjectors()) == 0 {
				continue
			}
			branch := pipeline.ForkBranch(apf(vvmCtx, builtInAppPackages.Name, as.AsyncProjectors(), builtInAppPackages.NumParts))
			branches = append(branches, branch)
		}
		if len(branches) == 0 {
			return &pipeline.NOOP{}
		}
		return pipeline.ForkOperator(pipeline.ForkSame, branches[0], branches[1:]...)
	}
}

func provideServicePipeline(vvmCtx context.Context, opCommandProcessors OperatorCommandProcessors, opQueryProcessors OperatorQueryProcessors,
	opAppServices OperatorAppServicesFactory, appPartsCtl IAppPartsCtlPipelineService, bootstrapSyncOp BootstrapOperator,
	adminEndpoint AdminEndpointServiceOperator, publicEndpoint PublicEndpointServiceOperator) ServicePipeline {
	return pipeline.NewSyncPipeline(vvmCtx, "ServicePipeline", pipeline.WireSyncOperator("internal services", pipeline.ForkOperator(pipeline.ForkSame, pipeline.ForkBranch(opQueryProcessors), pipeline.ForkBranch(opCommandProcessors), pipeline.ForkBranch(pipeline.ServiceOperator(appPartsCtl)))), pipeline.WireSyncOperator("admin endpoint", adminEndpoint), pipeline.WireSyncOperator("bootstrap", bootstrapSyncOp), pipeline.WireSyncOperator("public endpoint", publicEndpoint), pipeline.WireSyncOperator("async actualizers", opAppServices(vvmCtx)),
	)
}
