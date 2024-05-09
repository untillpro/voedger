/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package vit

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/goutils/logger"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/irates"
	"github.com/voedger/voedger/pkg/istorage"
	"github.com/voedger/voedger/pkg/istorage/cas"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/istructsmem"
	payloads "github.com/voedger/voedger/pkg/itokens-payloads"
	"github.com/voedger/voedger/pkg/itokensjwt"
	"github.com/voedger/voedger/pkg/state"
	"github.com/voedger/voedger/pkg/state/smtptest"
	"github.com/voedger/voedger/pkg/sys/authnz"
	"github.com/voedger/voedger/pkg/sys/blobber"
	"github.com/voedger/voedger/pkg/sys/verifier"
	coreutils "github.com/voedger/voedger/pkg/utils"
	"github.com/voedger/voedger/pkg/vvm"
)

func NewVIT(t testing.TB, vitCfg *VITConfig, opts ...vitOptFunc) (vit *VIT) {
	useCas := coreutils.IsCassandraStorage()
	if !vitCfg.isShared {
		vit = newVit(t, vitCfg, useCas)
	} else {
		ok := false
		if vit, ok = vits[vitCfg]; ok {
			if !vit.isFinalized {
				panic("Teardown() was not called on a previous VIT which used the provided shared config")
			}
			vit.isFinalized = false
		} else {
			vit = newVit(t, vitCfg, useCas)
			vits[vitCfg] = vit
		}
	}

	for _, opt := range opts {
		opt(vit)
	}

	vit.emailCaptor.checkEmpty(t)

	vit.T = t

	// run each test in the next day to mostly prevent previous tests impact and\or workspace initialization
	vit.TimeAdd(day)

	vit.initialGoroutinesNum = runtime.NumGoroutine()

	return vit
}

func newVit(t testing.TB, vitCfg *VITConfig, useCas bool) *VIT {
	cfg := vvm.NewVVMDefaultConfig()

	// only dynamic ports are used in tests
	cfg.VVMPort = 0
	cfg.MetricsServicePort = 0

	cfg.TimeFunc = coreutils.TimeFunc(func() time.Time { return ts.now() })
	if !coreutils.IsTest() {
		cfg.SecretsReader = itokensjwt.ProvideTestSecretsReader(cfg.SecretsReader)
	}

	emailMessagesChan := make(chan smtptest.Message, 1) // must be buffered
	cfg.ActualizerStateOpts = append(cfg.ActualizerStateOpts, state.WithEmailMessagesChan(emailMessagesChan))

	vitPreConfig := &vitPreConfig{
		vvmCfg:  &cfg,
		vitApps: vitApps{},
	}
	for _, opt := range vitCfg.opts {
		opt(vitPreConfig)
	}

	for _, initFunc := range vitPreConfig.initFuncs {
		initFunc()
	}

	// eliminate timeouts impact for debugging
	cfg.RouterReadTimeout = int(debugTimeout)
	cfg.RouterWriteTimeout = int(debugTimeout)
	cfg.BusTimeout = vvm.BusTimeout(debugTimeout)

	if useCas {
		cfg.StorageFactory = func() (provider istorage.IAppStorageFactory, err error) {
			logger.Info("using istoragecas ", fmt.Sprint(vvm.DefaultCasParams))
			return cas.Provide(vvm.DefaultCasParams)
		}
	}

	vvm, err := vvm.ProvideVVM(&cfg, 0)
	require.NoError(t, err)

	// register workspace templates
	for _, app := range vitPreConfig.vitApps {
		ep := vvm.AppsExtensionPoints[app.name]
		for _, tf := range app.wsTemplateFuncs {
			tf(ep)
		}
	}

	vit := &VIT{
		VoedgerVM:            vvm,
		VVMConfig:            &cfg,
		T:                    t,
		appWorkspaces:        map[istructs.AppQName]map[string]*AppWorkspace{},
		principals:           map[istructs.AppQName]map[string]*Principal{},
		lock:                 sync.Mutex{},
		isOnSharedConfig:     vitCfg.isShared,
		configCleanupsAmount: len(vitPreConfig.cleanups),
		emailCaptor:          emailMessagesChan,
	}
	httpClient, httpClientCleanup := coreutils.NewIHTTPClient()
	vit.httpClient = httpClient

	vit.cleanups = append(vit.cleanups, vitPreConfig.cleanups...)
	vit.cleanups = append(vit.cleanups, func(vit *VIT) { httpClientCleanup() })

	// запустим сервер
	require.NoError(t, vit.Launch())

	for _, app := range vitPreConfig.vitApps {
		// generate verified value tokens if queried
		//                desiredValue token
		verifiedValues := map[string]string{}
		for desiredValue, vvi := range app.verifiedValuesIntents {
			appTokens := vvm.IAppTokensFactory.New(app.name)
			verifiedValuePayload := payloads.VerifiedValuePayload{
				VerificationKind: appdef.VerificationKind_EMail,
				Entity:           vvi.docQName,
				Field:            vvi.fieldName,
				Value:            vvi.desiredValue,
			}
			verifiedValueToken, err := appTokens.IssueToken(verifier.VerifiedValueTokenDuration, &verifiedValuePayload)
			require.NoError(vit.T, err)
			verifiedValues[desiredValue] = verifiedValueToken
		}

		// создадим логины и рабочие области
		for _, login := range app.logins {
			vit.SignUp(login.Name, login.Pwd, login.AppQName,
				WithReqOpt(coreutils.WithExpectedCode(http.StatusOK)),
				WithReqOpt(coreutils.WithExpectedCode(http.StatusConflict)),
			)
			prn := vit.SignIn(login)
			appPrincipals, ok := vit.principals[app.name]
			if !ok {
				appPrincipals = map[string]*Principal{}
				vit.principals[app.name] = appPrincipals
			}
			appPrincipals[login.Name] = prn

			for doc, dataFactory := range login.docs {
				if !vit.PostProfile(prn, "q.sys.Collection", fmt.Sprintf(`{"args":{"Schema":"%s"}}`, doc)).IsEmpty() {
					continue
				}
				data := dataFactory(verifiedValues)
				data[appdef.SystemField_ID] = 1
				data[appdef.SystemField_QName] = doc.String()

				bb, err := json.Marshal(data)
				require.NoError(t, err)

				vit.PostProfile(prn, "c.sys.CUD", fmt.Sprintf(`{"cuds":[{"fields":%s}]}`, bb))
			}
		}

		// time.Sleep(10 * time.Second)
		sysToken, err := payloads.GetSystemPrincipalToken(vit, app.name)
		require.NoError(vit.T, err)
		for _, wsd := range app.ws {
			owner := vit.principals[app.name][wsd.ownerLoginName]
			appWorkspaces, ok := vit.appWorkspaces[app.name]
			if !ok {
				appWorkspaces = map[string]*AppWorkspace{}
				vit.appWorkspaces[app.name] = appWorkspaces
			}
			newAppWS := vit.CreateWorkspace(wsd, owner, coreutils.WithExpectedCode(http.StatusOK), coreutils.WithExpectedCode(http.StatusConflict))
			newAppWS.childs = wsd.childs
			newAppWS.docs = wsd.docs
			newAppWS.subjects = wsd.subjects
			appWorkspaces[wsd.Name] = newAppWS

			handleWSParam(vit, appWorkspaces[wsd.Name], appWorkspaces, verifiedValues, sysToken)
		}
	}
	if vitPreConfig.postInitFunc != nil {
		vitPreConfig.postInitFunc(vit)
	}
	return vit
}

func handleWSParam(vit *VIT, appWS *AppWorkspace, appWorkspaces map[string]*AppWorkspace, verifiedValues map[string]string, token string) {
	for doc, dataFactory := range appWS.docs {
		if !vit.PostWS(appWS, "q.sys.Collection", fmt.Sprintf(`{"args":{"Schema":"%s"}}`, doc), coreutils.WithAuthorizeBy(token)).IsEmpty() {
			continue
		}
		data := dataFactory(verifiedValues)
		data[appdef.SystemField_ID] = 1
		data[appdef.SystemField_QName] = doc.String()

		bb, err := json.Marshal(data)
		require.NoError(vit.T, err)

		vit.PostWS(appWS, "c.sys.CUD", fmt.Sprintf(`{"cuds":[{"fields":%s}]}`, bb), coreutils.WithAuthorizeBy(token))
	}
	for _, subject := range appWS.subjects {
		roles := ""
		for i, role := range subject.roles {
			if i > 0 {
				roles += ","
			}
			roles += role.String()
		}
		body := fmt.Sprintf(`{"cuds":[{"fields":{"sys.ID":1,"sys.QName":"sys.Subject","Login":"%s","Roles":"%s","SubjectKind":%d,"ProfileWSID":%d}}]}`,
			subject.login, roles, subject.subjectKind, vit.principals[appWS.GetAppQName()][subject.login].ProfileWSID)
		vit.PostWS(appWS, "c.sys.CUD", body, coreutils.WithAuthorizeBy(token))
	}

	for _, childWSParams := range appWS.childs {
		vit.InitChildWorkspace(childWSParams, appWS)
		childAppWS := vit.WaitForChildWorkspace(appWS, childWSParams.Name)
		require.Empty(vit.T, childAppWS.WSError)
		childAppWS.childs = childWSParams.childs
		childAppWS.subjects = childWSParams.subjects
		childAppWS.docs = childWSParams.docs
		childAppWS.ownerLoginName = childWSParams.ownerLoginName
		childAppWS.Owner = vit.GetPrincipal(appWS.GetAppQName(), childWSParams.ownerLoginName)
		appWorkspaces[childWSParams.Name] = childAppWS
		handleWSParam(vit, childAppWS, appWorkspaces, verifiedValues, token)
	}
}

func NewVITLocalCassandra(tb testing.TB, vitCfg *VITConfig, opts ...vitOptFunc) (vit *VIT) {
	vit = newVit(tb, vitCfg, true)
	for _, opt := range opts {
		opt(vit)
	}

	return vit
}

func (vit *VIT) WS(appQName istructs.AppQName, wsName string) *AppWorkspace {
	appWorkspaces, ok := vit.appWorkspaces[appQName]
	if !ok {
		panic("unknown app " + appQName.String())
	}
	if ws, ok := appWorkspaces[wsName]; ok {
		return ws
	}
	panic("unknown workspace " + wsName)
}

func (vit *VIT) TearDown() {
	vit.T.Helper()
	vit.isFinalized = true
	for _, cleanup := range vit.cleanups {
		cleanup(vit)
	}
	vit.cleanups = vit.cleanups[:vit.configCleanupsAmount]
	grNum := runtime.NumGoroutine()
	if grNum-vit.initialGoroutinesNum > allowedGoroutinesNumDiff {
		vit.T.Logf("!!! goroutines leak: was %d on VIT setup, now %d after teardown", vit.initialGoroutinesNum, grNum)
	}
	vit.emailCaptor.checkEmpty(vit.T)
	if vit.isOnSharedConfig {
		return
	}
	vit.emailCaptor.shutDown()
	vit.Shutdown()
}

func (vit *VIT) MetricsServicePort() int {
	return int(vit.VoedgerVM.MetricsServicePort())
}

func (vit *VIT) GetSystemPrincipal(appQName istructs.AppQName) *Principal {
	vit.T.Helper()
	vit.lock.Lock()
	defer vit.lock.Unlock()
	appPrincipals, ok := vit.principals[appQName]
	if !ok {
		appPrincipals = map[string]*Principal{}
		vit.principals[appQName] = appPrincipals
	}
	prn, ok := appPrincipals["___sys"]
	if !ok {
		as, err := vit.IAppStructsProvider.AppStructs(appQName)
		require.NoError(vit.T, err)
		sysToken, err := payloads.GetSystemPrincipalTokenApp(as.AppTokens())
		require.NoError(vit.T, err)
		prn = &Principal{
			Token:       sysToken,
			ProfileWSID: istructs.NullWSID,
			Login: Login{
				Name:        "___sys",
				AppQName:    appQName,
				subjectKind: istructs.SubjectKind_User,
			},
		}
		appPrincipals["___sys"] = prn
	}
	return prn
}

func (vit *VIT) GetPrincipal(appQName istructs.AppQName, login string) *Principal {
	vit.T.Helper()
	appPrincipals, ok := vit.principals[appQName]
	if !ok {
		vit.T.Fatalf("unknown app %s", appQName)
	}
	prn, ok := appPrincipals[login]
	if !ok {
		vit.T.Fatalf("unknown login %s", login)
	}
	return prn
}

func (vit *VIT) PostProfile(prn *Principal, funcName string, body string, opts ...coreutils.ReqOptFunc) *coreutils.FuncResponse {
	vit.T.Helper()
	opts = append(opts, coreutils.WithAuthorizeByIfNot(prn.Token))
	return vit.PostApp(prn.AppQName, prn.ProfileWSID, funcName, body, opts...)
}

func (vit *VIT) PostWS(ws *AppWorkspace, funcName string, body string, opts ...coreutils.ReqOptFunc) *coreutils.FuncResponse {
	vit.T.Helper()
	opts = append(opts, coreutils.WithAuthorizeByIfNot(ws.Owner.Token))
	return vit.PostApp(ws.Owner.AppQName, ws.WSID, funcName, body, opts...)
}

// PostWSSys is PostWS authorized by the System Token
func (vit *VIT) PostWSSys(ws *AppWorkspace, funcName string, body string, opts ...coreutils.ReqOptFunc) *coreutils.FuncResponse {
	vit.T.Helper()
	sysPrn := vit.GetSystemPrincipal(ws.Owner.AppQName)
	opts = append(opts, coreutils.WithAuthorizeByIfNot(sysPrn.Token))
	return vit.PostApp(ws.Owner.AppQName, ws.WSID, funcName, body, opts...)
}

func (vit *VIT) UploadBLOBs(appQName istructs.AppQName, wsid istructs.WSID, blobs []blobber.BLOB, opts ...coreutils.ReqOptFunc) (blobIDs []istructs.RecordID) {
	vit.T.Helper()
	blobIDs, err := vit.IFederation.UploadBLOBs(appQName, wsid, blobs, opts...)
	require.NoError(vit.T, err)
	return blobIDs
}

func (vit *VIT) UploadBLOB(appQName istructs.AppQName, wsid istructs.WSID, blobName string, blobMimeType string, blobContent []byte,
	opts ...coreutils.ReqOptFunc) (blobID istructs.RecordID) {
	vit.T.Helper()
	blobID, err := vit.IFederation.UploadBLOB(appQName, wsid, blobName, blobMimeType, blobContent, opts...)
	require.NoError(vit.T, err)
	return blobID
}

func (vit *VIT) Func(url string, body string, opts ...coreutils.ReqOptFunc) *coreutils.FuncResponse {
	vit.T.Helper()
	res, err := vit.IFederation.Func(url, body, opts...)
	require.NoError(vit.T, err)
	return res
}

func (vit *VIT) ReadBLOB(appQName istructs.AppQName, wsid istructs.WSID, blobID istructs.RecordID, optFuncs ...coreutils.ReqOptFunc) *coreutils.HTTPResponse {
	vit.T.Helper()
	resp, err := vit.IFederation.ReadBLOB(appQName, wsid, blobID)
	require.NoError(vit.T, err)
	return resp
}

func (vit *VIT) Post(url string, body string, opts ...coreutils.ReqOptFunc) *coreutils.HTTPResponse {
	vit.T.Helper()
	res, err := vit.httpClient.Req(url, body, opts...)
	require.NoError(vit.T, err)
	return res
}

func (vit *VIT) PostApp(appQName istructs.AppQName, wsid istructs.WSID, funcName string, body string, opts ...coreutils.ReqOptFunc) *coreutils.FuncResponse {
	vit.T.Helper()
	url := fmt.Sprintf("api/%s/%d/%s", appQName, wsid, funcName)
	res, err := vit.IFederation.Func(url, body, opts...)
	require.NoError(vit.T, err)
	return res
}

func (vit *VIT) Get(url string, opts ...coreutils.ReqOptFunc) *coreutils.HTTPResponse {
	vit.T.Helper()
	res, err := vit.GET(url, "", opts...)
	require.NoError(vit.T, err)
	return res
}

func (vit *VIT) WaitFor(consumer func() *coreutils.FuncResponse) *coreutils.FuncResponse {
	vit.T.Helper()
	start := time.Now()
	for time.Since(start) < testTimeout {
		resp := consumer()
		if len(resp.Sections) > 0 {
			return resp
		}
		logger.Info("waiting for projection")
		time.Sleep(100 * time.Millisecond)
	}
	vit.T.Fail()
	return nil
}

func (vit *VIT) refreshTokens() {
	vit.T.Helper()
	for _, appPrns := range vit.principals {
		for _, prn := range appPrns {
			// issue principal token
			principalPayload := payloads.PrincipalPayload{
				Login:       prn.Login.Name,
				SubjectKind: istructs.SubjectKind_User,
				ProfileWSID: prn.ProfileWSID,
			}
			as, err := vit.IAppStructsProvider.AppStructs(prn.AppQName)
			require.NoError(vit.T, err) // notest
			newToken, err := as.AppTokens().IssueToken(authnz.DefaultPrincipalTokenExpiration, &principalPayload)
			require.NoError(vit.T, err)
			prn.Token = newToken
		}
	}
}

func (vit *VIT) NextNumber() int {
	vit.lock.Lock()
	vit.nextNumber++
	res := vit.nextNumber
	vit.lock.Unlock()
	return res
}

func (vit *VIT) Now() time.Time {
	return ts.now()
}

func (vit *VIT) SetNow(now time.Time) {
	ts.setCurrentInstant(now)
	vit.refreshTokens()
}

func (vit *VIT) TimeAdd(dur time.Duration) {
	ts.add(dur)
	vit.refreshTokens()
}

func (vit *VIT) NextName() string {
	return "name_" + strconv.Itoa(vit.NextNumber())
}

// sets `bs` as state of Buckets for `rateLimitName` in `appQName`
// will be automatically restored on vit.TearDown() to the state the Bucket was before MockBuckets() call
func (vit *VIT) MockBuckets(appQName istructs.AppQName, rateLimitName string, bs irates.BucketState) {
	vit.T.Helper()
	as, err := vit.IAppStructsProvider.AppStructs(appQName)
	require.NoError(vit.T, err)
	appBuckets := istructsmem.IBucketsFromIAppStructs(as)
	initialState, err := appBuckets.GetDefaultBucketsState(rateLimitName)
	require.NoError(vit.T, err)
	appBuckets.SetDefaultBucketState(rateLimitName, bs)
	appBuckets.ResetRateBuckets(rateLimitName, bs)
	vit.cleanups = append(vit.cleanups, func(vit *VIT) {
		appBuckets.SetDefaultBucketState(rateLimitName, initialState)
		appBuckets.ResetRateBuckets(rateLimitName, initialState)
	})
}

// CaptureEmail waits for and returns the next sent email
// no emails during testEmailsAwaitingTimeout -> test failed
// an email was sent but CaptureEmail is not called -> test will be failed on VIT.TearDown()
func (vit *VIT) CaptureEmail() (msg smtptest.Message) {
	vit.T.Helper()
	tmr := time.NewTimer(testEmailsAwaitingTimeout)
	select {
	case msg = <-vit.emailCaptor:
		return msg
	case <-tmr.C:
		vit.T.Fatal("no email messages")
	}
	return
}

func (vit *VIT) Restart() {
	vit.VoedgerVM.Shutdown()
}

// sets delay on IAppStorage.Get() in mem implementation
// will be automatically reset to 0 on TearDown
func (vit *VIT) SetMemStorageGetDelay(delay time.Duration) {
	vit.T.Helper()
	vit.iterateDelaySetters(func(delaySetter istorage.IStorageDelaySetter) {
		delaySetter.SetTestDelayGet(delay)
		vit.cleanups = append(vit.cleanups, func(vit *VIT) {
			delaySetter.SetTestDelayGet(0)
		})
	})
}

// sets delay on IAppStorage.Put() in mem implementation
// will be automatically reset to 0 on TearDown
func (vit *VIT) SetMemStoragePutDelay(delay time.Duration) {
	vit.T.Helper()
	vit.iterateDelaySetters(func(delaySetter istorage.IStorageDelaySetter) {
		delaySetter.SetTestDelayPut(delay)
		vit.cleanups = append(vit.cleanups, func(vit *VIT) {
			delaySetter.SetTestDelayPut(0)
		})
	})
}

func (vit *VIT) iterateDelaySetters(cb func(delaySetter istorage.IStorageDelaySetter)) {
	vit.T.Helper()
	for anyAppQName := range vit.VVMAppsBuilder {
		as, err := vit.IAppStorageProvider.AppStorage(anyAppQName)
		require.NoError(vit.T, err)
		delaySetter, ok := as.(istorage.IStorageDelaySetter)
		if !ok {
			vit.T.Fatal("IAppStorage implementation is not in-mem")
		}
		cb(delaySetter)
	}
}

func (ts *timeService) now() time.Time {
	ts.m.Lock()
	res := ts.currentInstant
	ts.m.Unlock()
	return res
}

func (ts *timeService) add(dur time.Duration) {
	ts.m.Lock()
	ts.currentInstant = ts.currentInstant.Add(dur)
	ts.m.Unlock()
}

func (ts *timeService) setCurrentInstant(now time.Time) {
	ts.m.Lock()
	ts.currentInstant = now
	ts.m.Unlock()
}

func ScanSSE(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2, data[0:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func (ec emailCaptor) checkEmpty(t testing.TB) {
	select {
	case _, ok := <-ec:
		if ok {
			t.Log("unexpected email message received")
			t.Fail()
		}
	default:
	}
}

func (ec emailCaptor) shutDown() {
	close(ec)
}
