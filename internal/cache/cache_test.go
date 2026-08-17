package cache_test

import (
	"testing"
	"time"

	"github.com/Javier162380/busquets/internal/cache"

	"github.com/stretchr/testify/require"
)

type fixedClock struct{ now time.Time }

func (f *fixedClock) Now() time.Time { return f.now }

func (f *fixedClock) advance(d time.Duration) { f.now = f.now.Add(d) }

func TestGet(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		setup     func(*cache.MuxCache[string], *fixedClock)
		key       string
		advanceBy time.Duration
		wantValue string
		wantErr   error
	}{
		{
			name:    "returns error when key does not exist",
			setup:   func(_ *cache.MuxCache[string], _ *fixedClock) {},
			key:     "missing",
			wantErr: cache.ErrorKeyNotFound,
		},
		{
			name: "returns value when key exists and is not expired",
			setup: func(c *cache.MuxCache[string], _ *fixedClock) {
				c.Set("hello", "world")
			},
			key:       "hello",
			wantValue: "world",
		},
		{
			name: "returns error when key is expired",
			setup: func(c *cache.MuxCache[string], _ *fixedClock) {
				c.Set("hello", "world")
			},
			key:       "hello",
			advanceBy: 2 * time.Minute,
			wantErr:   cache.ErrorKeyExpired,
		},
		{
			name: "removes expired key on get",
			setup: func(c *cache.MuxCache[string], _ *fixedClock) {
				c.Set("hello", "world")
			},
			key:       "hello",
			advanceBy: 2 * time.Minute,
			wantErr:   cache.ErrorKeyExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &fixedClock{now: base}
			c := cache.NewWithProvider[string](0, time.Hour, clock)
			tt.setup(c, clock)
			clock.advance(tt.advanceBy)

			got, err := c.Get(tt.key)

			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.wantValue, got)
		})
	}
}

func TestSet(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		setup     func(*cache.MuxCache[string])
		key       string
		wantValue string
	}{
		{
			name: "stores new key",
			setup: func(c *cache.MuxCache[string]) {
				c.Set("key", "value")
			},
			key:       "key",
			wantValue: "value",
		},
		{
			name: "overwrites existing key with new value",
			setup: func(c *cache.MuxCache[string]) {
				c.Set("key", "first")
				c.Set("key", "second")
			},
			key:       "key",
			wantValue: "second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clock := &fixedClock{now: base}
			c := cache.NewWithProvider[string](0, time.Hour, clock)
			tt.setup(c)

			got, err := c.Get(tt.key)

			require.NoError(t, err)
			require.Equal(t, tt.wantValue, got)
		})
	}
}

func TestGcCleaner(t *testing.T) {
	t.Run("removes expired keys after gc interval", func(t *testing.T) {
		c := cache.New[string](10*time.Millisecond, 20*time.Millisecond)
		defer c.Stop()
		c.Set("expired", "value")

		time.Sleep(60 * time.Millisecond)

		_, err := c.Get("expired")
		require.ErrorIs(t, err, cache.ErrorKeyNotFound)
	})

	t.Run("keeps valid keys after gc interval", func(t *testing.T) {
		c := cache.New[string](time.Hour, 20*time.Millisecond)
		defer c.Stop()
		c.Set("valid", "value")

		time.Sleep(60 * time.Millisecond)

		got, err := c.Get("valid")
		require.NoError(t, err)
		require.Equal(t, "value", got)
	})
}

func TestStop(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "stop does not panic when gc is running"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cache.NewWithProvider[string](time.Minute, 10*time.Millisecond, &fixedClock{now: time.Now()})
			require.NotPanics(t, func() { c.Stop() })
		})
	}
}
