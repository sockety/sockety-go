package done_signal

import "sync/atomic"

type DoneSignal struct {
	ch   chan struct{}
	done atomic.Bool
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
	return s.done.Load()
}

func (s *DoneSignal) Close() bool {
	if s.done.Swap(true) == false {
		close(s.ch)
		return true
	}
	return false
}
