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
func Test_AppDef_AddWDoc(t *testing.T) {
	require := require.New(t)

	docName, recName := NewQName("test", "doc"), NewQName("test", "rec")

	var app IAppDef

	t.Run("must be ok to add document", func(t *testing.T) {
		appDef := New()
		doc := appDef.AddWDoc(docName)
		doc.
			AddField("f1", DataKind_int64, true).
			AddField("f2", DataKind_string, false)
		doc.AddContainer("rec", recName, 0, Occurs_Unbounded)
		rec := appDef.AddWRecord(recName)
		rec.
			AddField("f1", DataKind_int64, true).
			AddField("f2", DataKind_string, false)

		a, err := appDef.Build()
		require.NoError(err)

		app = a
	})

	t.Run("must be ok to find builded doc", func(t *testing.T) {
		def := app.Def(docName)
		require.Equal(DefKind_WDoc, def.Kind())

		doc := app.WDoc(docName)
		require.Equal(DefKind_WDoc, doc.Kind())
		require.Equal(def.(IWDoc), doc)

		require.Equal(2, doc.UserFieldCount())
		require.Equal(DataKind_int64, doc.Field("f1").DataKind())

		require.Equal(DefKind_WRecord, doc.Container("rec").Def().Kind())

		t.Run("must be ok to find builded record", func(t *testing.T) {
			def := app.Def(recName)
			require.Equal(DefKind_WRecord, def.Kind())

			rec := app.WRecord(recName)
			require.Equal(DefKind_WRecord, rec.Kind())
			require.Equal(def.(IWRecord), rec)

			require.Equal(2, rec.UserFieldCount())
			require.Equal(DataKind_int64, rec.Field("f1").DataKind())

			require.Equal(0, rec.ContainerCount())
		})
	})
}
