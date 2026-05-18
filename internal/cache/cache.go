// Package cache creats a simple cache to store in memory records.
package cache

import (
	"errors"
	"sync"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/nowprovider"
)

var (
	ErrorKeyNotFound = errors.New("key not found")
	ErrorKeyExpired  = errors.New("key is expired")
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
	mux         sync.RWMutex
	muxCache    map[string]cacheItem[T]
	nowProvider nowprovider.NowProvider
}

func NewWithProvider[T any](ttl, gcInterval time.Duration, np nowprovider.NowProvider) *MuxCache[T] {
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
		mux:         sync.RWMutex{},
		muxCache:    make(map[string]cacheItem[T]),
		nowProvider: nowprovider.SystemTimeProvider{},
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
			var toDelete []string
			mc.mux.RLock()
			for k, v := range mc.muxCache {
				if v.ExpiresAt.Before(mc.nowProvider.Now()) {
					toDelete = append(toDelete, k)
				}
			}
			mc.mux.RUnlock()

			if len(toDelete) > 0 {
				mc.mux.Lock()
				for _, k := range toDelete {
					delete(mc.muxCache, k)
				}
				mc.mux.Unlock()
			}
		case <-mc.stop:
			return
		}
	}
}

func (mc *MuxCache[T]) Get(key string) (T, error) {
	mc.mux.RLock()
	item, ok := mc.muxCache[key]
	mc.mux.RUnlock()

	var zero T
	if !ok {
		return zero, ErrorKeyNotFound
	}
	if item.ExpiresAt.Before(mc.nowProvider.Now()) {
		mc.mux.Lock()
		delete(mc.muxCache, key)
		mc.mux.Unlock()
		return zero, ErrorKeyExpired
	}
	return item.Value, nil
}

func (mc *MuxCache[T]) Set(key string, value T) {
	mc.mux.Lock()
	defer mc.mux.Unlock()

	now := mc.nowProvider.Now()
	mc.muxCache[key] = cacheItem[T]{Value: value, CreatedAt: now, ExpiresAt: now.Add(mc.ttl)}
}

func (mc *MuxCache[T]) Delete(key string) {
	mc.mux.Lock()
	defer mc.mux.Unlock()
	delete(mc.muxCache, key)
}

func (mc *MuxCache[T]) Stop() {
	close(mc.stop)
}
