package nil

import (
	"time"
)

// Locker implements a fake lock that is always obtainable.
//
// It is intended to be used in scenarios where only one instance of Cron is needed.
type Locker struct{}

func New() *Locker {
	return &Locker{}
}

func (l *Locker) Lock(job string, ttl time.Duration) (bool, error) {
	// The lock is always obtainable.
	return true, nil
}
