/*
 * Copyright (c) 2024-present unTill Software Development Group B.V.
 * @author Denis Gribanov
 */

package sys_it

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	it "github.com/voedger/voedger/pkg/vit"
	"github.com/voedger/voedger/pkg/vvm"
)

func TestSidecarApps_BasicUsage(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	cfg := it.NewOwnVITConfig(
		it.WithVVMConfig(func(cfg *vvm.VVMConfig) {
			cfg.DataPath = filepath.Join(wd, "testdata")
		}),
	)
	vit := it.NewVIT(t, &cfg)
	defer vit.TearDown()
}
