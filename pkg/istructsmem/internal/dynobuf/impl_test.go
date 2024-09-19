/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package dynobuf

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/untillpro/dynobuffers"
	"github.com/voedger/voedger/pkg/appdef"
)

func TestDynoBufSchemes(t *testing.T) {
	require := require.New(t)

	var appDef appdef.IAppDef

	t.Run("must ok to build application", func(t *testing.T) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		root := adb.AddObject(appdef.NewQName("test", "root"))
		root.
			AddField("int32Field", appdef.DataKind_int32, true).
			AddField("int64Field", appdef.DataKind_int64, false).
			AddField("float32Field", appdef.DataKind_float32, false).
			AddField("float64Field", appdef.DataKind_float64, false).
			AddField("bytesField", appdef.DataKind_bytes, false).
			AddField("strField", appdef.DataKind_string, false).
			AddField("qnameField", appdef.DataKind_QName, false).
			AddField("recIDField", appdef.DataKind_RecordID, false)
		root.
			AddContainer("child", appdef.NewQName("test", "child"), 1, appdef.Occurs_Unbounded)

		child := adb.AddObject(appdef.NewQName("test", "child"))
		child.
			AddField("int32Field", appdef.DataKind_int32, true).
			AddField("int64Field", appdef.DataKind_int64, false).
			AddField("float32Field", appdef.DataKind_float32, false).
			AddField("float64Field", appdef.DataKind_float64, false).
			AddField("bytesField", appdef.DataKind_bytes, false).
			AddField("strField", appdef.DataKind_string, false).
			AddField("qnameField", appdef.DataKind_QName, false).
			AddField("boolField", appdef.DataKind_bool, false).
			AddField("recIDField", appdef.DataKind_RecordID, false)
		child.
			AddContainer("grandChild", appdef.NewQName("test", "grandChild"), 0, 1)

		grandChild := adb.AddObject(appdef.NewQName("test", "grandChild"))
		grandChild.
			AddField("recIDField", appdef.DataKind_RecordID, false)

		view := adb.AddView(appdef.NewQName("test", "view"))
		view.Key().PartKey().AddField("pk1", appdef.DataKind_int64)
		view.Key().ClustCols().AddField("cc1", appdef.DataKind_string, appdef.MaxLen(100))
		view.Value().AddRefField("val1", true)

		sch, err := adb.Build()
		require.NoError(err)

		appDef = sch
	})

	schemes := newSchemes()
	require.NotNil(schemes)

	schemes.Prepare(appDef)

	checkScheme := func(name appdef.QName, fields appdef.IFields, dynoScheme *dynobuffers.Scheme) {
		require.NotNil(dynoScheme, "dynobuffer scheme for «%v» not found", name)

		require.EqualValues(len(dynoScheme.FieldsMap), fields.UserFieldCount())

		for _, f := range dynoScheme.Fields {
			fld := fields.Field(f.Name)
			require.NotNil(fld)
			require.Equal(DataKindToFieldType(fld.DataKind()), f.Ft)
		}

		for _, fld := range fields.Fields() {
			if !fld.IsSys() {
				f, ok := dynoScheme.FieldsMap[fld.Name()]
				require.True(ok)
				require.Equal(DataKindToFieldType(fld.DataKind()), f.Ft)
			}
		}
	}

	appDef.Types(
		func(typ appdef.IType) bool {
			name := typ.QName()
			if view, ok := typ.(appdef.IView); ok {
				checkScheme(name, view.Key().PartKey(), schemes.ViewPartKeyScheme(name))
				checkScheme(name, view.Key().ClustCols(), schemes.ViewClustColsScheme(name))
				checkScheme(name, view.Value(), schemes.Scheme(name))
				return true
			}
			if fld, ok := typ.(appdef.IFields); ok {
				checkScheme(name, fld, schemes.Scheme(name))
			}
			return true
		})
}
