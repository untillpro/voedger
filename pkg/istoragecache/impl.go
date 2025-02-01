/*
 * Copyright (c) 2022-present unTill Pro, Ltd.
 */

package istoragecache

import (
	"context"
	"time"

	"github.com/VictoriaMetrics/fastcache"

	"github.com/voedger/voedger/pkg/appdef"
	"github.com/voedger/voedger/pkg/coreutils"
	"github.com/voedger/voedger/pkg/coreutils/utils"
	"github.com/voedger/voedger/pkg/istorage"
	imetrics "github.com/voedger/voedger/pkg/metrics"
)

type cachedAppStorage struct {
	cache    *fastcache.Cache
	storage  istorage.IAppStorage
	vvm      string
	appQName appdef.AppQName
	iTime    coreutils.ITime

	/* metrics */
	mGetSeconds                 *imetrics.MetricValue
	mGetTotal                   *imetrics.MetricValue
	mGetCachedTotal             *imetrics.MetricValue
	mGetBatchSeconds            *imetrics.MetricValue
	mGetBatchTotal              *imetrics.MetricValue
	mGetBatchCachedTotal        *imetrics.MetricValue
	mPutTotal                   *imetrics.MetricValue
	mPutSeconds                 *imetrics.MetricValue
	mPutBatchTotal              *imetrics.MetricValue
	mPutBatchSeconds            *imetrics.MetricValue
	mPutBatchItemsTotal         *imetrics.MetricValue
	mReadTotal                  *imetrics.MetricValue
	mReadSeconds                *imetrics.MetricValue
	mIncreaseIfNotExistsSeconds *imetrics.MetricValue
	mCompareAndSwapSeconds      *imetrics.MetricValue
	mCompareAndDeleteSeconds    *imetrics.MetricValue
	mTTLGetSeconds              *imetrics.MetricValue
	mTTLReadSeconds             *imetrics.MetricValue
}

type implCachingAppStorageProvider struct {
	storageProvider istorage.IAppStorageProvider
	maxBytes        int
	metrics         imetrics.IMetrics
	vvmName         string
	iTime           coreutils.ITime
}

func (asp *implCachingAppStorageProvider) Prepare(work any) error {
	return asp.storageProvider.Prepare(work)
}

func (asp *implCachingAppStorageProvider) Run(ctx context.Context) {
	asp.storageProvider.Run(ctx)
}

func (asp *implCachingAppStorageProvider) Stop() {
	asp.storageProvider.Stop()
}

// normally must be called once per app
// Currently is called once in appStructsProviderType.Builtin() on VVMAppsBuilder.BuildAppsArtefacts() stage
func (asp *implCachingAppStorageProvider) AppStorage(appQName appdef.AppQName) (istorage.IAppStorage, error) {
	nonCachingAppStorage, err := asp.storageProvider.AppStorage(appQName)
	if err != nil {
		return nil, err
	}

	return newCachingAppStorage(
		asp.maxBytes,
		nonCachingAppStorage,
		asp.metrics,
		asp.vvmName,
		appQName,
		asp.iTime,
	), nil
}

func newCachingAppStorage(
	maxBytes int,
	nonCachingAppStorage istorage.IAppStorage,
	metrics imetrics.IMetrics,
	vvm string,
	appQName appdef.AppQName,
	iTime coreutils.ITime,
) istorage.IAppStorage {
	return &cachedAppStorage{
		cache:                       fastcache.New(maxBytes),
		storage:                     nonCachingAppStorage,
		mGetTotal:                   metrics.AppMetricAddr(getTotal, vvm, appQName),
		mGetCachedTotal:             metrics.AppMetricAddr(getCachedTotal, vvm, appQName),
		mGetSeconds:                 metrics.AppMetricAddr(getSeconds, vvm, appQName),
		mGetBatchSeconds:            metrics.AppMetricAddr(getBatchSeconds, vvm, appQName),
		mGetBatchTotal:              metrics.AppMetricAddr(getBatchTotal, vvm, appQName),
		mGetBatchCachedTotal:        metrics.AppMetricAddr(getBatchCachedTotal, vvm, appQName),
		mPutTotal:                   metrics.AppMetricAddr(putTotal, vvm, appQName),
		mPutSeconds:                 metrics.AppMetricAddr(putSeconds, vvm, appQName),
		mPutBatchTotal:              metrics.AppMetricAddr(putBatchTotal, vvm, appQName),
		mPutBatchSeconds:            metrics.AppMetricAddr(putBatchSeconds, vvm, appQName),
		mPutBatchItemsTotal:         metrics.AppMetricAddr(putBatchItemsTotal, vvm, appQName),
		mReadTotal:                  metrics.AppMetricAddr(readTotal, vvm, appQName),
		mReadSeconds:                metrics.AppMetricAddr(readSeconds, vvm, appQName),
		mIncreaseIfNotExistsSeconds: metrics.AppMetricAddr(insertIfNotExistsSeconds, vvm, appQName),
		mCompareAndSwapSeconds:      metrics.AppMetricAddr(compareAndSwapSeconds, vvm, appQName),
		mCompareAndDeleteSeconds:    metrics.AppMetricAddr(compareAndDeleteSeconds, vvm, appQName),
		mTTLGetSeconds:              metrics.AppMetricAddr(ttlGetSeconds, vvm, appQName),
		mTTLReadSeconds:             metrics.AppMetricAddr(ttlReadSeconds, vvm, appQName),
		vvm:                         vvm,
		appQName:                    appQName,
		iTime:                       iTime,
	}
}

//nolint:revive
func (s *cachedAppStorage) InsertIfNotExists(pKey []byte, cCols []byte, value []byte, ttlSeconds int) (ok bool, err error) {
	start := time.Now()
	defer func() {
		s.mIncreaseIfNotExistsSeconds.Increase(time.Since(start).Seconds())
	}()

	ok, err = s.storage.InsertIfNotExists(pKey, cCols, value, ttlSeconds)
	if err != nil {
		return false, err
	}

	if ok {
		expireAt := int64(0)
		if ttlSeconds > 0 {
			expireAt = s.iTime.Now().Add(time.Duration(ttlSeconds) * time.Second).UnixMilli()
		}

		d := coreutils.DataWithExpiration{Data: value, ExpireAt: expireAt}
		s.cache.Set(makeKey(pKey, cCols), d.ToBytes())
	}

	return ok, nil
}

//nolint:revive
func (s *cachedAppStorage) CompareAndSwap(pKey []byte, cCols []byte, oldValue, newValue []byte, ttlSeconds int) (ok bool, err error) {
	start := time.Now()
	defer func() {
		s.mCompareAndSwapSeconds.Increase(time.Since(start).Seconds())
	}()

	ok, err = s.storage.CompareAndSwap(pKey, cCols, oldValue, newValue, ttlSeconds)
	if err != nil {
		return false, err
	}

	if ok {
		expireAt := int64(0)
		if ttlSeconds > 0 {
			expireAt = s.iTime.Now().Add(time.Duration(ttlSeconds) * time.Second).UnixMilli()
		}

		d := coreutils.DataWithExpiration{Data: newValue, ExpireAt: expireAt}
		s.cache.Set(makeKey(pKey, cCols), d.ToBytes())
	}

	return ok, nil
}

//nolint:revive
func (s *cachedAppStorage) CompareAndDelete(pKey []byte, cCols []byte, expectedValue []byte) (ok bool, err error) {
	start := time.Now()
	defer func() {
		s.mCompareAndDeleteSeconds.Increase(time.Since(start).Seconds())
	}()

	ok, err = s.storage.CompareAndDelete(pKey, cCols, expectedValue)
	if err != nil {
		return false, err
	}

	if ok {
		s.cache.Del(makeKey(pKey, cCols))
	}

	return ok, nil
}

//nolint:revive
func (s *cachedAppStorage) TTLGet(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	start := time.Now()
	defer func() {
		s.mTTLGetSeconds.Increase(time.Since(start).Seconds())
	}()

	var found bool
	var key = makeKey(pKey, cCols)

	*data = (*data)[0:0]
	cachedData := make([]byte, 0)
	cachedData, found = s.cache.HasGet(*data, key)

	if found {
		d := coreutils.ReadWithExpiration(cachedData)

		if d.IsExpired(s.iTime.Now()) {
			s.cache.Del(key)

			return false, nil
		}
		*data = d.Data

		return len(*data) != 0, nil
	}

	return s.storage.TTLGet(pKey, cCols, data)
}

//nolint:revive
func (s *cachedAppStorage) TTLRead(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	start := time.Now()
	defer func() {
		s.mTTLReadSeconds.Increase(time.Since(start).Seconds())
	}()

	return s.storage.TTLRead(ctx, pKey, startCCols, finishCCols, cb)
}

func (s *cachedAppStorage) Put(pKey []byte, cCols []byte, value []byte) (err error) {
	start := time.Now()
	defer func() {
		s.mPutSeconds.Increase(time.Since(start).Seconds())
	}()
	s.mPutTotal.Increase(1.0)
	err = s.storage.Put(pKey, cCols, value)

	if err == nil {
		data := coreutils.DataWithExpiration{Data: value}
		s.cache.Set(makeKey(pKey, cCols), data.ToBytes())
	}

	return err
}

func (s *cachedAppStorage) PutBatch(items []istorage.BatchItem) (err error) {
	start := time.Now()
	defer func() {
		s.mPutBatchSeconds.Increase(time.Since(start).Seconds())
	}()
	s.mPutBatchTotal.Increase(1.0)
	s.mPutBatchItemsTotal.Increase(float64(len(items)))

	err = s.storage.PutBatch(items)
	if err == nil {
		for _, i := range items {
			data := coreutils.DataWithExpiration{Data: i.Value}
			s.cache.Set(makeKey(i.PKey, i.CCols), data.ToBytes())
		}
	}

	return err
}

func (s *cachedAppStorage) Get(pKey []byte, cCols []byte, data *[]byte) (ok bool, err error) {
	start := time.Now()
	defer func() {
		s.mGetSeconds.Increase(time.Since(start).Seconds())
	}()
	s.mGetTotal.Increase(1.0)

	key := makeKey(pKey, cCols)
	*data = (*data)[0:0]
	cachedData := make([]byte, 0)
	cachedData, ok = s.cache.HasGet(cachedData, key)

	if ok {
		s.mGetCachedTotal.Increase(1.0)

		if len(cachedData) != 0 {
			*data = cachedData[utils.Uint64Size:]
			return len(*data) != 0, nil
		}
	}

	ok, err = s.storage.Get(pKey, cCols, data)
	if err != nil {
		return false, err
	}

	d := coreutils.DataWithExpiration{}
	if ok {
		d.Data = *data
	}
	s.cache.Set(key, d.ToBytes())

	return ok, nil
}

func (s *cachedAppStorage) GetBatch(pKey []byte, items []istorage.GetBatchItem) (err error) {
	start := time.Now()
	defer func() {
		s.mGetBatchSeconds.Increase(time.Since(start).Seconds())
	}()
	s.mGetBatchTotal.Increase(1.0)
	if !s.getBatchFromCache(pKey, items) {
		return s.getBatchFromStorage(pKey, items)
	}
	return
}

func (s *cachedAppStorage) getBatchFromCache(pKey []byte, items []istorage.GetBatchItem) (ok bool) {
	for i := range items {
		cachedData, ok := s.cache.HasGet((*items[i].Data)[0:0], makeKey(pKey, items[i].CCols))
		if !ok {
			return false
		}

		*items[i].Data = cachedData[utils.Uint64Size:]
		items[i].Ok = len(*items[i].Data) != 0
	}
	s.mGetBatchCachedTotal.Increase(1.0)
	return true
}

func (s *cachedAppStorage) getBatchFromStorage(pKey []byte, items []istorage.GetBatchItem) (err error) {
	err = s.storage.GetBatch(pKey, items)
	if err != nil {
		return err
	}

	for _, item := range items {
		d := coreutils.DataWithExpiration{}
		if item.Ok {
			d.Data = *item.Data
		}
		s.cache.Set(makeKey(pKey, item.CCols), d.ToBytes())
	}

	return err
}

func (s *cachedAppStorage) Read(ctx context.Context, pKey []byte, startCCols, finishCCols []byte, cb istorage.ReadCallback) (err error) {
	start := time.Now()
	defer func() {
		s.mReadSeconds.Increase(time.Since(start).Seconds())
	}()
	s.mReadTotal.Increase(1.0)

	return s.storage.Read(ctx, pKey, startCCols, finishCCols, cb)
}

func (s *cachedAppStorage) SetTestDelayGet(delay time.Duration) {
	s.storage.(istorage.IStorageDelaySetter).SetTestDelayGet(delay)
}

func (s *cachedAppStorage) SetTestDelayPut(delay time.Duration) {
	s.storage.(istorage.IStorageDelaySetter).SetTestDelayPut(delay)
}

func makeKey(pKey []byte, cCols []byte) (res []byte) {
	res = make([]byte, 0, stackKeySize)
	// res = make([]byte, 0, len(pKey)+len(cCols)) // escapes to heap
	res = append(res, pKey...)
	res = append(res, cCols...)
	return res
}
