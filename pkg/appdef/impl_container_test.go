/*
 * Copyright (c) 2023-present Sigma-Soft, Ltd.
 * @author: Nikolay Nikitin
 */

package appdef_test

import (
	"fmt"
	"testing"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/goutils/testingu/require"
)

func Test_type_AddContainer(t *testing.T) {
	require := require.New(t)

	wsName := appdef.NewQName("test", "workspace")
	rootName := appdef.NewQName("test", "root")
	childName := appdef.NewQName("test", "child")

	t.Run("should be ok to add container", func(t *testing.T) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")
		wsb := adb.AddWorkspace(wsName)
		root := wsb.AddObject(rootName)
		_ = wsb.AddObject(childName)

		root.AddContainer("c1", childName, 1, appdef.Occurs_Unbounded)

		app, err := adb.Build()
		require.NoError(err)
		require.NotNil(app)

		t.Run("should be ok to find builded container", func(t *testing.T) {
			r := appdef.Object(app.Type, rootName)
			require.NotNil(r)

			require.EqualValues(1, r.ContainerCount())

			c1 := r.Container("c1")
			require.NotNil(c1)
			require.EqualValues(childName, c1.Type().QName())
		})
	})

	t.Run("should be ok to add containers use chain notation", func(t *testing.T) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")
		wsb := adb.AddWorkspace(wsName)
		_ = wsb.AddObject(childName)
		_ = wsb.AddObject(rootName).
			AddContainer("c1", childName, 1, appdef.Occurs_Unbounded).
			AddContainer("c2", childName, 1, appdef.Occurs_Unbounded).
			AddContainer("c3", childName, 1, appdef.Occurs_Unbounded)

		app, err := adb.Build()
		require.NoError(err)

		t.Run("should be ok to find builded containder", func(t *testing.T) {
			obj := appdef.Object(app.Type, rootName)
			require.NotNil(obj)
			require.EqualValues(3, obj.ContainerCount())
			require.NotNil(obj.Container("c1"))
			require.NotNil(obj.Container("c2"))
			require.NotNil(obj.Container("c3"))
			require.Nil(obj.Container("unknown"))
		})
	})

	t.Run("should be panics", func(t *testing.T) {
		adb := appdef.New()
		adb.AddPackage("test", "test.com/test")
		wsb := adb.AddWorkspace(wsName)
		root := wsb.AddObject(rootName)
		_ = wsb.AddObject(childName)

		t.Run("if empty container name", func(t *testing.T) {
			require.Panics(func() { root.AddContainer("", childName, 1, appdef.Occurs_Unbounded) },
				require.Is(appdef.ErrMissedError))
		})

		t.Run("if invalid container name", func(t *testing.T) {
			require.Panics(func() { root.AddContainer("naked_🔫", childName, 1, appdef.Occurs_Unbounded) },
				require.Is(appdef.ErrInvalidError))
		})

		t.Run("if container name dupe", func(t *testing.T) {
			root.AddContainer("dupe", childName, 1, appdef.Occurs_Unbounded)
			require.Panics(func() { root.AddContainer("dupe", childName, 1, appdef.Occurs_Unbounded) },
				require.Is(appdef.ErrAlreadyExistsError),
				require.Has("dupe"))
		})

		t.Run("if container type name missed", func(t *testing.T) {
			require.Panics(func() { root.AddContainer("c2", appdef.NullQName, 1, appdef.Occurs_Unbounded) },
				require.Is(appdef.ErrMissedError),
				require.Has("c2"))
		})

		t.Run("if invalid occurrences", func(t *testing.T) {
			require.Panics(func() { root.AddContainer("c2", childName, 1, 0) },
				require.Is(appdef.ErrOutOfBoundsError))
			require.Panics(func() { root.AddContainer("c3", childName, 2, 1) },
				require.Is(appdef.ErrOutOfBoundsError))
		})

		t.Run("if container type is incompatible", func(t *testing.T) {
			docName := appdef.NewQName("test", "doc")
			_ = wsb.AddCDoc(docName)
			require.Panics(func() { root.AddContainer("c2", docName, 1, 1) },
				require.Is(appdef.ErrInvalidError),
				require.Has(docName.String()))
		})

		t.Run("if too many containers", func(t *testing.T) {
			o := appdef.New().AddWorkspace(wsName).AddObject(childName)
			for i := 0; i < appdef.MaxTypeContainerCount; i++ {
				o.AddContainer(fmt.Sprintf("c_%#x", i), childName, 0, appdef.Occurs_Unbounded)
			}
			require.Panics(func() { o.AddContainer("errorContainer", childName, 0, appdef.Occurs_Unbounded) },
				require.Is(appdef.ErrTooManyError))
		})
	})
}

func TestValidateContainer(t *testing.T) {
	require := require.New(t)

	adb := appdef.New()
	adb.AddPackage("test", "test.com/test")

	wsb := adb.AddWorkspace(appdef.NewQName("test", "workspace"))

	doc := wsb.AddCDoc(appdef.NewQName("test", "doc"))
	doc.AddContainer("rec", appdef.NewQName("test", "rec"), 0, appdef.Occurs_Unbounded)

	t.Run("should be error if container type not found", func(t *testing.T) {
		_, err := adb.Build()
		require.Error(err, require.Is(appdef.ErrNotFoundError), require.Has("test.rec"))
	})

	rec := wsb.AddCRecord(appdef.NewQName("test", "rec"))
	_, err := adb.Build()
	require.NoError(err)

	t.Run("should be ok container recurse", func(t *testing.T) {
		rec.AddContainer("rec", appdef.NewQName("test", "rec"), 0, appdef.Occurs_Unbounded)
		_, err := adb.Build()
		require.NoError(err)
	})

	t.Run("should be ok container sub recurse", func(t *testing.T) {
		rec.AddContainer("rec1", appdef.NewQName("test", "rec1"), 0, appdef.Occurs_Unbounded)
		rec1 := wsb.AddCRecord(appdef.NewQName("test", "rec1"))
		rec1.AddContainer("rec", appdef.NewQName("test", "rec"), 0, appdef.Occurs_Unbounded)
		_, err := adb.Build()
		require.NoError(err)
	})

	t.Run("should be error if container kind is incompatible", func(t *testing.T) {
		doc.AddContainer("obj", appdef.NewQName("test", "obj"), 0, 1)
		_ = wsb.AddObject(appdef.NewQName("test", "obj"))
		_, err := adb.Build()
		require.Error(err, require.Is(appdef.ErrInvalidError), require.Has("test.obj"))
	})
}

func TestOccurs_String(t *testing.T) {
	tests := []struct {
		name string
		o    appdef.Occurs
		want string
	}{
		{
			name: "0 —> `0`",
			o:    0,
			want: `0`,
		},
		{
			name: "1 —> `1`",
			o:    1,
			want: `1`,
		},
		{
			name: "∞ —> `unbounded`",
			o:    appdef.Occurs_Unbounded,
			want: `unbounded`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.o.String(); got != tt.want {
				t.Errorf("Occurs.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOccurs_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		o       appdef.Occurs
		want    string
		wantErr bool
	}{
		{
			name:    "0 —> `0`",
			o:       0,
			want:    `0`,
			wantErr: false,
		},
		{
			name:    "1 —> `1`",
			o:       1,
			want:    `1`,
			wantErr: false,
		},
		{
			name:    "∞ —> `unbounded`",
			o:       appdef.Occurs_Unbounded,
			want:    `"unbounded"`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.o.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("Occurs.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("Occurs.MarshalJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOccurs_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    appdef.Occurs
		wantErr bool
	}{
		{
			name:    "0 —> 0",
			data:    `0`,
			want:    0,
			wantErr: false,
		},
		{
			name:    "1 —> 1",
			data:    `1`,
			want:    1,
			wantErr: false,
		},
		{
			name:    `"unbounded" —> ∞`,
			data:    `"unbounded"`,
			want:    appdef.Occurs_Unbounded,
			wantErr: false,
		},
		{
			name:    `"3" —> error`,
			data:    `"3"`,
			want:    0,
			wantErr: true,
		},
		{
			name:    `65536 —> error`,
			data:    `65536`,
			want:    0,
			wantErr: true,
		},
		{
			name:    `-1 —> error`,
			data:    `-1`,
			want:    0,
			wantErr: true,
		},
		{
			name:    `"abc" —> error`,
			data:    `"abc"`,
			want:    0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var o appdef.Occurs
			err := o.UnmarshalJSON([]byte(tt.data))
			if (err != nil) != tt.wantErr {
				t.Errorf("Occurs.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if o != tt.want {
					t.Errorf("o.UnmarshalJSON() result = %v, want %v", o, tt.want)
				}
			}
		})
	}
}
