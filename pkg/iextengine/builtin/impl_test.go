/*
  - Copyright (c) 2023-present unTill Software Development Group B.V.
    @author Michael Saigachenko
*/
package iextenginebuiltin

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/iextengine"
	"github.com/voedger/voedger/pkg/istructs"
)

func Test_BasicUsage(t *testing.T) {

	require := require.New(t)
	counter := 0

	ext1name := appdef.NewFullQName("test", "ext1")
	ext2name := appdef.NewFullQName("test", "ext2")
	ext1func := func(ctx context.Context, io iextengine.IExtensionIO) error {
		counter++
		if counter == 3 {
			return errors.New("test")
		}
		return nil
	}
	ext2func := func(ctx context.Context, io iextengine.IExtensionIO) error {
		counter--
		return nil
	}

	factory := ProvideExtensionEngineFactory(iextengine.BuiltInExtFuncs{
		ext1name: ext1func,
		ext2name: ext2func,
	})

	engines, err := factory.New(context.Background(), istructs.AppQName_test1_app1, nil, nil, 5)
	require.NoError(err)
	require.Len(engines, 5)

	require.NoError(engines[0].Invoke(context.Background(), ext1name, nil))
	require.NoError(engines[1].Invoke(context.Background(), ext1name, nil))
	require.ErrorContains(engines[2].Invoke(context.Background(), ext1name, nil), "test")
	require.NoError(engines[3].Invoke(context.Background(), ext2name, nil))
	require.NoError(engines[4].Invoke(context.Background(), ext2name, nil))
	require.ErrorContains(engines[2].Invoke(context.Background(), appdef.NewFullQName("test", "ext3"), nil), "undefined extension: test.ext3")
	require.Equal(1, counter)

}

func Test_Panics(t *testing.T) {

	require := require.New(t)

	ext1name := appdef.NewFullQName("test", "ext1")
	ext1func := func(ctx context.Context, io iextengine.IExtensionIO) error {
		panic("boom")
	}

	factory := ProvideExtensionEngineFactory(iextengine.BuiltInExtFuncs{
		ext1name: ext1func,
	})

	engines, err := factory.New(context.Background(), istructs.AppQName_test1_app1, nil, nil, 5)
	require.NoError(err)
	require.Len(engines, 5)

	require.ErrorContains(engines[0].Invoke(context.Background(), ext1name, nil), "extension panic: boom")

}
