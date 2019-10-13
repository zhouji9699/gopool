package gopool

import (
	"github.com/gopool/internal"
	"sync"
	"sync/atomic"
	"time"
)

type Pool struct {
	capacity int32

	running int32

	workers *workerStack

	release int32

	lock sync.Locker

	cond *sync.Cond

	once sync.Once

	workerCache sync.Pool

	blockingNum int

	options *Options
}

func (p *Pool) periodicallyPurge() {
	heartbeat := time.NewTicker(p.options.ExpiryDuration)
	defer heartbeat.Stop()

	for range heartbeat.C {
		if atomic.LoadInt32(&p.release) == CLOSED {
			break
		}

		p.lock.Lock()
		expiredWorkers := p.workers.findOutExpiry(p.options.ExpiryDuration)
		p.lock.Unlock()

		for i := range expiredWorkers {
			expiredWorkers[i].task <- nil
		}

		if p.Running() == 0 {
			p.cond.Broadcast()
		}
	}
}

func NewPool(size int, options ...Option) (*Pool, error) {
	if size <= 0 {
		return nil, ErrInvalidPoolSize
	}

	opts := new(Options)
	for _, option := range options {
		option(opts)
	}

	if expiry := opts.ExpiryDuration; expiry < 0 {
		return nil, ErrInvalidPoolExpiry
	} else if expiry == 0 {
		opts.ExpiryDuration = DefaultCleanIntervalTime
	}

	p := &Pool{
		capacity: int32(size),
		lock:     internal.NewSLock(),
		options:  opts,
	}
	p.workerCache = sync.Pool{
		New: func() interface{} {
			return &goWorker{
				pool: p,
				task: make(chan func(), workerChanCap),
			}
		},
	}


	p.workers = NewWorkerStack(0)
	p.cond = sync.NewCond(p.lock)

	go p.periodicallyPurge()

	return p, nil
}

func (p *Pool) Submit(task func()) error {
	if atomic.LoadInt32(&p.release) == CLOSED {
		return ErrPoolClosed
	}
	var w *goWorker
	if w = p.retrieveWorker(); w == nil {
		return ErrPoolOverload
	}
	w.task <- task
	return nil
}

func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) Free() int {
	return p.Cap() - p.Running()
}

func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

func (p *Pool) Tune(size int) {
	if size < 0 || p.Cap() == size || p.options.PreAlloc {
		return
	}
	atomic.StoreInt32(&p.capacity, int32(size))
}

func (p *Pool) Release() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.release, 1)
		p.lock.Lock()
		p.workers.release()
		p.lock.Unlock()
	})
}


func (p *Pool) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

func (p *Pool) decRunning() {
	atomic.AddInt32(&p.running, -1)
}

func (p *Pool) retrieveWorker() *goWorker {
	var w *goWorker
	spawnWorker := func() {
		w = p.workerCache.Get().(*goWorker)
		w.run()
	}

	p.lock.Lock()

	w = p.workers.detach()
	if w != nil {
		p.lock.Unlock()
	} else if p.Running() < p.Cap() {
		p.lock.Unlock()
		spawnWorker()
	} else {
		if p.options.Nonblocking {
			p.lock.Unlock()
			return nil
		}
	Reentry:
		if p.options.MaxBlockingTasks != 0 && p.blockingNum >= p.options.MaxBlockingTasks {
			p.lock.Unlock()
			return nil
		}
		p.blockingNum++
		p.cond.Wait()
		p.blockingNum--
		if p.Running() == 0 {
			p.lock.Unlock()
			spawnWorker()
			return w
		}

		w = p.workers.detach()
		if w == nil {
			goto Reentry
		}

		p.lock.Unlock()
	}
	return w
}

func (p *Pool) revertWorker(worker *goWorker) bool {
	if atomic.LoadInt32(&p.release) == CLOSED || p.Running() > p.Cap() {
		return false
	}
	worker.recycleTime = time.Now()
	p.lock.Lock()

	err := p.workers.insert(worker)
	if err != nil {
		return false
	}

	p.cond.Signal()
	p.lock.Unlock()
	return true
}

