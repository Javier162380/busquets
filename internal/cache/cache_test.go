package cache_test

import (
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/cache"
	"github.com/stretchr/testify/require"
)

type fixedClock struct{ now time.Time }

func (f *fixedClock) Now() time.Time { return f.now }

func (f *fixedClock) advance(d time.Duration) { f.now = f.now.Add(d) }

func TestGet(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		setup       func(*cache.MuxCache[string], *fixedClock)
		key         string
		advanceBy   time.Duration
		wantValue   string
		wantErr     error
	}{
		{
			name:    "returns error when key does not exist",
			setup:   func(_ *cache.MuxCache[string], _ *fixedClock) {},
			key:     "missing",
			wantErr: cache.ErrorKeyNotFoundError,
		},
		{
			name: "returns value when key exists and is not expired",
			setup: func(c *cache.MuxCache[string], _ *fixedClock) {
				c.Set("hello", "world", time.Minute)
			},
			key:       "hello",
			wantValue: "world",
		},
		{
			name: "returns error when key is expired",
			setup: func(c *cache.MuxCache[string], _ *fixedClock) {
				c.Set("hello", "world", time.Minute)
			},
			key:       "hello",
			advanceBy: 2 * time.Minute,
			wantErr:   cache.ErrorKeyExpiredError,
		},
		{
			name: "removes expired key on get",
			setup: func(c *cache.MuxCache[string], _ *fixedClock) {
				c.Set("hello", "world", time.Minute)
			},
			key:       "hello",
			advanceBy: 2 * time.Minute,
			wantErr:   cache.ErrorKeyExpiredError,
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
				c.Set("key", "value", time.Minute)
			},
			key:       "key",
			wantValue: "value",
		},
		{
			name: "overwrites existing key with new value",
			setup: func(c *cache.MuxCache[string]) {
				c.Set("key", "first", time.Minute)
				c.Set("key", "second", time.Minute)
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
	tests := []struct {
		name          string
		setup         func(*cache.MuxCache[string])
		key           string
		wantErr       error
		wantValue     string
	}{
		{
			name: "removes expired keys after gc interval",
			setup: func(c *cache.MuxCache[string]) {
				c.Set("expired", "value", 10*time.Millisecond)
			},
			key:     "expired",
			wantErr: cache.ErrorKeyNotFoundError,
		},
		{
			name: "keeps valid keys after gc interval",
			setup: func(c *cache.MuxCache[string]) {
				c.Set("valid", "value", time.Hour)
			},
			key:       "valid",
			wantValue: "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cache.NewWithProvider[string](50*time.Millisecond, 20*time.Millisecond, &fixedClock{now: time.Now()})
			defer c.Stop()
			tt.setup(c)

			time.Sleep(60 * time.Millisecond)

			got, err := c.Get(tt.key)
			require.ErrorIs(t, err, tt.wantErr)
			require.Equal(t, tt.wantValue, got)
		})
	}
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