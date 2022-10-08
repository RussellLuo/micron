package micron

import (
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
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

// SemaphoreLocker implements a lock for in-process mutual exclusion.
//
// It is intended to be used in scenarios where multiple instances of Cron are run in the same process.
type SemaphoreLocker struct {
	locks sync.Map
}

func NewSemaphoreLocker() *SemaphoreLocker {
	return &SemaphoreLocker{}
}

func (l *SemaphoreLocker) Lock(job string, ttl time.Duration) (bool, error) {
	lock, ok := l.locks.Load(job)
	if !ok {
		// Not found.
		//
		// Re-try to load the lock first since it may have been set by others concurrently.
		lock, _ = l.locks.LoadOrStore(job, semaphore.NewWeighted(1))
	}

	sem := lock.(*semaphore.Weighted)
	success := sem.TryAcquire(1)
	if success {
		// Release the obtained lock after ttl elapses.
		time.AfterFunc(ttl, func() {
			sem.Release(1)
		})
	}

	return success, nil
}
