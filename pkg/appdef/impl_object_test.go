/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/appdef"
)

func Test_AppDef_AddObject(t *testing.T) {
	require := require.New(t)

	wsName := appdef.NewQName("test", "workspace")
	rootName, childName := appdef.NewQName("test", "root"), appdef.NewQName("test", "child")

	var app appdef.IAppDef

	t.Run("should be ok to add objects", func(t *testing.T) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")

		wsb := adb.AddWorkspace(wsName)

		root := wsb.AddObject(rootName)
		root.
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false)
		root.AddContainer("child", childName, 0, appdef.Occurs_Unbounded)
		child := wsb.AddObject(childName)
		child.
			AddField("f1", appdef.DataKind_int64, true).
			AddField("f2", appdef.DataKind_string, false)

		a, err := adb.Build()
		require.NoError(err)

		app = a
	})

	testWith := func(tested testedTypes) {

		t.Run("should be ok to find builded root object", func(t *testing.T) {
			typ := tested.Type(rootName)
			require.Equal(appdef.TypeKind_Object, typ.Kind())

			root := appdef.Object(tested.Type, rootName)
			require.Equal(appdef.TypeKind_Object, root.Kind())
			require.Equal(typ.(appdef.IObject), root)

			require.Equal(wsName, root.Workspace().QName())

			require.NotNil(root.Field(appdef.SystemField_QName))

			require.Equal(2, root.UserFieldCount())
			require.Equal(appdef.DataKind_int64, root.Field("f1").DataKind())

			require.Equal(appdef.TypeKind_Object, root.Container("child").Type().Kind())

			t.Run("should be ok to find builded child object", func(t *testing.T) {
				typ := tested.Type(childName)
				require.Equal(appdef.TypeKind_Object, typ.Kind())

				child := appdef.Object(tested.Type, childName)
				require.Equal(appdef.TypeKind_Object, child.Kind())
				require.Equal(typ.(appdef.IObject), child)

				require.Equal(wsName, child.Workspace().QName())

				require.NotNil(child.Field(appdef.SystemField_QName))
				require.NotNil(child.Field(appdef.SystemField_Container))

				require.Equal(2, child.UserFieldCount())
				require.Equal(appdef.DataKind_int64, child.Field("f1").DataKind())

				require.Zero(child.ContainerCount())
			})
		})

		t.Run("should be ok to enumerate objects", func(t *testing.T) {
			var objects []appdef.QName
			for obj := range appdef.Objects(tested.Types()) {
				objects = append(objects, obj.QName())
			}
			require.Len(objects, 2)
			require.Equal(childName, objects[0])
			require.Equal(rootName, objects[1])
		})

		require.Nil(appdef.Object(tested.Type, appdef.NewQName("test", "unknown")), "should be nil if unknown")
	}

	testWith(app)
	testWith(app.Workspace(wsName))
}
