/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package btstrp

import (
	"context"
	"fmt"
	"net/url"

	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/coreutils/federation"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/logger"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/itokens"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
	"github.com/voedger/voedger/pkg/sys"
	"github.com/voedger/voedger/pkg/vvm/builtin/clusterapp"
)

func Bootstrap(federation federation.IFederation, asp istructs.IAppStructsProvider, time timeu.ITime, appparts appparts.IAppPartitions,
	clusterApp ClusterBuiltInApp, otherApps []appparts.BuiltInApp, sidecarApps []appparts.SidecarApp, itokens itokens.ITokens, storageProvider istorage.IAppStorageProvider,
	postWiredInterfacePtrs PostWireInterfacePtrs, blobHandler blobprocessor.IRequestHandler,
	requestSender bus.IRequestSender) (err error) {

	logCtx := logger.WithContextAttrs(context.Background(), map[string]any{
		logger.LogAttr_VApp:      sys.VApp_SysVoedger,
		logger.LogAttr_Extension: "sys._Bootstrap",
	})
	logger.InfoCtx(logCtx, "bootstrap", "started")

	// initialize cluster app workspace, use app ws amount 0
	if err := initClusterAppWS(asp, time); err != nil {
		return err
	}
	logger.InfoCtx(logCtx, "bootstrap", "cluster app workspace initialized")

	// Initialize AppStorageBlobber (* IAppStorage), AppStorageRouter (* IAppStorage)
	if *postWiredInterfacePtrs.BlobberAppStorage, err = storageProvider.AppStorage(istructs.AppQName_sys_blobber); err != nil {
		// notest
		return err
	}
	if *postWiredInterfacePtrs.RouterAppStorage, err = storageProvider.AppStorage(istructs.AppQName_sys_router); err != nil {
		// notest
		return err
	}

	*postWiredInterfacePtrs.RequestSender = requestSender

	*postWiredInterfacePtrs.BlobHandler = blobHandler

	// appparts: deploy single clusterApp partition
	appparts.DeployApp(istructs.AppQName_sys_cluster, nil, clusterApp.Def, clusterapp.ClusterAppNumPartitions,
		clusterapp.ClusterAppNumEngines, clusterapp.ClusterAppNumAppWS)
	appparts.DeployAppPartitions(istructs.AppQName_sys_cluster, []istructs.PartitionID{clusterapp.ClusterAppWSIDPartitionID})

	sysToken, err := payloads.GetSystemPrincipalToken(itokens, istructs.AppQName_sys_cluster)
	if err != nil {
		return err
	}

	// For each app in otherApps: check apps compatibility by calling c.cluster.DeployApp
	for _, app := range otherApps {
		logger.InfoCtx(logCtx, "bootstrap.appdeploy.builtin", app.Name)
		callDeployApp(federation, sysToken, app)
	}
	for _, sidecarApp := range sidecarApps {
		logger.InfoCtx(logCtx, "bootstrap.appdeploy.sidecar", sidecarApp.BuiltInApp.Name)
		callDeployApp(federation, sysToken, sidecarApp.BuiltInApp)
	}

	// For each app builtInApps: deploy a builtin app
	for _, app := range otherApps {
		deployAppPartitions(logCtx, "bootstrap.apppartdeploy.builtin", appparts, app, nil)
	}
	for _, app := range sidecarApps {
		deployAppPartitions(logCtx, "bootstrap.apppartdeploy.sidecar", appparts, app.BuiltInApp, app.ExtModuleURLs)
	}

	logger.InfoCtx(logCtx, "bootstrap", "completed")
	return nil
}

func deployAppPartitions(ctx context.Context, stage string, appparts appparts.IAppPartitions, app appparts.BuiltInApp, extModuleURLs map[string]*url.URL) {
	appparts.DeployApp(app.Name, extModuleURLs, app.Def, app.NumParts, app.EnginePoolSize, app.NumAppWorkspaces)
	partitionIDs := make([]istructs.PartitionID, app.NumParts)
	for id := istructs.NumAppPartitions(0); id < app.NumParts; id++ {
		partitionIDs[id] = istructs.PartitionID(id)
		logger.InfoCtx(ctx, stage, fmt.Sprintf("%s/%d", app.Name, id))
	}
	appparts.DeployAppPartitions(app.Name, partitionIDs)
}

func callDeployApp(federation federation.IFederation, sysToken string, app appparts.BuiltInApp) {
	// Use Admin Endpoint to send requests
	body := fmt.Sprintf(`{"args":{"AppQName":"%s","NumPartitions":%d,"NumAppWorkspaces":%d}}`, app.Name, app.NumParts, app.NumAppWorkspaces)
	_, err := federation.AdminFunc(fmt.Sprintf("api/%s/%d/c.cluster.DeployApp", istructs.AppQName_sys_cluster, clusterapp.ClusterAppPseudoWSID), body,
		httpu.WithDiscardResponse(),
		httpu.WithAuthorizeBy(sysToken),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to deploy app %s: %s", app.Name, err.Error()))
	}
}

func initClusterAppWS(asp istructs.IAppStructsProvider, time timeu.ITime) error {
	as, err := asp.BuiltIn(istructs.AppQName_sys_cluster)
	if err == nil {
		_, err = cluster.InitAppWS(as, clusterapp.ClusterAppWSIDPartitionID, clusterapp.ClusterAppWSID, istructs.FirstOffset, istructs.FirstOffset,
			istructs.UnixMilli(time.Now().UnixMilli()))
	}
	return err
}
