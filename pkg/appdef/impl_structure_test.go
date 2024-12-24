/*
 * Copyright (c) 2024-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef_test

import (
	"testing"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/testingu/require"
)

func Test_AppDef_StructuresAndRecords(t *testing.T) {
	require := require.New(t)

	wsName := appdef.NewQName("test", "workspace")
	docName, recName := appdef.NewQName("test", "doc"), appdef.NewQName("test", "rec")
	objName := appdef.NewQName("test", "obj")

	var app appdef.IAppDef

	t.Run("should be ok to add structures", func(t *testing.T) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		wsb := adb.AddWorkspace(wsName)

		doc := wsb.AddODoc(docName)
		doc.
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false)
		doc.AddContainer("rec", recName, 0, appdef.Occurs_Unbounded)
		rec := wsb.AddORecord(recName)
		rec.
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false)

		obj := wsb.AddObject(objName)
		obj.
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false)

		a, err := adb.Build()
		require.NoError(err)

		app = a
	})

	testWith := func(tested testedTypes) {
		t.Run("should be ok to find builded structures", func(t *testing.T) {
			findStruct := func(n appdef.QName, kind appdef.TypeKind) {
				typ := tested.Type(n)
				require.Equal(kind, typ.Kind())

				doc := appdef.Structure(tested.Type, n)
				require.Equal(kind, doc.Kind())

				require.Equal(wsName, doc.Workspace().QName())

				require.Equal(2, doc.UserFieldCount())
				require.Equal(appdef.DataKind_int64, doc.Field("f1").DataKind())
				require.Equal(appdef.DataKind_string, doc.Field("f2").DataKind())
			}
			findStruct(docName, appdef.TypeKind_ODoc)
			findStruct(recName, appdef.TypeKind_ORecord)
			findStruct(objName, appdef.TypeKind_Object)
		})

		require.Nil(appdef.Structure(tested.Type, appdef.NewQName("test", "unknown")), "should nil if not found")

		t.Run("should be ok to enumerate structures", func(t *testing.T) {
			var str []appdef.QName
			for s := range appdef.Structures(tested.Types()) {
				str = append(str, s.QName())
			}
			require.Equal(str, []appdef.QName{docName, objName, recName})
		})

		t.Run("should be ok to find builded records", func(t *testing.T) {
			findRecord := func(n appdef.QName, kind appdef.TypeKind) {
				typ := tested.Type(n)
				require.Equal(kind, typ.Kind())

				rec := appdef.Record(tested.Type, n)
				require.Equal(kind, rec.Kind())

				require.Equal(wsName, rec.Workspace().QName())

				require.Equal(2, rec.UserFieldCount())
				require.Equal(appdef.DataKind_int64, rec.Field("f1").DataKind())
				require.Equal(appdef.DataKind_string, rec.Field("f2").DataKind())
			}
			findRecord(docName, appdef.TypeKind_ODoc)
			findRecord(recName, appdef.TypeKind_ORecord)
		})

		require.Nil(appdef.Record(tested.Type, appdef.NewQName("test", "unknown")), "should nil if not found")
		require.Nil(appdef.Record(tested.Type, objName), "should nil if not record")

		t.Run("should be ok to enumerate records", func(t *testing.T) {
			var recs []appdef.QName
			for s := range appdef.Records(tested.Types()) {
				recs = append(recs, s.QName())
			}
			require.Equal(recs, []appdef.QName{docName, recName})
		})
	}

	testWith(app)
	testWith(app.Workspace(wsName))
}
