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
	WriteRawAtChannel(channel ChannelID, data []byte) error
	WriteAtChannel(channel ChannelID, p packet) error
	WriteAndReleaseChannel(channel ChannelID, p packet) error
	WriteExternalWithSignatureAtChannel(channel ChannelID, signature uint8, data []byte) error
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
	w.channeledMu.Lock()
	channel := w.channels.Reserve()
	w.channeledMu.Unlock()
	return channel
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
	err := w.WriteRawAtChannel(channel, p.GetBytes())
	p.Free()
	return err
}

func (w *writer) WriteRawAtChannel(channel ChannelID, data []byte) error {
	w.channeledMu.Lock()
	err := w.unsafeWriteAtChannel(channel, data)
	w.channeledMu.Unlock()
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

func (w *writer) WriteExternalWithSignatureAtChannel(channel ChannelID, signature uint8, data []byte) error {
	w.channeledMu.Lock()

	size := offset(len(data))
	var err error
	if size <= MaxUint8 {
		err = w.unsafeWriteAtChannel(channel, []byte{
			signature | packetSizeUint8Bits,
			byte(size),
		})
	} else if size <= MaxUint16 {
		err = w.unsafeWriteAtChannel(channel, []byte{
			signature | packetSizeUint16Bits,
			byte(size),
			byte(size >> 8),
		})
	} else if size <= MaxUint24 {
		err = w.unsafeWriteAtChannel(channel, []byte{
			signature | packetSizeUint24Bits,
			byte(size),
			byte(size >> 8),
			byte(size >> 16),
		})
	} else {
		err = w.unsafeWriteAtChannel(channel, []byte{
			signature | packetSizeUint32Bits,
			byte(size),
			byte(size >> 8),
			byte(size >> 16),
			byte(size >> 24),
		})
	}

	if err != nil {
		w.channeledMu.Unlock()
		return err
	}

	err = w.WriteRaw(data)
	w.channeledMu.Unlock()
	return err
}
