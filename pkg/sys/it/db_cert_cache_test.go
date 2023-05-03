/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * Aleksei Ponomarev
 */

package heeus_it

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/voedger/voedger/pkg/istructs"
	"github.com/voedger/voedger/pkg/vit"
	dbcertcache "github.com/voedger/voedger/pkg/vvm/db_cert_cache"
	"golang.org/x/crypto/acme/autocert"
)

func TestBasicUsage_db_cache(t *testing.T) {
	var domain string = "test.domain.com"
	ctx := context.TODO()
	require := require.New(t)

	hit := vit.NewHIT(t, &vit.SharedConfig_Simple)
	defer hit.TearDown()

	storage, err := hit.IAppStorageProvider.AppStorage(istructs.AppQName_sys_router)
	require.NoError(err)
	require.NotNil(storage)
	cache := dbcertcache.ProvideDbCache(storage)
	require.NotNil(cache)

	t.Run("Write certificate to router storage, using domain name as key", func(t *testing.T) {
		err = cache.Put(ctx, domain, certificateExample)
		require.NoError(err)
	})

	t.Run("Get certificate from router storage, using domain name as key", func(t *testing.T) {
		var crt []byte
		crt, err = cache.Get(ctx, domain)
		require.NoError(err)
		require.Equal(certificateExample, crt)
	})

	t.Run("Delete certificate from router storage, using domain name as key", func(t *testing.T) {
		err = cache.Delete(ctx, domain)
		require.NoError(err)
	})

	t.Run("Get certificate from router storage, "+
		"using domain name as key. Value must be nil. Error must be ErrCacheMiss.", func(t *testing.T) {
		var crt []byte
		crt, err = cache.Get(ctx, domain)
		require.Nil(crt)
		require.ErrorIs(err, autocert.ErrCacheMiss)
	})
}
