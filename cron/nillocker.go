package cron

import (
	"time"
)

// NilLocker implements a fake lock that is always obtainable.
//
// It is intended to be used in scenarios where only one instance of Cron is needed.
type NilLocker struct{}

func NewNilLocker() *NilLocker {
	return &NilLocker{}
}

func (l *NilLocker) Lock(job string, ttl time.Duration) (bool, error) {
	// The lock is always obtainable.
	return true, nil
}
