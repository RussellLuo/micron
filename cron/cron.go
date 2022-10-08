package cron

import (
	"errors"
	"fmt"
	"sync/atomic"
	"time"
	"unsafe"
)

var (
	ErrAlreadyExists = errors.New("already exists")
)

// Locker is a distributed lock.
type Locker interface {
	// Lock obtains the lock to execute the job named job. If the lock is
	// successfully obtained, Lock will return true, otherwise it will return false.
	//
	// The implementation of Locker must release the obtained lock automatically
	// after ttl elapses.
	Lock(job string, ttl time.Duration) (bool, error)
}

type Options struct {
	// The location name, which must be "Local", "UTC" or a location name corresponding
	// to a file in the IANA Time Zone database, such as "Asia/Shanghai".
	//
	// Defaults to "UTC".
	Timezone string

	// LockTTL is the time duration after which the successfully obtained lock
	// will be released. It is a time window used to protect a job from
	// being executed more than once per execution time of its schedule,
	// which may be caused by the clock error among different machines.
	//
	// Defaults to 1s.
	LockTTL time.Duration

	// The handler for errors.
	ErrHandler func(error)
}

func (o *Options) timezone() string {
	if o == nil {
		return "UTC"
	}
	return o.Timezone
}

func (o *Options) lockTTL() time.Duration {
	if o == nil {
		return time.Second
	}
	return o.LockTTL
}

func (o *Options) errHandler() func(error) {
	if o == nil {
		return func(error) {}
	}
	return o.ErrHandler
}

type Scheduler interface {
	Next(time.Time) time.Time
}

type job struct {
	name      string
	task      func()
	scheduler Scheduler

	locker Locker
	opts   *Options

	timer   unsafe.Pointer // type: *time.Timer
	stopped int32
}

func newJob(name string, task func(), scheduler Scheduler, locker Locker, opts *Options) *job {
	return &job{
		name:      name,
		task:      task,
		scheduler: scheduler,
		locker:    locker,
		opts:      opts,
	}
}

func (j *job) Schedule(prev time.Time) {
	next := j.scheduler.Next(prev)
	d := time.Until(next)

	t := time.AfterFunc(d, func() {
		if atomic.LoadInt32(&j.stopped) == 1 {
			// If stopped, just return.
			return
		}

		// Reschedule the job.
		j.Schedule(next)

		// Try to obtain the lock.
		ok, err := j.locker.Lock(j.name, j.opts.lockTTL())
		if err != nil {
			j.opts.errHandler()(err)
		}

		if ok {
			// The lock is obtained successfully, execute the job.
			j.task()
		}
	})

	atomic.StorePointer(&j.timer, unsafe.Pointer(t))
}

func (j *job) Stop() {
	t := (*time.Timer)(atomic.LoadPointer(&j.timer))
	// Try to stop the timer.
	if !t.Stop() {
		// The job has already been started, set the stopped flag
		// to stop the further rescheduling.
		atomic.StoreInt32(&j.stopped, 1)
	}
}

// Job represents a normal job, which will be scheduled by Cron.
type Job struct {
	// The unique name of the job.
	Name string

	// The cron expression. Two formats are supported currently:
	//
	// - `@every <duration>`, where "duration" is a string accepted by `time.ParseDuration` (http://golang.org/pkg/time/#ParseDuration).
	// - Standard Cron Expression (https://github.com/gorhill/cronexpr#implementation).
	//
	// Note that the execution interval of the job must be greater than LockTTL.
	Expr string

	// The handler of the job.
	Task func()
}

// Cron is a fault-tolerant job scheduler.
type Cron struct {
	jobs map[string]*job

	locker Locker
	opts   *Options

	location *time.Location
}

// New creates an instance of Cron.
func New(locker Locker, opts *Options) *Cron {
	location, err := time.LoadLocation(opts.timezone())
	if err != nil {
		panic(err)
	}

	return &Cron{
		jobs:     make(map[string]*job),
		locker:   locker,
		opts:     opts,
		location: location,
	}
}

// Add adds a job with the given properties. If name already exists, Add will
// return ErrAlreadyExists, otherwise it will return nil.
//
// Note that the execution interval of the job, which is specified by expr,
// must be greater than LockTTL.
func (c *Cron) Add(name, expr string, task func()) error {
	if _, ok := c.jobs[name]; ok {
		return ErrAlreadyExists
	}

	c.jobs[name] = newJob(
		name,
		task,
		MustParse(expr),
		c.locker,
		c.opts,
	)

	return nil
}

// AddJob adds one or more jobs into Cron c. If the name of any job already
// exists, AddJob will return ErrAlreadyExists, otherwise it will return nil.
func (c *Cron) AddJob(job ...Job) error {
	// Ensure the uniqueness of all job names first.
	for _, j := range job {
		if _, ok := c.jobs[j.Name]; ok {
			return fmt.Errorf("add job %s: %w", j.Name, ErrAlreadyExists)
		}
	}

	for _, j := range job {
		c.jobs[j.Name] = newJob(
			j.Name,
			j.Task,
			MustParse(j.Expr),
			c.locker,
			c.opts,
		)
	}

	return nil
}

// Start starts to schedule all jobs.
func (c *Cron) Start() {
	now := time.Now().In(c.location)
	for _, job := range c.jobs {
		job.Schedule(now)
	}
}

// Stop stops all the jobs. For simplicity now, it does not wait for the inner
// goroutines (which have been started before) to exit.
func (c *Cron) Stop() {
	for _, job := range c.jobs {
		job.Stop()
	}
}
