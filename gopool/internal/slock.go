package internal


import (
	"runtime"
	"sync"
	"sync/atomic"
)

type sLock uint32

func (sl *sLock) Lock() {
	for !atomic.CompareAndSwapUint32((*uint32)(sl), 0, 1) {
		runtime.Gosched()
	}
}

func (sl *sLock) Unlock() {
	atomic.StoreUint32((*uint32)(sl), 0)
}

func NewSLock() sync.Locker {
	return new(sLock)
}