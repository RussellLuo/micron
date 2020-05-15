package cron

import (
	"errors"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gorhill/cronexpr"
)

var (
	ErrJobExists = errors.New("job already exists")
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
	// LockTTL is the time duration after which the successfully obtained lock
	// will be released. It is a time window used to protect a job from
	// being executed more than once per execution time of its schedule,
	// which may be caused by the clock error among different machines.
	// Defaults to 1s.
	LockTTL time.Duration

	// The handler for errors.
	ErrHandler func(error)
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

type job struct {
	name string
	expr string
	task func()

	locker Locker
	opts   *Options

	scheduler *cronexpr.Expression

	timer   unsafe.Pointer // type: *time.Timer
	stopped int32
}

func newJob(name, expr string, task func(), locker Locker, opts *Options) *job {
	return &job{
		name:      name,
		expr:      expr,
		task:      task,
		scheduler: cronexpr.MustParse(expr),
		locker:    locker,
		opts:      opts,
	}
}

func (j *job) Schedule(prev time.Time) {
	next := j.scheduler.Next(prev)
	d := next.Sub(prev)

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
	// Try to stop the timer.
	t := atomic.LoadPointer(&j.timer)
	(*time.Timer)(t).Stop()

	// Set the stopped flag to stop the further rescheduling.
	atomic.StoreInt32(&j.stopped, 1)
}

type Cron struct {
	jobs map[string]*job

	locker Locker
	opts   *Options
}

// New creates an instance of Cron.
func New(locker Locker, opts *Options) *Cron {
	return &Cron{
		jobs:   make(map[string]*job),
		locker: locker,
		opts:   opts,
	}
}

func (c *Cron) Add(name, expr string, task func()) error {
	if _, ok := c.jobs[name]; ok {
		return ErrJobExists
	}

	c.jobs[name] = newJob(
		name,
		expr,
		task,
		c.locker,
		c.opts,
	)

	return nil
}

// Start is shorthand for StartFrom(time.Now()).
func (c *Cron) Start() {
	c.StartFrom(time.Now())
}

// Start starts to schedule all jobs from the time now.
func (c *Cron) StartFrom(now time.Time) {
	for _, job := range c.jobs {
		job.Schedule(now)
	}
}

func (c *Cron) Stop() {
	for _, job := range c.jobs {
		// For simplicity now, we do not wait for the inner goroutine to exit.
		job.Stop()
	}
}
