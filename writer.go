package sockety

import (
	"io"
	"sync"
)

type WriterOptions struct {
	Channels uint16
}

type writer struct {
	target   io.Writer
	channels ChannelManager

	channeledMu sync.Mutex
}

type Writer interface {
	ObtainChannel() ChannelID
	ReleaseChannel(channel ChannelID)

	WriteRaw(data []byte) error

	WriteStandalone(p packet) error
	Write(p packet) error
	WriteAndObtainChannel(p packet) (ChannelID, error)
	WriteAtChannel(channel ChannelID, p packet) error
	WriteAndReleaseChannel(channel ChannelID, p packet) error
}

func NewWriter(target io.Writer, options WriterOptions) Writer {
	// Read & validate options
	channels := getDefault(options.Channels, MaxChannels)
	if err := validateChannelsCount(channels); err != nil {
		panic(err)
	}

	return &writer{
		target:   target,
		channels: NewChannelManager(channels),
	}
}

func (w *writer) ObtainChannel() ChannelID {
	return w.channels.Reserve()
}

func (w *writer) ReleaseChannel(channel ChannelID) {
	w.channels.Release(channel)
}

func (w *writer) WriteRaw(data []byte) error {
	_, err := w.target.Write(data)
	return err
}

func (w *writer) unsafeWriteAtChannel(channel ChannelID, data []byte) error {
	if w.channels.Is(channel) {
		return w.WriteRaw(data)
	}

	w.channels.SetCurrent(channel)
	return w.WriteRaw(append(createSwitchChannelPacket(channel), data...))
}

func (w *writer) WriteStandalone(p packet) error {
	err := w.WriteRaw(p.GetBytes())
	p.Free()
	return err
}

// TODO: Think if it's possible to unlock immediately when Write is buffered
func (w *writer) Write(p packet) error {
	w.channeledMu.Lock()
	err := w.unsafeWriteAtChannel(w.channels.GetFree(), p.GetBytes())
	w.channeledMu.Unlock()
	p.Free()
	return err
}

// TODO: Think if it's possible to unlock immediately when Write is buffered
func (w *writer) WriteAndObtainChannel(p packet) (ChannelID, error) {
	channel := w.ObtainChannel()
	w.channeledMu.Lock()
	err := w.unsafeWriteAtChannel(channel, p.GetBytes())
	w.channeledMu.Unlock()
	if err != nil {
		w.ReleaseChannel(channel)
		p.Free()
		return ChannelID(0), err
	}
	p.Free()
	return channel, nil
}

// TODO: Think if it's possible to unlock immediately when Write is buffered
func (w *writer) WriteAtChannel(channel ChannelID, p packet) error {
	w.channeledMu.Lock()
	err := w.unsafeWriteAtChannel(channel, p.GetBytes())
	w.channeledMu.Unlock()
	p.Free()
	return err
}

// TODO: Think if it's possible to unlock immediately when Write is buffered
func (w *writer) WriteAndReleaseChannel(channel ChannelID, p packet) error {
	w.channeledMu.Lock()
	err := w.unsafeWriteAtChannel(channel, p.GetBytes())
	w.channeledMu.Unlock()
	w.ReleaseChannel(channel)
	p.Free()
	return err
}
