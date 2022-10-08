package cron

import (
	"strings"
	"time"

	"github.com/gorhill/cronexpr"
)

func Parse(expr string) (Scheduler, error) {
	everyPrefix := "@every"
	if strings.HasPrefix(expr, everyPrefix) {
		// Expected format: "@every <duration>"
		s := strings.TrimSpace(strings.TrimPrefix(expr, everyPrefix))
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, err
		}
		return Every(d), nil
	}

	return cronexpr.Parse(expr)
}

func MustParse(expr string) Scheduler {
	s, err := Parse(expr)
	if err != nil {
		panic(err)
	}
	return s
}

// everyScheduler is a Scheduler that will activate once every duration.
type everyScheduler struct {
	duration time.Duration
}

// Every returns a Scheduler that will activate once every duration.
// Duration d will be truncated down to a multiple of one second. If the
// truncated result is zero, the final duration will be set to one second.
func Every(d time.Duration) Scheduler {
	d = d.Truncate(time.Second)
	if d == 0 {
		d = time.Second
	}
	return &everyScheduler{duration: d}
}

// Next returns the next time the scheduler will activate. The result time
// will be truncated down to a multiple of one second.
func (s *everyScheduler) Next(prev time.Time) time.Time {
	return prev.Add(s.duration).Truncate(time.Second)
}
