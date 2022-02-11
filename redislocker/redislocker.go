package redislocker

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
)

// Locker implements a distributed lock based on Redis (with a single instance).
type Locker struct {
	lockClient *redislock.Client
}

func New(client redis.UniversalClient) *Locker {
	return &Locker{
		lockClient: redislock.New(client),
	}
}

func (l *Locker) Lock(job string, ttl time.Duration) (bool, error) {
	ctx := context.Background()
	if _, err := l.lockClient.Obtain(ctx, job, ttl, nil); err != nil {
		if err == redislock.ErrNotObtained {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
