/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Short test form. Full test ref. to gdoc_test.go
func Test_AppDef_AddCDoc(t *testing.T) {
	require := require.New(t)

	docName, recName := NewQName("test", "doc"), NewQName("test", "rec")

	var app IAppDef

	t.Run("must be ok to add document", func(t *testing.T) {
		apb := New()
		doc := apb.AddCDoc(docName)
		doc.
			AddField("f1", DataKind_int64, true).
			AddField("f2", DataKind_string, false)
		doc.AddContainer("rec", recName, 0, Occurs_Unbounded)
		rec := apb.AddCRecord(recName)
		rec.
			AddField("f1", DataKind_int64, true).
			AddField("f2", DataKind_string, false)

		a, err := apb.Build()
		require.NoError(err)

		app = a
	})

	t.Run("must be ok to find builded doc", func(t *testing.T) {
		def := app.Def(docName)
		require.Equal(DefKind_CDoc, def.Kind())

		doc := app.CDoc(docName)
		require.Equal(DefKind_CDoc, doc.Kind())
		require.Equal(def.(ICDoc), doc)

		require.Equal(2, doc.UserFieldCount())
		require.Equal(DataKind_int64, doc.Field("f1").DataKind())

		require.Equal(DefKind_CRecord, doc.ContainerDef("rec").Kind())

		t.Run("must be ok to find builded record", func(t *testing.T) {
			def := app.Def(recName)
			require.Equal(DefKind_CRecord, def.Kind())

			rec := app.CRecord(recName)
			require.Equal(DefKind_CRecord, rec.Kind())
			require.Equal(def.(ICRecord), rec)

			require.Equal(2, rec.UserFieldCount())
			require.Equal(DataKind_int64, rec.Field("f1").DataKind())

			require.Equal(0, rec.ContainerCount())
		})
	})
}

func Test_AppDef_AddSingleton(t *testing.T) {
	require := require.New(t)

	docName := NewQName("test", "doc")

	var app IAppDef

	t.Run("must be ok to add singleton", func(t *testing.T) {
		appDef := New()
		doc := appDef.AddSingleton(docName)
		doc.
			AddField("f1", DataKind_int64, true).
			AddField("f2", DataKind_string, false)

		a, err := appDef.Build()
		require.NoError(err)

		app = a
	})

	t.Run("must be ok to find builded singleton", func(t *testing.T) {
		def := app.Def(docName)
		require.Equal(DefKind_CDoc, def.Kind())

		doc := app.CDoc(docName)
		require.Equal(DefKind_CDoc, doc.Kind())
		require.Equal(def.(ICDoc), doc)

		require.True(doc.Singleton())
	})
}
