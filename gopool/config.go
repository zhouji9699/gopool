package gopool


import (
	"errors"
	"runtime"
	"time"
)

const (
	DefaultCleanIntervalTime = time.Second
	CLOSED = 1
)

var (
	ErrInvalidPoolSize = errors.New("invalid size for pool")
	ErrInvalidPoolExpiry = errors.New("invalid expiry for pool")
	ErrPoolClosed = errors.New("this pool has been closed")
	ErrPoolOverload = errors.New("too many goroutines blocked on submit or Nonblocking is set")
	workerChanCap = func() int {
		if runtime.GOMAXPROCS(0) == 1 {
			return 0
		}

		return 1
	}()

	defaultPool, _ = NewPool(runtime.NumCPU())
)

type Option func(opts *Options)

type Options struct {
	ExpiryDuration time.Duration

	PreAlloc bool

	MaxBlockingTasks int

	Nonblocking bool

	PanicHandler func(interface{})
}

func Submit(task func()) error {
	return defaultPool.Submit(task)
}

func Running() int {
	return defaultPool.Running()
}

func Cap() int {
	return defaultPool.Cap()
}

func Free() int {
	return defaultPool.Free()
}

func Release() {
	defaultPool.Release()
}