package cron

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

type everyScheduler struct {
	d time.Duration
}

func every(d time.Duration) *everyScheduler {
	return &everyScheduler{d: d}
}

func (s *everyScheduler) Next(prev time.Time) time.Time {
	return prev.Add(s.d)
}

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
		// Release the obtained lock after ttl elapses.
		time.AfterFunc(ttl, func() {
			sem.Release(1)
		})
	}

	return success, nil
}

func TestJob_Schedule(t *testing.T) {
	job := newJob(
		"job",
		nil,                // task will be set later
		every(time.Second), // executed every second
		&locker{},
		&Options{
			// LockTTL must be less than the execution interval of the job.
			LockTTL: 100 * time.Millisecond,
		},
	)

	// So we expect that the job will be executed at least 3 times
	// by waiting for 4s.
	start := time.Now().Truncate(time.Second)
	timePoints := []time.Time{
		start.Add(time.Second),
		start.Add(2 * time.Second),
		start.Add(3 * time.Second),
	}
	waitAndStop := func() {
		time.Sleep(4 * time.Second)
		job.Stop()
	}

	exitC := make(chan time.Time, len(timePoints)+1) // one more size for error-tolerant

	// Set the task and schedule the job.
	job.task = func() {
		exitC <- time.Now()
	}
	job.Schedule(start)

	waitAndStop()

	// Check the final execution time.
	for _, tp := range timePoints {
		got := <-exitC
		want := tp

		err := 100 * time.Millisecond
		max := want.Add(err)

		if got.Before(want) || got.After(max) {
			t.Fatalf("Job executed at: want [%s, %s], got %s", want, max, got)
		}
	}
}

func TestJob_Stop(t *testing.T) {
	job := newJob(
		"job",
		nil,                  // task will be set later
		every(2*time.Second), // executed every two seconds
		&locker{},
		nil,
	)

	count := int32(0)
	job.task = func() {
		atomic.AddInt32(&count, 1)
	}
	job.Schedule(time.Now())

	// Stop the job before the time it's scheduled to execute.
	time.Sleep(100 * time.Millisecond)
	job.Stop()
	wantCount := int32(0)

	// Wait for another 1.9s to ensure that 2s elapses.
	time.Sleep(1900 * time.Millisecond)

	gotCount := atomic.LoadInt32(&count)
	if gotCount != wantCount {
		t.Fatalf("Count: got (%d), want (%d)", gotCount, wantCount)
	}
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
			wantErr: ErrAlreadyExists,
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

func TestCron_AddJob(t *testing.T) {
	cron := New(&locker{}, nil)

	cases := []struct {
		name     string
		inJob    Job
		wantJobs map[string]*job
		wantErr  error
	}{
		{
			name: "add new job",
			inJob: Job{
				Name: "job1",
				Expr: "0 * * * * * *", // at second 0
				Task: func() {},
			},
			wantErr: nil,
		},
		{
			name: "add duplicate job",
			inJob: Job{
				Name: "job1",
				Expr: "1 * * * * * *", // at second 1
				Task: func() {},
			},
			wantErr: ErrAlreadyExists,
		},
		{
			name: "add more job",
			inJob: Job{
				Name: "job2",
				Expr: "2 * * * * * *", // at second 2
				Task: func() {},
			},
			wantErr: nil,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := cron.AddJob(c.inJob)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("Err: got (%v), want (%v)", err, c.wantErr)
			}
		})
	}
}

func TestCron_MultipleInstances(t *testing.T) {
	count := int32(0)
	locker := &locker{}

	startCron := func() *Cron {
		cron := New(locker, nil)
		// Executed at every 2nd second.
		cron.Add("job", "*/2 * * * * * *", func() { // nolint:errcheck
			atomic.AddInt32(&count, 1)
		})
		cron.Start()
		return cron
	}

	// Start 3 Cron instances concurrently.
	cron1 := startCron()
	cron2 := startCron()
	cron3 := startCron()

	// Expect that the job will be executed at least 3 times by waiting for 6s.
	time.Sleep(6 * time.Second)
	cron1.Stop()
	cron2.Stop()
	cron3.Stop()
	wantCount := int32(3)

	gotCount := atomic.LoadInt32(&count)
	if gotCount < wantCount {
		t.Fatalf("Count: got (%v), want (>=%v)", gotCount, wantCount)
	}
}
