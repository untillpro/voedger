/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_def_AddUnique(t *testing.T) {
	require := require.New(t)

	qName := NewQName("test", "user")
	un1 := UniqueQName(qName, "EMail")
	un2 := UniqueQName(qName, "Full")

	adb := New()
	adb.AddPackage("test", "test.com/test")

	doc := adb.AddCDoc(qName)
	require.NotNil(doc)

	doc.
		AddField("name", DataKind_string, true).
		AddField("surname", DataKind_string, false).
		AddField("lastName", DataKind_string, false).
		AddField("birthday", DataKind_int64, false).
		AddField("sex", DataKind_bool, false).
		AddField("eMail", DataKind_string, false)
	doc.
		AddUnique(un1, []string{"eMail"}).
		AddUnique(un2, []string{"name", "surname", "lastName"})

	t.Run("test is ok", func(t *testing.T) {
		app, err := adb.Build()
		require.NoError(err)

		doc := app.CDoc(qName)
		require.NotEqual(TypeKind_null, doc.Kind())

		require.Equal(2, doc.UniqueCount())

		u := doc.UniqueByName(un2)
		require.Len(u.Fields(), 3)
		require.Equal("lastName", u.Fields()[0].Name())
		require.Equal("name", u.Fields()[1].Name())
		require.Equal("surname", u.Fields()[2].Name())

		require.Equal(doc.UniqueCount(), func() int {
			cnt := 0
			for n, u := range doc.Uniques() {
				cnt++
				require.Equal(n, u.Name())
				switch n {
				case un1:
					require.Len(u.Fields(), 1)
					require.Equal("eMail", u.Fields()[0].Name())
					require.Equal(DataKind_string, u.Fields()[0].DataKind())
				case un2:
					require.Len(u.Fields(), 3)
					require.Equal("lastName", u.Fields()[0].Name())
					require.Equal("name", u.Fields()[1].Name())
					require.Equal("surname", u.Fields()[2].Name())
				}
			}
			return cnt
		}())
	})

	t.Run("test panics", func(t *testing.T) {

		require.Panics(func() {
			doc.AddUnique(NullQName, []string{"sex"})
		}, "panics if empty unique name")

		require.Panics(func() {
			doc.AddUnique(NewQName("naked", "🔫"), []string{"sex"})
		}, "panics if invalid unique name")

		require.Panics(func() {
			doc.AddUnique(un1, []string{"name", "surname", "lastName"})
		}, "panics unique with name is already exists")

		require.Panics(func() {
			doc.AddUnique(qName, []string{"name", "surname", "lastName"})
		}, "panics if type with unique name is exists")

		require.Panics(func() {
			doc.AddUnique(NewQName("test", "user$uniqueEmpty"), []string{})
		}, "panics if fields set is empty")

		require.Panics(func() {
			doc.AddUnique(NewQName("test", "user$uniqueFiledDup"), []string{"birthday", "birthday"})
		}, "if fields has duplicates")

		t.Run("panics if too many fields", func(t *testing.T) {
			adb := New()
			adb.AddPackage("test", "test.com/test")
			rec := adb.AddCRecord(NewQName("test", "rec"))
			fldNames := []string{}
			for i := 0; i <= MaxTypeUniqueFieldsCount; i++ {
				n := fmt.Sprintf("f_%#x", i)
				rec.AddField(n, DataKind_bool, false)
				fldNames = append(fldNames, n)
			}
			require.Panics(func() { rec.AddUnique(NewQName("test", "user$uniqueTooLong"), fldNames) })
		})

		require.Panics(func() {
			doc.AddUnique(NewQName("test", "user$uniqueFieldsSetDup"), []string{"name", "surname", "lastName"})
		}, "if fields set is already exists")

		require.Panics(func() {
			doc.AddUnique(NewQName("test", "user$uniqueFieldsSetOverlaps"), []string{"surname"})
		}, "if fields set overlaps exists")

		require.Panics(func() {
			doc.AddUnique(NewQName("test", "user$uniqueFieldsSetOverlapped"), []string{"eMail", "birthday"})
		}, "if fields set overlapped by exists")

		require.Panics(func() {
			doc.AddUnique(NewQName("test", "user$uniqueFieldsUnknown"), []string{"unknown"})
		}, "if fields not exists")

		t.Run("panics if too many uniques", func(t *testing.T) {
			adb := New()
			adb.AddPackage("test", "test.com/test")
			rec := adb.AddCRecord(NewQName("test", "rec"))
			for i := 0; i < MaxTypeUniqueCount; i++ {
				n := fmt.Sprintf("f_%#x", i)
				rec.AddField(n, DataKind_int32, false)
				rec.AddUnique(NewQName("test", fmt.Sprintf("rec$uniques$%s", n)), []string{n})
			}
			rec.AddField("lastStraw", DataKind_int32, false)
			require.Panics(func() { rec.AddUnique(NewQName("test", "rec$uniques$lastStraw"), []string{"lastStraw"}) })
		})
	})
}

func Test_type_UniqueField(t *testing.T) {
	// This tests old-style uniques. See [issue #173](https://github.com/voedger/voedger/issues/173)
	require := require.New(t)

	qName := NewQName("test", "user")

	adb := New()
	adb.AddPackage("test", "test.com/test")

	doc := adb.AddCDoc(qName)
	require.NotNil(doc)

	doc.
		AddField("name", DataKind_string, true).
		AddField("surname", DataKind_string, false).
		AddField("lastName", DataKind_string, false).
		AddField("birthday", DataKind_int64, false).
		AddField("sex", DataKind_bool, false).
		AddField("eMail", DataKind_string, true)
	doc.SetUniqueField("eMail")

	t.Run("test is ok", func(t *testing.T) {
		app, err := adb.Build()
		require.NoError(err)

		d := app.CDoc(qName)
		require.NotEqual(TypeKind_null, d.Kind())

		fld := d.UniqueField()
		require.Equal("eMail", fld.Name())
		require.True(fld.Required())
	})

	t.Run("must be ok to clear unique field", func(t *testing.T) {
		doc.SetUniqueField("")

		app, err := adb.Build()
		require.NoError(err)

		d := app.CDoc(qName)
		require.NotEqual(TypeKind_null, d.Kind())

		require.Nil(d.UniqueField())
	})

	t.Run("test panics", func(t *testing.T) {
		require.Panics(func() {
			doc.SetUniqueField("naked-🔫")
		}, "panics if invalid unique field name")

		require.Panics(func() {
			doc.SetUniqueField("unknownField")
		}, "panics if unknown unique field name")
	})
}

func Test_duplicates(t *testing.T) {
	require := require.New(t)

	require.Negative(duplicates([]string{"a"}))
	require.Negative(duplicates([]string{"a", "b"}))
	require.Negative(duplicates([]int{0, 1, 2}))

	i, j := duplicates([]int{0, 1, 0})
	require.True(i == 0 && j == 2)

	i, j = duplicates([]int{0, 1, 2, 1})
	require.True(i == 1 && j == 3)

	i, j = duplicates([]bool{true, true})
	require.True(i == 0 && j == 1)

	i, j = duplicates([]string{"a", "b", "c", "c"})
	require.True(i == 2 && j == 3)
}

func Test_subSet(t *testing.T) {
	require := require.New(t)

	t.Run("check empty slices", func(t *testing.T) {
		require.True(subSet([]int{}, []int{}))
		require.True(subSet(nil, []string{}))
		require.True(subSet([]bool{}, nil))
		require.True(subSet[int](nil, nil))

		require.True(subSet(nil, []string{"a", "b"}))
		require.True(subSet([]bool{}, []bool{true, false}))
	})

	t.Run("must be true", func(t *testing.T) {
		require.True(subSet([]int{1}, []int{1}))
		require.True(subSet([]string{"a"}, []string{"a", "b"}))
		require.True(subSet([]int{1, 2, 3}, []int{0, 1, 2, 3, 4}))
	})

	t.Run("must be false", func(t *testing.T) {
		require.False(subSet([]int{1}, []int{}))
		require.False(subSet([]string{"a"}, []string{"b", "c"}))
		require.False(subSet([]int{1, 2, 3}, []int{0, 2, 4, 6, 8}))
	})
}

func Test_overlaps(t *testing.T) {
	require := require.New(t)

	t.Run("check empty slices", func(t *testing.T) {
		require.True(overlaps([]int{}, []int{}))
		require.True(overlaps(nil, []string{}))
		require.True(overlaps([]bool{}, nil))
		require.True(overlaps[int](nil, nil))

		require.True(overlaps(nil, []string{"a", "b"}))
		require.True(overlaps([]bool{true, false}, []bool{}))
	})

	t.Run("must be true", func(t *testing.T) {
		require.True(overlaps([]int{1}, []int{1}))
		require.True(overlaps([]string{"a"}, []string{"a", "b"}))
		require.True(overlaps([]int{0, 1, 2, 3, 4}, []int{1, 2, 3}))
	})

	t.Run("must be false", func(t *testing.T) {
		require.False(overlaps([]int{1}, []int{2}))
		require.False(overlaps([]string{"a"}, []string{"b", "c"}))
		require.False(overlaps([]int{1, 2, 3}, []int{7, 0, 3, 2, 0, -1}))
	})
}
