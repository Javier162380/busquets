package nowProvider

import "time"


type NowProvider interface {
	Now() time.Time
}

type SystemTimeProvider struct{}

func (s SystemTimeProvider) Now() time.Time {
	return time.Now()
}