/*
 * Copyright (c) 2021-present unTill Pro, Ltd.
 */

package queryprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/pipeline"
	"github.com/voedger/voedger/pkg/state"
	coreutils "github.com/voedger/voedger/pkg/utils"
)

func TestEnrichmentOperator_DoSync(t *testing.T) {
	t.Run("Should set reference fields", func(t *testing.T) {
		require := require.New(t)
		commonSchema := coreutils.TestSchema{
			Fields_: map[string]istructs.DataKindType{
				"id_lower_case_name": istructs.DataKind_RecordID,
			},
			QName_: istructs.NullQName,
		}
		commonFields := []IRefField{refField{field: "id_lower_case_name", ref: "name", key: "id_lower_case_name/name"}}
		schemas := coreutils.TestSchemas{Schemas_: map[istructs.QName]istructs.ISchema{
			istructs.NewQName("", "root"):                  commonSchema,
			istructs.NewQName("f", "first-children-1"):     commonSchema,
			istructs.NewQName("f", "deep-children-1"):      commonSchema,
			istructs.NewQName("f", "very-deep-children-1"): commonSchema,
			istructs.NewQName("s", "first-children-2"):     commonSchema,
			istructs.NewQName("s", "deep-children-1"):      commonSchema,
			istructs.NewQName("s", "very-deep-children-1"): commonSchema,
			qNameXLowerCase: coreutils.TestSchema{
				Fields_: map[string]istructs.DataKindType{
					"name": istructs.DataKind_string,
				},
				QName_: qNameXLowerCase,
			},
		}}
		elements := []IElement{
			element{
				path: path{rootDocument},
				refs: commonFields,
			},
			element{
				path: path{"first-children-1"},
				refs: commonFields,
			},
			element{
				path: path{"first-children-1", "deep-children-1"},
				refs: commonFields,
			},
			element{
				path: path{"first-children-1", "deep-children-1", "very-deep-children-1"},
				refs: commonFields,
			},
			element{
				path: path{"first-children-2"},
				refs: commonFields,
			},
			element{
				path: path{"first-children-2", "deep-children-1"},
				refs: commonFields,
			},
			element{
				path: path{"first-children-2", "deep-children-1", "very-deep-children-1"},
				refs: commonFields,
			},
		}
		row := func(idLowerCaseName int) IOutputRow {
			return &outputRow{
				keyToIdx: map[string]int{
					"id_lower_case_name/name": 0,
				},
				values: []interface{}{
					istructs.RecordID(idLowerCaseName),
				},
			}
		}
		record := func(name string) istructs.IStateValue {
			r := &mockRecord{}
			r.
				On("AsString", "name").Return(name).
				On("QName").Return(qNameXLowerCase)
			sv := &mockStateValue{}
			sv.On("AsRecord", "").Return(r)
			return sv
		}
		work := func() pipeline.IWorkpiece {
			o := &coreutils.TestObject{
				Name:    istructs.NewQName("", "root"),
				Id:      istructs.RecordID(1),
				Parent_: istructs.NullRecordID,
				Data: map[string]interface{}{
					"id_lower_case_name": istructs.RecordID(2001),
					"name":               "ROOT",
				},
				Containers_: map[string][]*coreutils.TestObject{
					"first-children-1": {
						{
							Name:    istructs.NewQName("f", "first-children-1"),
							Id:      istructs.RecordID(101),
							Parent_: istructs.RecordID(1),
							Data: map[string]interface{}{
								"id_lower_case_name": istructs.RecordID(200101),
								"name":               "FIRST-CHILDREN-1-101",
							},
							Containers_: map[string][]*coreutils.TestObject{
								"deep-children-1": {
									{
										Name:    istructs.NewQName("f", "deep-children-1"),
										Id:      istructs.RecordID(201),
										Parent_: istructs.RecordID(101),
										Data: map[string]interface{}{
											"id_lower_case_name": istructs.RecordID(200201),
											"name":               "DEEP-CHILDREN-1-201",
										},
										Containers_: map[string][]*coreutils.TestObject{
											"very-deep-children-1": {
												{
													Name:    istructs.NewQName("f", "very-deep-children-1"),
													Id:      istructs.RecordID(301),
													Parent_: istructs.RecordID(201),
													Data: map[string]interface{}{
														"id_lower_case_name": istructs.RecordID(200301),
														"name":               "VERY-DEEP-CHILDREN-1-301",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name:    istructs.NewQName("f", "first-children-1"),
							Id:      istructs.RecordID(102),
							Parent_: istructs.RecordID(1),
							Data: map[string]interface{}{
								"id_lower_case_name": istructs.RecordID(200102),
								"name":               "FIRST-CHILDREN-1-102",
							},
						},
					},
					"first-children-2": {
						{
							Name:    istructs.NewQName("s", "first-children-2"),
							Id:      istructs.RecordID(401),
							Parent_: istructs.RecordID(1),
							Data: map[string]interface{}{
								"id_lower_case_name": istructs.RecordID(200401),
								"name":               "FIRST-CHILDREN-2-401",
							},
							Containers_: map[string][]*coreutils.TestObject{
								"deep-children-1": {
									{
										Name:    istructs.NewQName("s", "deep-children-1"),
										Id:      istructs.RecordID(501),
										Parent_: istructs.RecordID(401),
										Data: map[string]interface{}{
											"id_lower_case_name": istructs.RecordID(200501),
											"name":               "DEEP-CHILDREN-1-501",
										},
										Containers_: map[string][]*coreutils.TestObject{
											"very-deep-children-1": {
												{
													Name:    istructs.NewQName("s", "very-deep-children-1"),
													Id:      istructs.RecordID(601),
													Parent_: istructs.RecordID(501),
													Data: map[string]interface{}{
														"id_lower_case_name": istructs.RecordID(200601),
														"name":               "VERY-DEEP-CHILDREN-1-601",
													},
												},
												{
													Name:    istructs.NewQName("s", "very-deep-children-1"),
													Id:      istructs.RecordID(602),
													Parent_: istructs.RecordID(501),
													Data: map[string]interface{}{
														"id_lower_case_name": istructs.RecordID(200602),
														"name":               "VERY-DEEP-CHILDREN-1-602",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			return workpiece{
				object: o,
				outputRow: &outputRow{
					keyToIdx: map[string]int{
						rootDocument:                       0,
						"first-children-1":                 1,
						"first-children-1/deep-children-1": 2,
						"first-children-1/deep-children-1/very-deep-children-1": 3,
						"first-children-2":                                      4,
						"first-children-2/deep-children-1":                      5,
						"first-children-2/deep-children-1/very-deep-children-1": 6,
					},
					values: []interface{}{
						[]IOutputRow{row(2001)},
						[]IOutputRow{row(200101), row(200102)},
						[]IOutputRow{row(200201)},
						[]IOutputRow{row(200301)},
						[]IOutputRow{row(200401)},
						[]IOutputRow{row(200501)},
						[]IOutputRow{row(200601), row(200602)},
					},
				},
				enrichedRootSchema: make(map[string]istructs.DataKindType),
			}
		}
		skb := &mockStateKeyBuilder{}
		skb.On("PutRecordID", mock.Anything, mock.Anything)
		s := &mockState{}
		s.
			On("KeyBuilder", state.RecordsStorage, istructs.NullQName).Return(skb).
			On("MustExist", mock.Anything).Return(record("root")).Once().
			On("MustExist", mock.Anything).Return(record("first-children-1-101")).Once().
			On("MustExist", mock.Anything).Return(record("first-children-1-102")).Once().
			On("MustExist", mock.Anything).Return(record("deep-children-1-201")).Once().
			On("MustExist", mock.Anything).Return(record("very-deep-children-1-301")).Once().
			On("MustExist", mock.Anything).Return(record("first-children-2-401")).Once().
			On("MustExist", mock.Anything).Return(record("deep-children-1-501")).Once().
			On("MustExist", mock.Anything).Return(record("very-deep-children-1-601")).Once().
			On("MustExist", mock.Anything).Return(record("very-deep-children-1-602")).Once()
		op := &EnrichmentOperator{
			state:        s,
			elements:     elements,
			schemasCache: newSchemasCache(schemas),
			metrics:      &testMetrics{},
		}

		outWork, err := op.DoAsync(context.Background(), work())

		require.NoError(err)
		require.Len(outWork.(IWorkpiece).OutputRow().Value(rootDocument).([]IOutputRow), 1)
		require.Equal("root", outWork.(IWorkpiece).OutputRow().Value(rootDocument).([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Len(outWork.(IWorkpiece).OutputRow().Value("first-children-1").([]IOutputRow), 2)
		require.Equal("first-children-1-101", outWork.(IWorkpiece).OutputRow().Value("first-children-1").([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Equal("first-children-1-102", outWork.(IWorkpiece).OutputRow().Value("first-children-1").([]IOutputRow)[1].Value("id_lower_case_name/name"))
		require.Len(outWork.(IWorkpiece).OutputRow().Value("first-children-1/deep-children-1").([]IOutputRow), 1)
		require.Equal("deep-children-1-201", outWork.(IWorkpiece).OutputRow().Value("first-children-1/deep-children-1").([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Len(outWork.(IWorkpiece).OutputRow().Value("first-children-1/deep-children-1/very-deep-children-1").([]IOutputRow), 1)
		require.Equal("very-deep-children-1-301", outWork.(IWorkpiece).OutputRow().Value("first-children-1/deep-children-1/very-deep-children-1").([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Len(outWork.(IWorkpiece).OutputRow().Value("first-children-2").([]IOutputRow), 1)
		require.Equal("first-children-2-401", outWork.(IWorkpiece).OutputRow().Value("first-children-2").([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Len(outWork.(IWorkpiece).OutputRow().Value("first-children-2/deep-children-1").([]IOutputRow), 1)
		require.Equal("deep-children-1-501", outWork.(IWorkpiece).OutputRow().Value("first-children-2/deep-children-1").([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Len(outWork.(IWorkpiece).OutputRow().Value("first-children-2/deep-children-1/very-deep-children-1").([]IOutputRow), 2)
		require.Equal("very-deep-children-1-601", outWork.(IWorkpiece).OutputRow().Value("first-children-2/deep-children-1/very-deep-children-1").([]IOutputRow)[0].Value("id_lower_case_name/name"))
		require.Equal("very-deep-children-1-602", outWork.(IWorkpiece).OutputRow().Value("first-children-2/deep-children-1/very-deep-children-1").([]IOutputRow)[1].Value("id_lower_case_name/name"))
	})
	t.Run("Should handle ctx error during row fill with ref fields", func(t *testing.T) {
		require := require.New(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		work := workpiece{
			outputRow: &outputRow{
				keyToIdx: map[string]int{rootDocument: 0},
				values: []interface{}{
					[]IOutputRow{&outputRow{}},
				},
			},
		}
		op := EnrichmentOperator{
			elements: []IElement{element{path: path{""}, refs: []IRefField{refField{"", "", ""}}}},
			metrics:  &testMetrics{},
		}

		outWork, err := op.DoAsync(ctx, work)

		require.Equal("context canceled", err.Error())
		require.Equal(work, outWork)
	})
}
