//go:generate go run github.com/google/wire/cmd/wire
//go:build wireinject
// +build wireinject

/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package vvm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"

	"github.com/google/wire"
	"golang.org/x/crypto/acme/autocert"

	"github.com/voedger/voedger/pkg/appdef/builder"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/apppartsctl"
	"github.com/voedger/voedger/pkg/btstrp"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/iblobstorage"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/itokens"
	"github.com/voedger/voedger/pkg/parser"
	"github.com/voedger/voedger/pkg/processors"
	"github.com/voedger/voedger/pkg/processors/actualizers"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
	"github.com/voedger/voedger/pkg/processors/schedulers"
	"github.com/voedger/voedger/pkg/router"
	builtinapps "github.com/voedger/voedger/pkg/vvm/builtin"
	"github.com/voedger/voedger/pkg/vvm/engines"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/federation"
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
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	"github.com/voedger/voedger/pkg/pipeline"
	commandprocessor "github.com/voedger/voedger/pkg/processors/command"
	queryprocessor "github.com/voedger/voedger/pkg/processors/query"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/sys/invite"
	"github.com/voedger/voedger/pkg/sys/sysprovide"
	dbcertcache "github.com/voedger/voedger/pkg/vvm/db_cert_cache"
	"github.com/voedger/voedger/pkg/vvm/metrics"
)

func ProvideVVM(vvmCfg *VVMConfig, vvmIdx VVMIdxType) (voedgerVM *VoedgerVM, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	voedgerVM = &VoedgerVM{vvmCtxCancel: cancel}
	vvmCfg.addProcessorChannel(
		// command processors
		// each restaurant must go to the same cmd proc -> one single cmd processor behind the each command service channel
		iprocbusmem.ChannelGroup{
			NumChannels:       uint(vvmCfg.NumCommandProcessors),
			ChannelBufferSize: uint(DefaultNumCommandProcessors), // to avoid bus timeout on big values of `vvmCfg.NumCommandProcessors``
		},
		ProcessorChannel_Command,
	)

	vvmCfg.addProcessorChannel(
		// query processors
		// all query processors sits on a single channel because any restaurant could be served by any query proc
		iprocbusmem.ChannelGroup{
			NumChannels:       1,
			ChannelBufferSize: 0,
		},
		ProcessorChannel_Query,
	)

	vvmCfg.addProcessorChannel(
		iprocbusmem.ChannelGroup{
			NumChannels:       1,
			ChannelBufferSize: 0,
		},
		ProcessorChannel_BLOB,
	)

	voedgerVM.VVM, voedgerVM.vvmCleanup, err = ProvideCluster(ctx, vvmCfg, vvmIdx)
	if err != nil {
		return nil, err
	}
	return voedgerVM, nil
}

func (vvm *VoedgerVM) Shutdown() {
	vvm.vvmCtxCancel() // VVM services are stopped here
	vvm.ServicePipeline.Close()
	vvm.vvmCleanup()
}

func (vvm *VoedgerVM) Launch() error {
	ign := ignition{}
	err := vvm.ServicePipeline.SendSync(ign)
	if err != nil {
		err = errors.Join(err, ErrVVMLaunchFailure)
		logger.Error(err)
	}
	return err
}

// vvmCtx must be cancelled by the caller right before vvm.ServicePipeline.Close()
func ProvideCluster(vvmCtx context.Context, vvmConfig *VVMConfig, vvmIdx VVMIdxType) (*VVM, func(), error) {
	panic(wire.Build(
		wire.Struct(new(VVM), "*"),
		wire.Struct(new(builtinapps.APIs), "*"),
		wire.Struct(new(schedulers.BasicSchedulerConfig), "VvmName", "SecretReader", "Tokens", "Metrics", "Broker", "Federation", "Time"),
		provideServicePipeline,
		provideCommandProcessors,
		provideQueryProcessors,
		provideOpBLOBProcessors,
		provideBlobAppStoragePtr,
		provideVVMApps,
		provideBuiltInAppsArtefacts,
		provideServiceChannelFactory,
		provideBlobStorage,
		provideChannelGroups,
		provideProcessorChannelGroupIdxCommand,
		provideProcessorChannelGroupIdxQuery,
		provideProcessorChannelGroupIdxBLOB,
		provideQueryChannel,
		provideCommandChannelFactory,
		provideRequestHandler,
		provideBLOBChannel,
		provideRouterParams,
		provideRouterAppStoragePtr,
		provideIFederation,
		provideCachingAppStorageProvider,  // IAppStorageProvider
		itokensjwt.ProvideITokens,         // ITokens
		provideIAppStructsProvider,        // IAppStructsProvider
		payloads.ProvideIAppTokensFactory, // IAppTokensFactory
		provideAppPartitions,
		in10nmem.ProvideEx2,
		queryprocessor.ProvideServiceFactory,
		commandprocessor.ProvideServiceFactory,
		metrics.ProvideMetricsService,
		dbcertcache.ProvideDbCache,
		imetrics.Provide,
		actualizers.ProvideSyncActualizerFactory,
		actualizers.NewSyncActualizerFactoryFactory,
		iprocbusmem.Provide,
		provideRouterServices,
		provideMetricsServiceOperator,
		provideMetricsServicePortGetter,
		provideMetricsServicePort,
		provideVVMPortSource,
		iauthnzimpl.NewDefaultAuthenticator,
		provideNumsAppsWorkspaces,
		provideSecretKeyJWT,
		provideBucketsFactory,
		provideSubjectGetterFunc,
		provideStorageFactory,
		provideIAppStorageUncachingProviderFactory,
		provideAppPartsCtlPipelineService,
		provideIsDeviceAllowedFunc,
		provideBuiltInApps,
		provideBasicAsyncActualizerConfig, // projectors.BasicAsyncActualizerConfig
		provideAsyncActualizersService,    // projectors.IActualizersService
		provideSchedulerRunner,            // appparts.IProcessorRunner
		apppartsctl.New,
		provideAppConfigsTypeEmpty,
		provideBuiltInAppPackages,
		provideBootstrapOperator,
		provideAdminEndpointServiceOperator,
		providePublicEndpointServiceOperator,
		provideBuildInfo,
		provideAppsExtensionPoints,
		provideStatelessResources,
		provideSidecarApps,
		provideN10NQuotas,
		provideWLimiterFactory,
		bus.NewIRequestSender,
		blobprocessor.NewIRequestHandler,
		// wire.Value(vvmConfig.NumCommandProcessors) -> (wire bug?) value github.com/untillpro/airs-bp3/vvm.CommandProcessorsCount can't be used: vvmConfig is not declared in package scope
		wire.FieldsOf(&vvmConfig,
			"NumCommandProcessors",
			"NumQueryProcessors",
			"NumBLOBProcessors",
			"Time",
			"BLOBMaxSize",
			"Name",
			"MaxPrepareQueries",
			"StorageCacheSize",
			"SendTimeout",
			"VVMPort",
			"MetricsServicePort",
			"ActualizerStateOpts",
			"SecretsReader",
		),
	))
}

func provideWLimiterFactory(maxSize iblobstorage.BLOBMaxSizeType) blobprocessor.WLimiterFactory {
	return func() iblobstorage.WLimiterType {
		return iblobstoragestg.NewWLimiter_Size(maxSize)
	}
}

func provideN10NQuotas(vvmCfg *VVMConfig) in10n.Quotas {
	return in10n.Quotas{
		Channels:                int(DefaultQuotasChannelsFactor * vvmCfg.NumCommandProcessors),
		ChannelsPerSubject:      DefaultQuotasChannelsPerSubject,
		Subscriptions:           int(DefaultQuotasSubscriptionsFactor * vvmCfg.NumCommandProcessors),
		SubscriptionsPerSubject: DefaultQuotasSubscriptionsPerSubject,
	}
}

func provideSchedulerRunner(cfg schedulers.BasicSchedulerConfig) appparts.ISchedulerRunner {
	return schedulers.ProvideSchedulers(cfg)
}

func provideBootstrapOperator(federation federation.IFederation, asp istructs.IAppStructsProvider, time coreutils.ITime, apppar appparts.IAppPartitions,
	builtinApps []appparts.BuiltInApp, sidecarApps []appparts.SidecarApp, itokens itokens.ITokens, storageProvider istorage.IAppStorageProvider, blobberAppStoragePtr iblobstoragestg.BlobAppStoragePtr,
	routerAppStoragePtr dbcertcache.RouterAppStoragePtr) (BootstrapOperator, error) {
	var clusterBuiltinApp btstrp.ClusterBuiltInApp
	otherApps := make([]appparts.BuiltInApp, 0, len(builtinApps)-1)
	for _, app := range builtinApps {
		if app.Name == istructs.AppQName_sys_cluster {
			clusterBuiltinApp = btstrp.ClusterBuiltInApp(app)
		} else {
			isSidecarApp := slices.ContainsFunc(sidecarApps, func(sa appparts.SidecarApp) bool {
				return sa.Name == app.Name
			})
			if !isSidecarApp {
				otherApps = append(otherApps, app)
			}
		}
	}
	if clusterBuiltinApp.Name == appdef.NullAppQName {
		return nil, fmt.Errorf("%s app should be added to VVM builtin apps", istructs.AppQName_sys_cluster)
	}
	return pipeline.NewSyncOp(func(ctx context.Context, work pipeline.IWorkpiece) (err error) {
		return btstrp.Bootstrap(federation, asp, time, apppar, clusterBuiltinApp, otherApps, sidecarApps, itokens, storageProvider, blobberAppStoragePtr, routerAppStoragePtr)
	}), nil
}

func provideBuiltInAppPackages(builtInAppsArtefacts BuiltInAppsArtefacts) []BuiltInAppPackages {
	return builtInAppsArtefacts.builtInAppPackages
}

func provideAppConfigsTypeEmpty() AppConfigsTypeEmpty {
	return AppConfigsTypeEmpty(istructsmem.AppConfigsType{})
}

// AppConfigsTypeEmpty is provided here despite it looks senceless. But ok: it is a map that will be filled later, on BuildAppsArtefacts(), and used after filling only
// provide builtInAppsArtefacts.AppConfigsType here -> wire cycle: BuildappsArtefacts requires APIs requires IAppStructsProvider requires AppConfigsType obtained from BuildappsArtefacts
// The same approach does not work for IAppPartitions implementation, because the appparts.NewWithActualizerWithExtEnginesFactories() accepts
// iextengine.ExtensionEngineFactories that must be initialized with the already filled AppConfigsType
func provideIAppStructsProvider(cfgs AppConfigsTypeEmpty, bucketsFactory irates.BucketsFactoryType, appTokensFactory payloads.IAppTokensFactory,
	storageProvider istorage.IAppStorageProvider) istructs.IAppStructsProvider {
	return istructsmem.Provide(istructsmem.AppConfigsType(cfgs), bucketsFactory, appTokensFactory, storageProvider)
}

func provideBasicAsyncActualizerConfig(
	vvm processors.VVMName,
	secretReader isecrets.ISecretReader,
	tokens itokens.ITokens,
	metrics imetrics.IMetrics,
	broker in10n.IN10nBroker,
	federation federation.IFederation,
	opts ...state.StateOptFunc,
) actualizers.BasicAsyncActualizerConfig {
	return actualizers.BasicAsyncActualizerConfig{
		VvmName:       string(vvm),
		SecretReader:  secretReader,
		Tokens:        tokens,
		Metrics:       metrics,
		Broker:        broker,
		Federation:    federation,
		Opts:          opts,
		IntentsLimit:  actualizers.DefaultIntentsLimit,
		FlushInterval: actualizerFlushInterval,
	}
}

func provideAsyncActualizersService(cfg actualizers.BasicAsyncActualizerConfig) actualizers.IActualizersService {
	return actualizers.ProvideActualizers(cfg)
}

func provideBuildInfo() (*debug.BuildInfo, error) {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, errors.New("no build info")
	}
	return buildInfo, nil
}

func provideAppsExtensionPoints(vvmConfig *VVMConfig) map[appdef.AppQName]extensionpoints.IExtensionPoint {
	res := map[appdef.AppQName]extensionpoints.IExtensionPoint{}
	for appQName := range vvmConfig.VVMAppsBuilder {
		res[appQName] = extensionpoints.NewRootExtensionPoint()
	}
	return res
}

func provideStatelessResources(cfgs AppConfigsTypeEmpty, vvmCfg *VVMConfig, appEPs map[appdef.AppQName]extensionpoints.IExtensionPoint,
	buildInfo *debug.BuildInfo, sp istorage.IAppStorageProvider, itokens itokens.ITokens, federation federation.IFederation,
	asp istructs.IAppStructsProvider, atf payloads.IAppTokensFactory) istructsmem.IStatelessResources {
	ssr := istructsmem.NewStatelessResources()
	sysprovide.ProvideStateless(ssr, vvmCfg.SmtpConfig, appEPs, buildInfo, sp, vvmCfg.WSPostInitFunc, vvmCfg.Time, itokens, federation,
		asp, atf)
	return ssr
}

func provideAppPartitions(
	vvmCtx context.Context,
	asp istructs.IAppStructsProvider,
	saf appparts.SyncActualizerFactory,
	act actualizers.IActualizersService,
	sch appparts.ISchedulerRunner,
	bf irates.BucketsFactoryType,
	sr istructsmem.IStatelessResources,
	builtinAppsArtefacts BuiltInAppsArtefacts,
	vvmName processors.VVMName,
	imetrics imetrics.IMetrics,
) (ap appparts.IAppPartitions, cleanup func(), err error) {

	eef := engines.ProvideExtEngineFactories(engines.ExtEngineFactoriesConfig{
		StatelessResources: sr,
		AppConfigs:         builtinAppsArtefacts.AppConfigsType,
		WASMConfig: iextengine.WASMFactoryConfig{
			Compile: false,
		},
	}, vvmName, imetrics)

	return appparts.New2(
		vvmCtx,
		asp,
		saf,
		act,
		sch,
		eef,
		bf,
	)
}

func provideIsDeviceAllowedFunc(appEPs map[appdef.AppQName]extensionpoints.IExtensionPoint) iauthnzimpl.IsDeviceAllowedFuncs {
	res := iauthnzimpl.IsDeviceAllowedFuncs{}
	for appQName, appEP := range appEPs {
		val, ok := appEP.Find(builtinapps.EPIsDeviceAllowedFunc)
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

func provideBuiltInApps(builtInAppsArtefacts BuiltInAppsArtefacts, sidecarApps []appparts.SidecarApp) []appparts.BuiltInApp {
	res := []appparts.BuiltInApp{}
	for _, pkg := range builtInAppsArtefacts.builtInAppPackages {
		res = append(res, pkg.BuiltInApp)
	}
	for _, sidecarApp := range sidecarApps {
		res = append(res, sidecarApp.BuiltInApp)
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

func provideStorageFactory(vvmConfig *VVMConfig) (provider istorage.IAppStorageFactory, err error) {
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
			// notest
			return nil, err
		}
		res := []appdef.QName{}
		subjectID := subjectsIdx.AsRecordID(invite.Field_SubjectID)
		cdocSubject, err := as.Records().Get(wsid, true, istructs.RecordID(subjectID))
		if err != nil {
			// notest
			return nil, err
		}
		if !cdocSubject.AsBool(appdef.SystemField_IsActive) {
			return nil, nil
		}
		roles := strings.Split(cdocSubject.AsString(invite.Field_Roles), ",")
		for _, role := range roles {
			roleQName, err := appdef.ParseQName(role)
			if err != nil {
				// notest
				// must be gauranted by the side that inserted this qname
				return nil, err
			}
			res = append(res, roleQName)
		}
		return res, nil
	}
}

func provideBucketsFactory(time coreutils.ITime) irates.BucketsFactoryType {
	return func() irates.IBuckets {
		return iratesce.Provide(time)
	}
}

func provideSecretKeyJWT(sr isecrets.ISecretReader) (itokensjwt.SecretKeyType, error) {
	return sr.ReadSecret(itokensjwt.SecretKeyJWTName)
}

func provideNumsAppsWorkspaces(vvmApps VVMApps, asp istructs.IAppStructsProvider, sidecarApps []appparts.SidecarApp) (map[appdef.AppQName]istructs.NumAppWorkspaces, error) {
	res := map[appdef.AppQName]istructs.NumAppWorkspaces{}
	for _, appQName := range vvmApps {
		sidecarNumAppWorkspaces := istructs.NumAppWorkspaces(0)
		for _, sa := range sidecarApps {
			if sa.Name == appQName {
				sidecarNumAppWorkspaces = sa.NumAppWorkspaces
				break
			}
		}
		if sidecarNumAppWorkspaces > 0 {
			// is sidecar app
			res[appQName] = sidecarNumAppWorkspaces
		} else {
			as, err := asp.BuiltIn(appQName)
			if err != nil {
				// notest
				return nil, err
			}
			res[appQName] = as.NumAppWorkspaces()
		}
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
			// notest
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

func provideBuiltInAppsArtefacts(vvmConfig *VVMConfig, apis builtinapps.APIs, cfgs AppConfigsTypeEmpty,
	appEPs map[appdef.AppQName]extensionpoints.IExtensionPoint) (BuiltInAppsArtefacts, error) {
	return vvmConfig.VVMAppsBuilder.BuildAppsArtefacts(apis, cfgs, appEPs)
}

// extModuleURLs is filled here
func parseSidecarAppSubDir(fullPath string, basePath string, out_extModuleURLs map[string]*url.URL) (asts []*parser.PackageSchemaAST, err error) {
	dirEntries, err := os.ReadDir(fullPath)
	if err != nil {
		// notest
		return nil, err
	}
	modulePath := strings.ReplaceAll(fullPath, basePath, "")
	modulePath = strings.TrimPrefix(modulePath, string(os.PathSeparator))
	modulePath = strings.ReplaceAll(modulePath, string(os.PathSeparator), "/")
	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			subASTs, err := parseSidecarAppSubDir(filepath.Join(fullPath, dirEntry.Name()), basePath, out_extModuleURLs)
			if err != nil {
				return nil, err
			}
			asts = append(asts, subASTs...)
			continue
		}
		if filepath.Ext(dirEntry.Name()) == ".wasm" {
			moduleURL, err := url.Parse("file:///" + filepath.Join(fullPath, dirEntry.Name()))
			if err != nil {
				// notest
				return nil, err
			}

			out_extModuleURLs[modulePath] = moduleURL
			continue
		}
	}

	dirAST, err := parser.ParsePackageDir(modulePath, os.DirFS(fullPath).(coreutils.IReadFS), ".")
	if err == nil {
		asts = append(asts, dirAST)
	} else if !errors.Is(err, parser.ErrDirContainsNoSchemaFiles) {
		return nil, err
	}
	return asts, nil
}

func provideSidecarApps(vvmConfig *VVMConfig) (res []appparts.SidecarApp, err error) {
	if len(vvmConfig.DataPath) == 0 {
		return nil, nil
	}
	appsPath := filepath.Join(vvmConfig.DataPath, "apps")
	appsEntries, err := os.ReadDir(appsPath)
	if err != nil {
		return nil, err
	}
	for _, appEntry := range appsEntries {
		if !appEntry.IsDir() {
			continue
		}
		appNameStr := filepath.Base(appEntry.Name())
		appNameParts := strings.Split(appNameStr, ".")
		appQName := appdef.NewAppQName(appNameParts[0], appNameParts[1])
		if _, ok := istructs.ClusterApps[appQName]; !ok {
			return nil, fmt.Errorf("ClusterAppID for sidecar app %s is unkknown", appQName)
		}
		appPath := filepath.Join(appsPath, appNameStr)
		appDirEntries, err := os.ReadDir(appPath)
		if err != nil {
			// notest
			return nil, err
		}
		var appDD *appparts.AppDeploymentDescriptor
		appASTs := []*parser.PackageSchemaAST{}
		extModuleURLs := map[string]*url.URL{}
		for _, appDirEntry := range appDirEntries {
			// descriptor.json file and image/pkg/ folder here
			if !appDirEntry.IsDir() && appDirEntry.Name() == "descriptor.json" {
				descriptorContent, err := os.ReadFile(filepath.Join(appPath, "descriptor.json"))
				if err != nil {
					// notest
					return nil, err
				}
				if err := json.Unmarshal(descriptorContent, &appDD); err != nil {
					return nil, fmt.Errorf("failed to unmarshal descriptor for sidecar app %s: %w", appEntry.Name(), err)
				}
			}
			if appDirEntry.IsDir() && appDirEntry.Name() == "image" {
				// how to consider that could be >1 ExtensionModules here?
				pkgPath := filepath.Join(appPath, "image", "pkg")
				appASTs, err = parseSidecarAppSubDir(pkgPath, pkgPath, extModuleURLs)
				if err != nil {
					return nil, err
				}
			}
		}
		if appDD == nil {
			return nil, fmt.Errorf("no descriptor for sidecar app %s", appQName)
		}

		appSchemaAST, err := parser.BuildAppSchema(appASTs)
		if err != nil {
			return nil, err
		}
		appDefBuilder := builder.New()
		if err := parser.BuildAppDefs(appSchemaAST, appDefBuilder); err != nil {
			return nil, err
		}

		appDef, err := appDefBuilder.Build()
		if err != nil {
			return nil, err
		}

		// TODO: implement sidecar apps schemas compatibility check (baseline_schemas)
		res = append(res, appparts.SidecarApp{
			BuiltInApp: appparts.BuiltInApp{
				AppDeploymentDescriptor: *appDD,
				Name:                    appQName,
				Def:                     appDef,
			},
			ExtModuleURLs: extModuleURLs,
		})
		logger.Info(fmt.Sprintf("sidecar app %s parsed", appQName))
	}
	return res, nil
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

func provideProcessorChannelGroupIdxBLOB(vvmCfg *VVMConfig) blobprocessor.BLOBServiceChannelGroupIdx {
	return blobprocessor.BLOBServiceChannelGroupIdx(getChannelGroupIdx(vvmCfg, ProcessorChannel_BLOB))
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

func provideCachingAppStorageProvider(storageCacheSize StorageCacheSizeType, metrics imetrics.IMetrics,
	vvmName processors.VVMName, uncachingProvider IAppStorageUncachingProviderFactory, iTime coreutils.ITime) istorage.IAppStorageProvider {
	aspNonCaching := uncachingProvider()
	return istoragecache.Provide(int(storageCacheSize), aspNonCaching, metrics, string(vvmName), iTime)
}

func provideBlobAppStoragePtr(astp istorage.IAppStorageProvider) iblobstoragestg.BlobAppStoragePtr {
	return new(istorage.IAppStorage)
}

func provideBlobStorage(bas iblobstoragestg.BlobAppStoragePtr, time coreutils.ITime) BlobStorage {
	return iblobstoragestg.Provide(bas, time)
}

func provideRouterAppStoragePtr(astp istorage.IAppStorageProvider) dbcertcache.RouterAppStoragePtr {
	return new(istorage.IAppStorage)
}

// port 80 -> [0] is http server, port 443 -> [0] is https server, [1] is acme server
func provideRouterServices(rp router.RouterParams, sendTimeout bus.SendTimeout, broker in10n.IN10nBroker, blobRequestHandler blobprocessor.IRequestHandler, quotas in10n.Quotas,
	wLimiterFactory blobprocessor.WLimiterFactory, blobStorage BlobStorage,
	autocertCache autocert.Cache, requestSender bus.IRequestSender, vvmPortSource *VVMPortSource) RouterServices {
	httpSrv, acmeSrv, adminSrv := router.Provide(rp, broker, blobRequestHandler, autocertCache, requestSender)
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

func provideBLOBChannel(sch ServiceChannelFactory) blobprocessor.BLOBServiceChannel {
	return blobprocessor.BLOBServiceChannel(sch(ProcessorChannel_BLOB, 0))
}

func provideCommandChannelFactory(sch ServiceChannelFactory) CommandChannelFactory {
	return func(channelIdx uint) commandprocessor.CommandChannel {
		return commandprocessor.CommandChannel(sch(ProcessorChannel_Command, channelIdx))
	}
}

func provideOpBLOBProcessors(numBLOBWorkers istructs.NumBLOBProcessors, blobServiceChannel blobprocessor.BLOBServiceChannel,
	blobStorage BlobStorage, wLimiterFactory blobprocessor.WLimiterFactory) OperatorBLOBProcessors {
	forks := make([]pipeline.ForkOperatorOptionFunc, numBLOBWorkers)
	for i := 0; i < int(numBLOBWorkers); i++ {
		forks[i] = pipeline.ForkBranch(pipeline.ServiceOperator(blobprocessor.ProvideService(blobServiceChannel, blobStorage,
			wLimiterFactory)))
	}
	return pipeline.ForkOperator(pipeline.ForkSame, forks[0], forks[1:]...)
}

func provideQueryProcessors(qpCount istructs.NumQueryProcessors, qc QueryChannel, appParts appparts.IAppPartitions, qpFactory queryprocessor.ServiceFactory,
	imetrics imetrics.IMetrics, vvm processors.VVMName, mpq MaxPrepareQueriesType, authn iauthnz.IAuthenticator,
	tokens itokens.ITokens, federation federation.IFederation, statelessResources istructsmem.IStatelessResources, secretReader isecrets.ISecretReader) OperatorQueryProcessors {
	forks := make([]pipeline.ForkOperatorOptionFunc, qpCount)
	for i := 0; i < int(qpCount); i++ {
		forks[i] = pipeline.ForkBranch(pipeline.ServiceOperator(qpFactory(iprocbus.ServiceChannel(qc), appParts, int(mpq), imetrics,
			string(vvm), authn, tokens, federation, statelessResources, secretReader)))
	}
	return pipeline.ForkOperator(pipeline.ForkSame, forks[0], forks[1:]...)
}

func provideCommandProcessors(cpCount istructs.NumCommandProcessors, ccf CommandChannelFactory, cpFactory commandprocessor.ServiceFactory) OperatorCommandProcessors {
	forks := make([]pipeline.ForkOperatorOptionFunc, cpCount)
	for i := uint(0); i < uint(cpCount); i++ {
		forks[i] = pipeline.ForkBranch(pipeline.ServiceOperator(cpFactory(ccf(i))))
	}
	return pipeline.ForkOperator(pipeline.ForkSame, forks[0], forks[1:]...)
}

func provideServicePipeline(
	vvmCtx context.Context,
	opCommandProcessors OperatorCommandProcessors,
	opQueryProcessors OperatorQueryProcessors,
	opBLOBProcessors OperatorBLOBProcessors,
	opAsyncActualizers actualizers.IActualizersService,
	appPartsCtl IAppPartsCtlPipelineService,
	bootstrapSyncOp BootstrapOperator,
	adminEndpoint AdminEndpointServiceOperator,
	publicEndpoint PublicEndpointServiceOperator,
	appStorageProvider istorage.IAppStorageProvider,
) ServicePipeline {
	return pipeline.NewSyncPipeline(vvmCtx, "ServicePipeline",
		pipeline.WireSyncOperator("internal services", pipeline.ForkOperator(pipeline.ForkSame,
			pipeline.ForkBranch(opQueryProcessors),
			pipeline.ForkBranch(opCommandProcessors),
			pipeline.ForkBranch(opBLOBProcessors),
			pipeline.ForkBranch(pipeline.ServiceOperator(opAsyncActualizers)),
			pipeline.ForkBranch(pipeline.ServiceOperator(appPartsCtl)),
			pipeline.ForkBranch(pipeline.ServiceOperator(appStorageProvider)),
		)),
		pipeline.WireSyncOperator("admin endpoint", adminEndpoint),
		pipeline.WireSyncOperator("bootstrap", bootstrapSyncOp),
		pipeline.WireSyncOperator("public endpoint", publicEndpoint),
	)
}
