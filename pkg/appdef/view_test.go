/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddView(t *testing.T) {
	require := require.New(t)

	adb := New()

	numName := NewQName("test", "natural")
	_ = adb.AddData(numName, DataKind_int64, NullQName, MinExcl(0))

	digsData := NewQName("test", "digs")
	_ = adb.AddData(digsData, DataKind_string, NullQName, Pattern(`^\d+$`, "only digits allowed"))

	docName := NewQName("test", "doc")
	_ = adb.AddCDoc(docName)

	kbName := NewQName("test", "KB")
	_ = adb.AddData(kbName, DataKind_bytes, NullQName, MinLen(1), MaxLen(1024, "up to 1 KB"))

	viewName := NewQName("test", "view")
	vb := adb.AddView(viewName)

	t.Run("must be ok to build view", func(t *testing.T) {

		vb.SetComment("test view")

		t.Run("must be ok to add partition key fields", func(t *testing.T) {
			vb.KeyBuilder().PartKeyBuilder().AddDataField("pkF1", numName)
			vb.KeyBuilder().PartKeyBuilder().AddField("pkF2", DataKind_bool)

			t.Run("panic if field already exists in view", func(t *testing.T) {
				require.Panics(func() {
					vb.KeyBuilder().PartKeyBuilder().AddField("pkF1", DataKind_int64)
				})
			})

			t.Run("panic if variable length field added to pk", func(t *testing.T) {
				require.Panics(func() {
					vb.KeyBuilder().PartKeyBuilder().AddField("pkF3", DataKind_string)
				})
				require.Panics(func() {
					vb.KeyBuilder().PartKeyBuilder().AddDataField("pkF3", digsData)
				})
			})

			t.Run("panic if unknown data type field added to pk", func(t *testing.T) {
				require.Panics(func() {
					vb.KeyBuilder().PartKeyBuilder().AddDataField("pkF3", NewQName("test", "unknown"))
				})
			})
		})

		t.Run("must be ok to add clustering columns fields", func(t *testing.T) {
			vb.KeyBuilder().ClustColsBuilder().AddField("ccF1", DataKind_int64)
			vb.KeyBuilder().ClustColsBuilder().AddRefField("ccF2", docName)

			t.Run("panic if field already exists in view", func(t *testing.T) {
				require.Panics(func() {
					vb.KeyBuilder().ClustColsBuilder().AddField("ccF1", DataKind_int64)
				})
			})

			t.Run("panic if unknown data type field added to cc", func(t *testing.T) {
				require.Panics(func() {
					vb.KeyBuilder().ClustColsBuilder().AddDataField("ccF3", NewQName("test", "unknown"))
				})
			})
		})

		t.Run("must be ok to add value fields", func(t *testing.T) {
			vb.ValueBuilder().
				AddField("valF1", DataKind_bool, true).
				AddDataField("valF2", digsData, false, MaxLen(100)).SetFieldComment("valF2", "up to 100 digits")
		})
	})

	app, err := adb.Build()
	require.NoError(err)
	view := app.View(viewName)

	t.Run("must be ok to read view", func(t *testing.T) {
		require.Equal("test view", view.Comment())
		require.Equal(viewName, view.QName())
		require.Equal(TypeKind_ViewRecord, view.Kind())

		checkValueValF2 := func(f IField) {
			require.Equal("valF2", f.Name())
			require.False(f.Required())
			cnt := 0
			f.Constraints(func(c IConstraint) {
				cnt++
				switch cnt {
				case 1:
					require.Equal(ConstraintKind_MaxLen, c.Kind())
					require.EqualValues(100, c.Value())
				case 2:
					require.Equal(ConstraintKind_Pattern, c.Kind())
					require.EqualValues(`^\d+$`, c.Value().(*regexp.Regexp).String())
					require.Equal("only digits allowed", c.Comment())
				default:
					require.Fail("unexpected constraint", "constraint: %v", c)
				}
			})
			require.Equal("up to 100 digits", f.Comment())
		}

		require.Equal(7, view.FieldCount())
		cnt := 0
		view.Fields(func(f IField) {
			cnt++
			switch cnt {
			case 1:
				require.Equal(SystemField_QName, f.Name())
				require.True(f.IsSys())
			case 2:
				require.Equal("pkF1", f.Name())
				require.Equal(numName, f.Data().QName())
				require.True(f.Required())
			case 3:
				require.Equal("pkF2", f.Name())
				require.True(f.Required())
			case 4:
				require.Equal("ccF1", f.Name())
				require.False(f.Required())
			case 5:
				require.Equal("ccF2", f.Name())
				require.False(f.Required())
			case 6:
				require.Equal("valF1", f.Name())
				require.True(f.Required())
			case 7:
				checkValueValF2(f)
			default:
				require.Fail("unexpected field «%s»", f.Name())
			}
		})
		require.Equal(view.FieldCount(), cnt)

		t.Run("must be ok to read view full key", func(t *testing.T) {
			key := view.Key()
			require.Equal(4, key.FieldCount())
			cnt := 0
			key.Fields(func(f IField) {
				cnt++
				switch cnt {
				case 1:
					require.Equal("pkF1", f.Name())
					require.Equal(numName, f.Data().QName())
					require.True(f.Required())
				case 2:
					require.Equal("pkF2", f.Name())
					require.True(f.Required())
				case 3:
					require.Equal("ccF1", f.Name())
					require.False(f.Required())
				case 4:
					require.Equal("ccF2", f.Name())
					require.False(f.Required())
				default:
					require.Fail("unexpected field «%s»", f.Name())
				}
			})
			require.Equal(key.FieldCount(), cnt)
		})

		t.Run("must be ok to read view partition key", func(t *testing.T) {
			pk := view.Key().PartKey()
			require.Equal(2, pk.FieldCount())
			cnt := 0
			pk.Fields(func(f IField) {
				cnt++
				switch cnt {
				case 1:
					require.Equal("pkF1", f.Name())
					require.Equal(numName, f.Data().QName())
					require.True(f.Required())
				case 2:
					require.Equal("pkF2", f.Name())
					require.True(f.Required())
				default:
					require.Fail("unexpected field «%s»", f.Name())
				}
			})
			require.Equal(pk.FieldCount(), cnt)
		})

		t.Run("must be ok to read view clustering columns", func(t *testing.T) {
			cc := view.Key().ClustCols()
			require.Equal(2, cc.FieldCount())
			cnt := 0
			cc.Fields(func(f IField) {
				cnt++
				switch cnt {
				case 1:
					require.Equal("ccF1", f.Name())
					require.False(f.Required())
				case 2:
					require.Equal("ccF2", f.Name())
					require.False(f.Required())
				default:
					require.Fail("unexpected field «%s»", f.Name())
				}
			})
			require.Equal(cc.FieldCount(), cnt)
		})

		t.Run("must be ok to read view value", func(t *testing.T) {
			val := view.Value()
			require.Equal(3, val.FieldCount())
			cnt := 0
			val.Fields(func(f IField) {
				cnt++
				switch cnt {
				case 1:
					require.Equal(SystemField_QName, f.Name())
					require.True(f.IsSys())
				case 2:
					require.Equal("valF1", f.Name())
					require.True(f.Required())
				case 3:
					checkValueValF2(f)
				default:
					require.Fail("unexpected field «%s»", f.Name())
				}
			})
			require.Equal(val.FieldCount(), cnt)
		})

		t.Run("must be ok to cast Type() as IView", func(t *testing.T) {
			typ := app.Type(viewName)
			require.NotNil(typ)
			require.Equal(TypeKind_ViewRecord, typ.Kind())

			v, ok := typ.(IView)
			require.True(ok)
			require.Equal(v, view)
		})

		require.Nil(adb.View(NewQName("test", "unknown")), "find unknown view must return nil")

		t.Run("must be nil if not view", func(t *testing.T) {
			require.Nil(app.View(docName))

			typ := app.Type(docName)
			require.NotNil(typ)
			v, ok := typ.(IView)
			require.False(ok)
			require.Nil(v)
		})
	})

	t.Run("must be ok to add fields to view after app build", func(t *testing.T) {
		vb.KeyBuilder().PartKeyBuilder().
			AddRefField("pkF3", docName).
			SetFieldComment("pkF3", "test comment")

		vb.KeyBuilder().ClustColsBuilder().
			AddDataField("ccF3", kbName).SetFieldComment("ccF3", "one KB")

		t.Run("panic if add second variable length field", func(t *testing.T) {
			require.Panics(func() {
				vb.KeyBuilder().ClustColsBuilder().AddField("ccF3_1", DataKind_bytes)
			})
			require.Panics(func() {
				vb.KeyBuilder().ClustColsBuilder().AddDataField("ccF3_1", kbName)
			})
		})

		vb.ValueBuilder().
			AddRefField("valF3", false, docName).
			AddField("valF4", DataKind_bytes, false, MaxLen(1024)).SetFieldComment("valF4", "test comment").
			AddField("valF5", DataKind_bool, false).SetFieldVerify("valF5", VerificationKind_EMail)

		_, err := adb.Build()
		require.NoError(err)

		require.Equal(3, view.Key().PartKey().FieldCount())
		require.Equal(3, view.Key().ClustCols().FieldCount())
		require.Equal(6, view.Key().FieldCount())

		require.Equal(view.Key().FieldCount()+view.Value().FieldCount(), view.FieldCount())

		require.Equal("test comment", view.Key().Field("pkF3").Comment())
		require.Equal("one KB", view.Key().Field("ccF3").Comment())

		require.Equal(5, view.Value().UserFieldCount())

		view.Value().Fields(func(f IField) {
			switch f.Name() {
			case SystemField_QName:
				require.Equal(DataKind_QName, f.DataKind())
				require.True(f.IsSys())
			case "valF1":
				require.Equal(DataKind_bool, f.DataKind())
				require.True(f.Required())
			case "valF2":
				require.Equal(DataKind_string, f.DataKind())
				require.False(f.Required())
				// valF2 constraints checked above
			case "valF3":
				require.Equal(DataKind_RecordID, f.DataKind())
				require.False(f.Required())
				require.EqualValues(QNames{docName}, f.(IRefField).Refs())
			case "valF4":
				require.Equal(DataKind_bytes, f.DataKind())
				require.False(f.Required())
				require.Equal("test comment", f.Comment())
				cnt := 0
				f.Constraints(func(c IConstraint) {
					cnt++
					switch cnt {
					case 1:
						require.Equal(ConstraintKind_MaxLen, c.Kind())
						require.EqualValues(1024, c.Value())
					default:
						require.Fail("unexpected constraint", "constraint: %v", c)
					}
				})
				require.EqualValues(1, cnt)
			case "valF5":
				require.Equal(DataKind_bool, f.DataKind())
				require.False(f.Required())
				require.True(f.Verifiable())
				require.True(f.VerificationKind(VerificationKind_EMail))
				require.False(f.VerificationKind(VerificationKind_Phone))
			default:
				require.Fail("unexpected value field", "field name: %s", f.Name())
			}
		})
	})
}

func TestViewValidate(t *testing.T) {
	require := require.New(t)

	app := New()
	viewName := NewQName("test", "view")
	v := app.AddView(viewName)
	require.NotNil(v)

	t.Run("must be error if no pkey fields", func(t *testing.T) {
		_, err := app.Build()
		require.ErrorIs(err, ErrFieldsMissed)
	})

	v.KeyBuilder().PartKeyBuilder().AddField("pk1", DataKind_bool)

	t.Run("must be error if no ccols fields", func(t *testing.T) {
		_, err := app.Build()
		require.ErrorIs(err, ErrFieldsMissed)
	})

	v.KeyBuilder().ClustColsBuilder().AddField("cc1", DataKind_string, MaxLen(100))
	_, err := app.Build()
	require.NoError(err)
}
