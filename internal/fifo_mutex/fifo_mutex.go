package fifo_mutex

type FIFOMutex struct {
	sem chan struct{}
}

func New(concurrent uint16) *FIFOMutex {
	return &FIFOMutex{
		sem: make(chan struct{}, concurrent),
	}
}

func (f *FIFOMutex) Lock() {
	f.sem <- struct{}{}
}

func (f *FIFOMutex) Unlock() {
	<-f.sem
}
