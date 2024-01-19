/*
* Copyright (c) 2021-present unTill Pro, Ltd.
* @author Maxim Geraskin
 */

package queryprocessor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/iauthnzimpl"
	"github.com/voedger/voedger/pkg/iprocbus"
	"github.com/voedger/voedger/pkg/istructs"
	imetrics "github.com/voedger/voedger/pkg/metrics"
	ibus "github.com/voedger/voedger/staging/src/github.com/untillpro/airs-ibus"
)

func Benchmark_pipelineIService_Sequential(b *testing.B) {
	require := require.New(b)
	serviceChannel := make(iprocbus.ServiceChannel)
	res := make(chan interface{})
	body := []byte(`{
						"args":{
							"from":1257894000,
							"till":2257894000
						},
						"elements":[
							{
								"path":"",
								"fields":["sys.ID","name"],
								"refs":[["id_department","name"]]
							}
						],
						"filters":[{"expr":"eq","args":{"field":"id_department/name","value":"Alcohol drinks"}}],
						"orderBy":[{"field":"name"}],
						"count":1,
						"startFrom":1
					}`)
	rs := testResultSenderClosable{
		startArraySection: func(sectionType string, path []string) {},
		sendElement: func(name string, element interface{}) (err error) {
			values := element.([]interface{})
			res <- values
			return
		},
		close: func(err error) {
		},
	}
	authn := iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter)
	authz := iauthnzimpl.NewDefaultAuthorizer()
	cfgs, appStructsProvider, appTokens := getTestCfg(require, nil)
	queryProcessor := ProvideServiceFactory()(serviceChannel, func(ctx context.Context, sender ibus.ISender) IResultSenderClosable { return rs }, appStructsProvider, 3,
		imetrics.Provide(), "vvm", authn, authz, cfgs)
	go queryProcessor.Run(context.Background())
	as, err := appStructsProvider.AppStructs(istructs.AppQName_test1_app1)
	require.NoError(err)
	query := as.AppDef().Query(qNameFunction)
	start := time.Now()
	sysToken := getSystemToken(appTokens)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		serviceChannel <- NewQueryMessage(context.Background(), istructs.AppQName_test1_app1, 15, nil, body, query, "", sysToken)
		<-res
	}

	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "ops")
}

//Before   	    					6730	    180564 ns/op	      5538 ops
//With sync pipeline re-use   	   12390	     97833 ns/op	     10221 ops

func Benchmark_pipelineIService_Parallel(b *testing.B) {
	start := time.Now()

	b.SetParallelism(4)

	b.RunParallel(func(pb *testing.PB) {
		require := require.New(b)
		serviceChannel := make(iprocbus.ServiceChannel)
		body := []byte(`{
						"args":{
							"from":1257894000,
							"till":2257894000
						},
						"elements":[
							{
								"path":"",
								"fields":["sys.ID","name"],
								"refs":[["id_department","name"]]
							}
						],
						"filters":[{"expr":"eq","args":{"field":"id_department/name","value":"Alcohol drinks"}}],
						"orderBy":[{"field":"name"}],
						"count":1,
						"startFrom":1
					}`)
		res := make(chan interface{})
		rs := testResultSenderClosable{
			startArraySection: func(sectionType string, path []string) {},
			sendElement: func(name string, element interface{}) (err error) {
				values := element.([]interface{})
				res <- values
				return
			},
			close: func(err error) {
			},
		}
		authn := iauthnzimpl.NewDefaultAuthenticator(iauthnzimpl.TestSubjectRolesGetter)
		authz := iauthnzimpl.NewDefaultAuthorizer()
		cfgs, appStructsProvider, appTokens := getTestCfg(require, nil)
		queryProcessor := ProvideServiceFactory()(serviceChannel, func(ctx context.Context, sender ibus.ISender) IResultSenderClosable { return rs },
			appStructsProvider, 3, imetrics.Provide(), "vvm", authn, authz, cfgs)
		go queryProcessor.Run(context.Background())
		as, err := appStructsProvider.AppStructs(istructs.AppQName_test1_app1)
		require.NoError(err)
		query := as.AppDef().Query(qNameFunction)
		sysToken := getSystemToken(appTokens)

		b.ResetTimer()

		for pb.Next() {
			serviceChannel <- NewQueryMessage(context.Background(), istructs.AppQName_test1_app1, 15, nil, body, query, "", sysToken)
			<-res
		}
	})
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "ops")
}

//Before   	   						19144	     61210 ns/op	     16172 ops
//With sync pipeline re-use   	   	37027	     32734 ns/op	     30474 ops
