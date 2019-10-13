package gopool

import (
	"log"
	"runtime"
	"time"
)


type goWorker struct {
	pool *Pool

	task chan func()

	recycleTime time.Time
}

func (w *goWorker) run() {
	w.pool.incRunning()
	go func() {
		defer func() {
			w.pool.decRunning()
			if p := recover(); p != nil {
				if ph := w.pool.options.PanicHandler; ph != nil {
					ph(p)
				} else {
					log.Printf("worker exits from a panic: %v\n", p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					log.Printf("worker exits from panic: %s\n", string(buf[:n]))
				}
			}
			w.pool.workerCache.Put(w)
		}()

		for f := range w.task {
			if f == nil {
				return
			}
			f()
			if ok := w.pool.revertWorker(w); !ok {
				return
			}
		}
	}()
}