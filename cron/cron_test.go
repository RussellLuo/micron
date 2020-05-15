package cron

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

type locker struct {
	locks sync.Map
}

func (l *locker) Lock(job string, ttl time.Duration) (bool, error) {
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
		// The lock is obtained successfully.
		go func() {
			// Release the lock after ttl elapses.
			time.Sleep(ttl)
			sem.Release(1)
		}()
	}

	return success, nil
}

func TestCron_Add(t *testing.T) {
	cron := New(&locker{}, nil)

	cases := []struct {
		name     string
		inName   string
		inExpr   string
		wantJobs map[string]*job
		wantErr  error
	}{
		{
			name:    "add new",
			inName:  "job1",
			inExpr:  "0 * * * * * *", // at second 0
			wantErr: nil,
		},
		{
			name:    "add exists",
			inName:  "job1",
			inExpr:  "1 * * * * * *", // at second 1
			wantErr: ErrJobExists,
		},
		{
			name:    "add more",
			inName:  "job2",
			inExpr:  "2 * * * * * *", // at second 2
			wantErr: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := cron.Add(c.inName, c.inExpr, func() {})
			if err != c.wantErr {
				t.Fatalf("Err: got (%v), want (%v)", err, c.wantErr)
			}
		})
	}
}

func TestCron_StartFrom(t *testing.T) {
	cron := New(&locker{}, nil)

	// Job is scheduled to be executed every second.
	expr := "* * * * * * *"

	// So we expect that the job will be executed at least 3 times
	// by waiting for 4s.
	start := time.Now()
	durations := []time.Duration{
		time.Second, // start + 1s
		time.Second, // start + 2s
		time.Second, // start + 3s
	}
	waitAndStop := func() {
		time.Sleep(4 * time.Second)
		cron.Stop()
	}

	exitC := make(chan time.Time, len(durations)+1) // one more size for error-tolerant

	// Add and start the job.
	cron.Add("job", expr, func() {
		exitC <- time.Now()
	})
	cron.StartFrom(start)

	waitAndStop()

	// Check the final execution time.
	accum := time.Duration(0)
	for _, d := range durations {
		got := (<-exitC).Truncate(time.Millisecond)
		accum += d
		want := start.Add(accum).Truncate(time.Millisecond)

		// The maximal error of cronexpr is about 1s.
		err := 900 * time.Millisecond
		min := want.Add(-err)
		max := want.Add(err)

		if got.Before(min) || got.After(max) {
			t.Fatalf("Job executed at: want [%s, %s], got %s", min, max, got)
		}
	}
}

func TestCron_Stop(t *testing.T) {
	count := int32(0)

	cron := New(&locker{}, nil)
	cron.Add("job", "*/2 * * * * * *", func() { // every two seconds
		atomic.AddInt32(&count, 1)
	})
	cron.Start()

	// Stop the job before the time it's scheduled to execute.
	time.Sleep(500 * time.Millisecond)
	cron.Stop()
	wantCount := int32(0)

	// Wait for another 1.5ms to ensure that 2s elapses.
	time.Sleep(1500 * time.Millisecond)

	gotCount := atomic.LoadInt32(&count)
	if gotCount != wantCount {
		t.Fatalf("Count: got (%d), want (%d)", gotCount, wantCount)
	}
}

func Test_MultipleCrons(t *testing.T) {
	count := int32(0)
	locker := &locker{}

	startCron := func() *Cron {
		cron := New(locker, nil)
		cron.Add("job", "* * * * * * *", func() { // every second
			atomic.AddInt32(&count, 1)
		})
		cron.Start()
		return cron
	}

	// Start 3 Cron instances concurrently.
	cron1 := startCron()
	cron2 := startCron()
	cron3 := startCron()

	// Expect that the job will be executed at least 3 times by waiting for 4s.
	time.Sleep(4 * time.Second)
	cron1.Stop()
	cron2.Stop()
	cron3.Stop()
	wantCount := int32(3)

	gotCount := atomic.LoadInt32(&count)
	if gotCount < wantCount {
		t.Fatalf("Count: got (%v), want (>=%v)", gotCount, wantCount)
	}
}
