package redis

import (
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis"
)

type Locker struct {
	lockClient *redislock.Client
}

func New(client redis.UniversalClient) *Locker {
	return &Locker{
		lockClient: redislock.New(client),
	}
}

func (l *Locker) Lock(job string, ttl time.Duration) (bool, error) {
	if _, err := l.lockClient.Obtain(job, ttl, nil); err != nil {
		if err == redislock.ErrNotObtained {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
