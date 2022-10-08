package main

import (
	"context"
	"time"

	"github.com/bsm/redislock"
	"github.com/go-redis/redis/v8"
)

// RedisLocker implements a distributed lock based on Redis (with a single instance).
type RedisLocker struct {
	lockClient *redislock.Client
}

func NewRedisLocker(client redis.UniversalClient) *RedisLocker {
	return &RedisLocker{
		lockClient: redislock.New(client),
	}
}

func (l *RedisLocker) Lock(job string, ttl time.Duration) (bool, error) {
	ctx := context.Background()
	if _, err := l.lockClient.Obtain(ctx, job, ttl, nil); err != nil {
		if err == redislock.ErrNotObtained {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
