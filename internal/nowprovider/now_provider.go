// Package nowprovider provides a simple nowProvider to provide testability to different moduels.
package nowprovider

import "time"

type NowProvider interface {
	Now() time.Time
}

type SystemTimeProvider struct{}

func (s SystemTimeProvider) Now() time.Time {
	return time.Now()
}
