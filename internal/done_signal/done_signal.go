package done_signal

import "sync/atomic"

type DoneSignal struct {
	ch   chan struct{}
	done uint32
}

func New() *DoneSignal {
	return &DoneSignal{
		ch: make(chan struct{}),
	}
}

func (s *DoneSignal) Get() <-chan struct{} {
	return s.ch
}

func (s *DoneSignal) Load() bool {
	return atomic.LoadUint32(&s.done) == 1
}

func (s *DoneSignal) LoadUnsafe() bool {
	return s.done == 1
}

func (s *DoneSignal) Close() bool {
	if atomic.SwapUint32(&s.done, 1) == 0 {
		close(s.ch)
		return true
	}
	return false
}
