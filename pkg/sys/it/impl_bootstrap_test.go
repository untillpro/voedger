/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package sys_it

import (
	"fmt"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appparts"
	"github.com/voedger/voedger/pkg/btstrp"
	"github.com/voedger/voedger/pkg/bus"
	"github.com/voedger/voedger/pkg/cluster"
	"github.com/voedger/voedger/pkg/extensionpoints"
	"github.com/voedger/voedger/pkg/goutils/httpu"
	"github.com/voedger/voedger/pkg/goutils/testingu"
	"github.com/voedger/voedger/pkg/goutils/timeu"
	"github.com/voedger/voedger/pkg/iblobstoragestg"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/mem"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/parser"
	blobprocessor "github.com/voedger/voedger/pkg/processors/blobber"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/sysprovide"
	it "github.com/voedger/voedger/pkg/vit"
	"github.com/voedger/voedger/pkg/vvm"
	builtinapps "github.com/voedger/voedger/pkg/vvm/builtin"
	"github.com/voedger/voedger/pkg/vvm/builtin/clusterapp"
	dbcertcache "github.com/voedger/voedger/pkg/vvm/db_cert_cache"
)

func TestBoostrap_BasicUsage(t *testing.T) {
	require := require.New(t)
	memStorage := mem.Provide(testingu.MockTime)

	// launch the VVM with an app with a certain NumParts and NumAppWorkspaces
	numParts := istructs.NumAppPartitions(42)
	numAppWS := istructs.NumAppWorkspaces(43)
	cfg := getTestCfg(numParts, numAppWS, memStorage)
	vit := it.NewVIT(t, &cfg)
	defer vit.TearDown()

	var clusterApp btstrp.ClusterBuiltInApp
	otherApps := []appparts.BuiltInApp{}
	for _, app := range vit.BuiltInAppsPackages {
		if app.Name == istructs.AppQName_sys_cluster {
			clusterApp = btstrp.ClusterBuiltInApp(app.BuiltInApp)
		} else {
			otherApps = append(otherApps, app.BuiltInApp)
		}
	}

	t.Run("basic usage", func(t *testing.T) {
		appParts, cleanup := appparts.NewTestAppParts(vit.IAppStructsProvider)
		defer cleanup()
		postWiredInterfacePtrs := newPostWiredInterfacePtrs()
		testBlobRequestHandler := blobprocessor.NewIRequestHandler(nil, 0, nil)
		testRequestSender := bus.NewIRequestSender(testingu.MockTime, nil)
		err := btstrp.Bootstrap(vit.IFederation, vit.IAppStructsProvider, vit.Time, appParts, clusterApp, otherApps,
			nil, vit.ITokens, vit.IAppStorageProvider, postWiredInterfacePtrs, testBlobRequestHandler, testRequestSender)
		require.NoError(err)
		require.NotNil(*postWiredInterfacePtrs.BlobberAppStorage)
		require.NotNil(*postWiredInterfacePtrs.RouterAppStorage)
		require.NotNil(*postWiredInterfacePtrs.RequestSender)
		require.NotNil(*postWiredInterfacePtrs.BlobHandler)

	})

	t.Run("panic on NumPartitions change", func(t *testing.T) {
		appParts, cleanup := appparts.NewTestAppParts(vit.IAppStructsProvider)
		defer cleanup()
		otherApps[0].AppDeploymentDescriptor.NumParts++
		defer func() {
			otherApps[0].AppDeploymentDescriptor.NumParts--
		}()
		postWiredInterfacePtrs := newPostWiredInterfacePtrs()
		testBlobRequestHandler := blobprocessor.NewIRequestHandler(nil, 0, nil)
		testRequestSender := bus.NewIRequestSender(testingu.MockTime, nil)
		//nolint errcheck
		require.PanicsWithValue(fmt.Sprintf("failed to deploy app %[1]s: status 409, expected [200 201]: num partitions changed: app %[1]s declaring NumPartitions=%d but was previously deployed with NumPartitions=%d",
			otherApps[0].Name, otherApps[0].AppDeploymentDescriptor.NumParts, otherApps[0].AppDeploymentDescriptor.NumParts-1), func() {
			btstrp.Bootstrap(vit.IFederation, vit.IAppStructsProvider, vit.Time, appParts, clusterApp, otherApps,
				nil, vit.ITokens, vit.IAppStorageProvider, postWiredInterfacePtrs, testBlobRequestHandler, testRequestSender)
		})
	})

	t.Run("panic on NumAppPartitions change", func(t *testing.T) {
		appParts, cleanup := appparts.NewTestAppParts(vit.IAppStructsProvider)
		defer cleanup()
		otherApps[0].AppDeploymentDescriptor.NumAppWorkspaces++
		defer func() {
			otherApps[0].AppDeploymentDescriptor.NumAppWorkspaces--
		}()

		//nolint errcheck
		require.PanicsWithValue(fmt.Sprintf("failed to deploy app %[1]s: status 409, expected [200 201]: num application workspaces changed: app %[1]s declaring NumAppWorkspaces=%d but was previously deployed with NumAppWorkspaces=%d",
			otherApps[0].Name, otherApps[0].AppDeploymentDescriptor.NumAppWorkspaces, otherApps[0].AppDeploymentDescriptor.NumAppWorkspaces-1), func() {
			postWiredInterfacePtrs := newPostWiredInterfacePtrs()
			testBlobRequestHandler := blobprocessor.NewIRequestHandler(nil, 0, nil)
			testRequestSender := bus.NewIRequestSender(testingu.MockTime, nil)
			btstrp.Bootstrap(vit.IFederation, vit.IAppStructsProvider, vit.Time, appParts, clusterApp, otherApps,
				nil, vit.ITokens, vit.IAppStorageProvider, postWiredInterfacePtrs, testBlobRequestHandler, testRequestSender)
		})
	})
}

func newPostWiredInterfacePtrs() btstrp.PostWireInterfacePtrs {
	return btstrp.PostWireInterfacePtrs{
		BlobberAppStorage: iblobstoragestg.BlobAppStoragePtr(new(istorage.IAppStorage)),
		RouterAppStorage:  dbcertcache.RouterAppStoragePtr(new(istorage.IAppStorage)),
		BlobHandler:       blobprocessor.IRequestHandlerPtr(new(blobprocessor.IRequestHandler)),
		RequestSender:     bus.IRequestSenderPtr(new(bus.IRequestSender)),
	}
}

func getTestCfg(numParts istructs.NumAppPartitions, numAppWS istructs.NumAppWorkspaces, storage istorage.IAppStorageFactory) it.VITConfig {
	fs := fstest.MapFS{
		"app.vsql": &fstest.MapFile{
			Data: []byte(`APPLICATION app1();`),
		},
	}
	app1PackageFS := parser.PackageFS{
		Path: it.App1PkgPath,
		FS:   fs,
	}
	return it.NewOwnVITConfig(
		it.WithApp(istructs.AppQName_test1_app1, func(apis builtinapps.APIs, cfg *istructsmem.AppConfigType, ep extensionpoints.IExtensionPoint) builtinapps.Def {
			sysPkg := sysprovide.Provide(cfg)
			return builtinapps.Def{
				AppDeploymentDescriptor: appparts.AppDeploymentDescriptor{
					NumParts:         numParts,
					EnginePoolSize:   it.DefaultTestAppEnginesPool,
					NumAppWorkspaces: numAppWS,
				},
				AppQName: istructs.AppQName_test1_app1,
				Packages: []parser.PackageFS{sysPkg, app1PackageFS},
			}
		}),
		it.WithVVMConfig(func(cfg *vvm.VVMConfig) {
			// use predefined storage
			cfg.StorageFactory = func(timeu.ITime) (provider istorage.IAppStorageFactory, err error) {
				return storage, nil
			}
		}),
	)
}

func TestDeployAppErrors(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	sysToken, err := payloads.GetSystemPrincipalToken(vit.ITokens, istructs.AppQName_sys_cluster)
	require.NoError(err)

	t.Run("sys/cluster can not be deployed by c.cluster.DeployApp", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"AppQName":"%s","NumPartitions":1,"NumAppWorkspaces":1}}`, istructs.AppQName_sys_cluster)
		vit.PostApp(istructs.AppQName_sys_cluster, clusterapp.ClusterAppPseudoWSID, "c.cluster.DeployApp", body,
			httpu.WithAuthorizeBy(sysToken), httpu.Expect400()).Println()
	})

	var test1App1DeploymentDescriptor appparts.AppDeploymentDescriptor
	for _, app := range vit.BuiltInAppsPackages {
		if app.Name == istructs.AppQName_test1_app1 {
			test1App1DeploymentDescriptor = app.AppDeploymentDescriptor
			break
		}
	}

	t.Run("409 conflict on try to deploy with different NumPartitions", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"AppQName":"%s","NumPartitions":%d,"NumAppWorkspaces":%d}}`,
			istructs.AppQName_test1_app1,
			test1App1DeploymentDescriptor.NumParts+1, test1App1DeploymentDescriptor.NumAppWorkspaces)
		resp := vit.PostApp(istructs.AppQName_sys_cluster, clusterapp.ClusterAppPseudoWSID, "c.cluster.DeployApp", body,
			httpu.WithAuthorizeBy(sysToken),
			httpu.Expect409(),
		)
		resp.Println()
		require.Empty(resp.NewIDs)
	})

	t.Run("409 conflict on try to deploy with different NumAppPartitions", func(t *testing.T) {
		body := fmt.Sprintf(`{"args":{"AppQName":"%s","NumPartitions":%d,"NumAppWorkspaces":%d}}`,
			istructs.AppQName_test1_app1,
			test1App1DeploymentDescriptor.NumParts, test1App1DeploymentDescriptor.NumAppWorkspaces+1)
		resp := vit.PostApp(istructs.AppQName_sys_cluster, clusterapp.ClusterAppPseudoWSID, "c.cluster.DeployApp", body,
			httpu.WithAuthorizeBy(sysToken),
			httpu.Expect409(),
		)
		resp.Println()
		require.Empty(resp.NewIDs)
	})

	t.Run("400 bad request on wrong appQName", func(t *testing.T) {
		body := `{"args":{"AppQName":"wrong","NumPartitions":1,"NumAppWorkspaces":1}}`
		vit.PostApp(istructs.AppQName_sys_cluster, clusterapp.ClusterAppPseudoWSID, "c.cluster.DeployApp", body,
			httpu.WithAuthorizeBy(sysToken),
			httpu.Expect400(),
		).Println()
	})
}

func TestAppWSInitIndempotency(t *testing.T) {
	require := require.New(t)
	vit := it.NewVIT(t, &it.SharedConfig_App1)
	defer vit.TearDown()

	checkCDocsWSDesc(vit.VVMConfig, vit.VVM, require)

	// init app ws again (first is done on NewVIT()) -> expect no errors + assume next tests will work as well
	for _, app := range vit.BuiltInAppsPackages {
		as, err := vit.BuiltIn(app.Name)
		require.NoError(err)
		initedWSIDs, err := cluster.InitAppWSes(as, as.NumAppWorkspaces(), app.NumParts, istructs.UnixMilli(vit.Time.Now().UnixMilli()))
		require.NoError(err)
		require.Empty(initedWSIDs)
	}
}

func checkCDocsWSDesc(vvmCfg *vvm.VVMConfig, vvm *vvm.VVM, require *require.Assertions) {
	for appQName := range vvmCfg.VVMAppsBuilder {
		as, err := vvm.BuiltIn(appQName)
		require.NoError(err)
		for wsNum := 0; istructs.NumAppWorkspaces(wsNum) < as.NumAppWorkspaces(); wsNum++ {
			appWSID := istructs.NewWSID(istructs.CurrentClusterID(), istructs.WSID(wsNum+int(istructs.FirstBaseAppWSID)))
			existingCDocWSDesc, err := as.Records().GetSingleton(appWSID, authnz.QNameCDocWorkspaceDescriptor)
			require.NoError(err)
			require.Equal(authnz.QNameCDocWorkspaceDescriptor, existingCDocWSDesc.QName())
		}
	}
}
