/*
* Copyright (c) 2021-present unTill Pro, Ltd.
* @author Maxim Geraskin
 */
package istructsmem

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iratesce"
	"github.com/voedger/voedger/pkg/istructs"
)

// Ref. bench.md for results

// Register a command "cmd" with ODoc "oDoc" as an argument
// oDoc has numOfIntFields int64 fields and same number of string fields
// Test BuildRawEvent performance
func Benchmark_BuildRawEvent(b *testing.B) {

	numOfIntFields := 2
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_BuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 4
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_BuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 8
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_BuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 16
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_BuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 32
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_BuildRawEvent(b, numOfIntFields)
	})
}

func bench_BuildRawEvent(b *testing.B, numOfIntFields int) {

	require := require.New(b)

	// Names

	appName := istructs.AppQName_test1_app1
	oDocQName := appdef.NewQName("test", "oDoc")
	cmdQName := appdef.NewQName("test", "cmd")

	// oDoc field names and values

	intFieldNames := make([]string, numOfIntFields)
	intFieldNamesFloat64Values := make(map[string]float64)
	stringFieldNames := make([]string, numOfIntFields)
	stringFieldValues := make(map[string]string)

	// application
	appDef := func() appdef.IAppDefBuilder {
		app := appdef.New()

		doc := app.AddODoc(oDocQName)
		for i := 0; i < numOfIntFields; i++ {

			intFieldName := fmt.Sprintf("i%v", i)
			doc.AddField(intFieldName, appdef.DataKind_int64, true)
			intFieldNames[i] = intFieldName
			intFieldNamesFloat64Values[intFieldName] = float64(i)

			stringFieldName := fmt.Sprintf("s%v", i)
			doc.AddField(stringFieldName, appdef.DataKind_string, true)
			stringFieldNames[i] = stringFieldName
			stringFieldValues[stringFieldName] = stringFieldName

		}
		app.AddCommand(cmdQName).SetParam(oDocQName)
		return app
	}

	// Con

	configs := make(AppConfigsType, 1)
	cfg := configs.AddConfig(appName, appDef())

	// Register command
	{
		cfg.Resources.Add(NewCommandFunction(cmdQName, NullCommandExec))
	}

	provider := Provide(configs, iratesce.TestBucketsFactory, testTokensFactory(), simpleStorageProvider())

	appStructs, err := provider.AppStructs(appName)
	require.NoError(err)

	start := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		bld := appStructs.Events().GetSyncRawEventBuilder(
			istructs.SyncRawEventBuilderParams{
				GenericRawEventBuilderParams: istructs.GenericRawEventBuilderParams{
					HandlingPartition: 55,
					PLogOffset:        10000,
					Workspace:         1234,
					WLogOffset:        1000,
					QName:             cmdQName,
					RegisteredAt:      100500,
				},
				Device:   762,
				SyncedAt: 1005001,
			})

		cmd := bld.ArgumentObjectBuilder()
		cmd.PutRecordID(appdef.SystemField_ID, 1)
		for i := 0; i < numOfIntFields; i++ {
			cmd.PutNumber(intFieldNames[i], intFieldNamesFloat64Values[intFieldNames[i]])
			cmd.PutString(stringFieldNames[i], stringFieldValues[stringFieldNames[i]])
		}

		_, buildErr := bld.BuildRawEvent()
		if buildErr != nil {
			panic(buildErr)
		}

	}
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "op/s")
	require.NoError(err)

}

func Benchmark_UnmarshalJSONForBuildRawEvent(b *testing.B) {
	numOfIntFields := 2
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_UnmarshalJSONForBuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 4
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_UnmarshalJSONForBuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 8
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_UnmarshalJSONForBuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 16
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_UnmarshalJSONForBuildRawEvent(b, numOfIntFields)
	})

	numOfIntFields = 32
	b.Run(fmt.Sprint("numOfFields=", numOfIntFields*2), func(b *testing.B) {
		bench_UnmarshalJSONForBuildRawEvent(b, numOfIntFields)
	})
}

func bench_UnmarshalJSONForBuildRawEvent(b *testing.B, numOfIntFields int) {

	require := require.New(b)

	srcMap := make(map[string]interface{})

	// Prepare source map
	{
		for i := 0; i < numOfIntFields; i++ {

			intFieldName := fmt.Sprintf("i%v", i)
			srcMap[intFieldName] = float64(i)

			stringFieldName := fmt.Sprintf("s%v", i)
			srcMap[stringFieldName] = stringFieldName

		}
	}
	bytes, err := json.Marshal(srcMap)
	require.NoError(err)

	start := time.Now()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{})
		err = json.Unmarshal(bytes, &m)
		if err != nil {
			panic("err != nil")
		}
		if len(m) < 1 {
			panic("len(m) < 1")
		}
	}
	b.ReportMetric(float64(b.N)/time.Since(start).Seconds(), "op/s")
}
