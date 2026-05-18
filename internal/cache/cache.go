package cache

import (
	"errors"
	"sync"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/nowProvider"
)

var (
	ErrorKeyNotFoundError = errors.New("key not found")
	ErrorKeyExpiredError  = errors.New("key is expired")
)

type cacheItem[T any] struct {
	Value     T
	CreatedAt time.Time
	ExpiresAt time.Time
}

type MuxCache[T any] struct {
	ttl         time.Duration
	gcInternal  time.Duration
	stop        chan struct{}
	mux         sync.Mutex
	muxCache    map[string]cacheItem[T]
	nowProvider nowProvider.NowProvider
}

func NewWithProvider[T any](ttl, gcInterval time.Duration, np nowProvider.NowProvider) *MuxCache[T] {
	mc := &MuxCache[T]{
		ttl:         ttl,
		gcInternal:  gcInterval,
		stop:        make(chan struct{}),
		muxCache:    make(map[string]cacheItem[T]),
		nowProvider: np,
	}
	if ttl > 0 {
		go mc.gcCleaner()
	}
	return mc
}

func New[T any](ttl, gcInterval time.Duration) *MuxCache[T] {
	mux := &MuxCache[T]{
		ttl:         ttl,
		gcInternal:  gcInterval,
		stop:        make(chan struct{}),
		mux:         sync.Mutex{},
		muxCache:    make(map[string]cacheItem[T]),
		nowProvider: nowProvider.SystemTimeProvider{},
	}

	if ttl > 0 {
		go mux.gcCleaner()
	}
	return mux
}

func (mc *MuxCache[T]) gcCleaner() {
	ticker := time.NewTicker(mc.gcInternal)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.mux.Lock()
			for k, v := range mc.muxCache {
				if v.ExpiresAt.Before(mc.nowProvider.Now()) {
					delete(mc.muxCache, k)
				}
			}
			mc.mux.Unlock()
		case <-mc.stop:
			return
		}
	}
}

func (mc *MuxCache[T]) Get(key string) (T, error) {
	mc.mux.Lock()
	defer mc.mux.Unlock()

	var zero T
	item, ok := mc.muxCache[key]
	if !ok {
		return zero, ErrorKeyNotFoundError
	}
	if item.ExpiresAt.Before(mc.nowProvider.Now()) {
		delete(mc.muxCache, key)
		return zero, ErrorKeyExpiredError
	}
	return item.Value, nil
}

func (mc *MuxCache[T]) Set(key string, value T, ttl time.Duration) {
	mc.mux.Lock()
	defer mc.mux.Unlock()

	now := mc.nowProvider.Now()
	mc.muxCache[key] = cacheItem[T]{Value: value, CreatedAt: now, ExpiresAt: now.Add(ttl)}
}

func (mc *MuxCache[T]) Delete(key string) {
	mc.mux.Lock()
	defer mc.mux.Unlock()
	delete(mc.muxCache, key)
}

func (mc *MuxCache[T]) Stop() {
	close(mc.stop)
}
