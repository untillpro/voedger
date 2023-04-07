/*
 * Copyright (c) 2021-present Sigma-Soft, Ltd.
 * @author: Dmitry Molchanovsky
 */

package iratesce

import (
	"time"

	irates "github.com/untillpro/voedger/pkg/irates"
)

// является ли состояние bucket'а неопределенным (нулевым)
// is the bucket's state undefined (zero)
func BucketStateIsZero(state *irates.BucketState) bool {
	return state.TakenTokens == 0 && state.Period == 0 && state.MaxTokensPerPeriod == 0
}

// создание нового bucket'а с переданными параметрами
// creating a new bucket with the passed parameters
func newBucket(state irates.BucketState, now time.Time) (p *bucketType) {
	b := bucketType{
		state: state,
	}
	b.reset(now)
	return &b
}

// применить к backet'у параметры state
// apply state parameters to the backet
func (bucket *bucketType) resetToState(state irates.BucketState, now time.Time) {
	bucket.state = state
	bucket.reset(now)
}

// пересчитывает количество токенов bucket.state.TakenTokens на время TimeFunc
// recalculates the number of bucket.state tokens.TakenTokens for the TimeFunc time
func (bucket *bucketType) recalcBuketState(now time.Time) {
	_, _, tokens := bucket.limiter.advance(now)
	value := float64(bucket.limiter.burst) - tokens
	if value < 0 {
		value = 0
	}
	bucket.state.TakenTokens = irates.NumTokensType(value)
}

// сбросить bucket в исходное состояние, соответствующее параметрам, с которыми он был создан (наполнить его токенами)
// reset the bucket to its original state corresponding to the parameters with which it was created (fill it with tokens)
func (bucket *bucketType) reset(now time.Time) {
	var interval Limit
	if bucket.state.MaxTokensPerPeriod > 0 {
		interval = every(time.Duration(int64(bucket.state.Period) / int64(bucket.state.MaxTokensPerPeriod)))
	}
	bucket.limiter = *newLimiter(interval, int(bucket.state.MaxTokensPerPeriod))
	bucket.limiter.allowN(now, int(bucket.state.TakenTokens))
}

// Попробуем взять n токенов из заданных ведер
// Операция атомарна - либо доступные токены есть в ведрах для всех ключей, либо нет
// Try to take n tokens from the given buckets
// The operation must be atomic - either all buckets are modified or none
func (b *bucketsType) TakeTokens(buckets []irates.BucketKey, n int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	var keyIdx int
	res := true
	t := b.timeFunc()
	// проверим наличие токеноа по запрашиваемым ключам
	// let's check the presence of a token using the requested keys
	for keyIdx = 0; keyIdx < len(buckets); keyIdx++ {
		bucket := b.bucketByKey(&buckets[keyIdx])
		// если по каким-то причинам бакет для очередного ключа не найден, то его отсутствие не должно влиять на общий результат проверки
		// ключ может содержать имя действия, для которого ограничение не было задано. В таком случае, ограничение данного действия не производится
		// if for some reason the bucket for the next key is not found, then its absence should not affect the overall result of the check
		// the key may contain the name of the action for which the restriction was not set. In this case, the restriction of this action is not performed
		if bucket == nil {
			continue
		}

		// если очередной токен не получен, то уходим из цикла запросов
		// if the next token is not received, then we leave the request cycle
		if !bucket.limiter.allowN(t, n) {
			res = false
			break
		}

	}

	// если не получили токены по всем ключам, то вернем взятые токены обратно в вёдра
	// if we have not received tokens for all keys, then we will return the tokens taken back to the buckets
	if !res {
		for i := 0; i < keyIdx; i++ {
			if bucket := b.bucketByKey(&buckets[i]); bucket != nil {
				bucket.limiter.allowN(t, -n)
			}
		}
	}
	return res
}

// вернет bucket из отображения
// если для переваемого ключа бакета еще нет, то он будет предварительно создан с параметрами "по умолчанию"
// returns bucket from the map
// if there is no bucket for the requested key yet, it will be pre-created with the "default" parameters
func (b *bucketsType) bucketByKey(key *irates.BucketKey) (bucket *bucketType) {
	if bucket, ok := b.buckets[*key]; ok {
		return bucket
	}

	// если для ключа bucket'а еще нет, то создадим его
	// if there is no bucket for the key yet, then we will create it
	bs, ok := b.defaultStates[key.RateLimitName]
	if !ok {
		return nil
	}
	bucket = newBucket(bs, b.timeFunc())
	b.buckets[*key] = bucket
	return bucket
}

// установка параметров ограничения "по умолчанию" для действия с именем RateLimitName
// при этом работающие Bucket'ы параметры ограничений не меняют
// для изменения параметров работающих бакетов используйте функцию ReserRateBuckets
// at the same time, the working Bucket's parameters of restrictions do not change
// to change the parameters of working buckets, use the ReserRateBuckets function
// setting the "default" constraint parameters for an action named RateLimitName
func (b *bucketsType) SetDefaultBucketState(RateLimitName string, bucketState irates.BucketState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.defaultStates[RateLimitName] = bucketState
}

// returns irates.ErrorRateLimitNotFound
func (b *bucketsType) GetDefaultBucketsState(RateLimitName string) (state irates.BucketState, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if state, ok := b.defaultStates[RateLimitName]; ok {
		return state, nil
	}
	return state, irates.ErrorRateLimitNotFound
}

// изменить параметры ограничения с именем RateLimitName для работающих bucket'ов на bucketState
// соответствующие bucket'ы будут "сброшены" до максимально допустимого количества доступных токенов
// change the restriction parameters with the name RateLimitName for running buckets on bucketState
// the corresponding buckets will be "reset" to the maximum allowed number of available tokens
func (b *bucketsType) ResetRateBuckets(RateLimitName string, bucketState irates.BucketState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	_, ok := b.defaultStates[RateLimitName]

	// если параметры "по умолчанию" для данного ограничения не были заданы ранее, то
	// bucket'ов для этого ограничения точно нет. Просто уходим
	// if the "default" parameters for this restriction were not set earlier, then
	// there are definitely no buckets for this restriction. Just leave
	if !ok {
		return
	}

	for bucketKey, bucket := range b.buckets {
		if bucketKey.RateLimitName == RateLimitName {
			bucket.resetToState(bucketState, b.timeFunc())
		}
	}
}

// получение параметров ограничения для bucket'а, соответствующего передаваемому ключу
// getting the restriction parameters for the bucket corresponding to the transmitted key
func (b *bucketsType) GetBucketState(bucketKey irates.BucketKey) (state irates.BucketState, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	buc := b.bucketByKey(&bucketKey)

	if buc != nil {
		buc.recalcBuketState(b.timeFunc())
		return buc.state, nil
	}
	return state, irates.ErrorRateLimitNotFound
}

// установить новые параметры ограничения для bucket'а, соответствующего ключу bucketKey
func (b *bucketsType) SetBucketState(bucketKey irates.BucketKey, state irates.BucketState) (err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	buc := b.bucketByKey(&bucketKey)

	if buc == nil {
		return irates.ErrorRateLimitNotFound
	}

	buc.resetToState(state, b.timeFunc())
	return nil
}
