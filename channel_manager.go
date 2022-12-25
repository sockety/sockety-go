package sockety

import (
	"sync/atomic"
)

type ChannelID uint16

type channelManager struct {
	// Configuration
	channelsCount uint16

	// State
	current  ChannelID
	locks    []uint32
	released chan ChannelID
}

type ChannelManager interface {
	Current() ChannelID           // not thread-safe
	Is(id ChannelID) bool         // not thread-safe
	SetCurrent(channel ChannelID) // not thread-safe
	GetFree() ChannelID           // not thread-safe
	Reserve() ChannelID
	Release(channel ChannelID)
}

func NewChannelManager(channelsCount uint16) ChannelManager {
	if err := validateChannelsCount(channelsCount); err != nil {
		panic(err)
	}
	return &channelManager{
		channelsCount: channelsCount,
		locks:         make([]uint32, channelsCount),
	}
}

func (c *channelManager) obtainLock(channel uint16) bool {
	return atomic.CompareAndSwapUint32(&c.locks[channel], 0, 1)
}

func (c *channelManager) isTemporarilyFree(channel uint16) bool {
	return c.locks[channel] == 0
}

func (c *channelManager) Current() ChannelID {
	return c.current
}

func (c *channelManager) Is(channel ChannelID) bool {
	return c.current == channel
}

func (c *channelManager) SetCurrent(channel ChannelID) {
	if c.current != channel {
		c.current = channel
	}
}

func (c *channelManager) Reserve() ChannelID {
	// Try to obtain the channel that is free at the moment
	for i := uint16(0); i < c.channelsCount; i++ {
		channel := (uint16(c.current) + i) % c.channelsCount
		if c.obtainLock(channel) {
			return ChannelID(channel)
		}
	}

	// Otherwise, wait until some channel will be released and try to reserve it
	for {
		channel := uint16(<-c.released)
		if c.obtainLock(channel) {
			return ChannelID(channel)
		}
	}
}

// TODO: Consider if this fast path is actually helpful
func (c *channelManager) GetFree() ChannelID {
	// Try to obtain the channel that is free at the moment
	for i := uint16(0); i < c.channelsCount; i++ {
		channel := (uint16(c.current) + i) % c.channelsCount
		if c.isTemporarilyFree(channel) {
			return ChannelID(channel)
		}
	}

	// Otherwise, wait until some channel will be released and try to reserve it
	for {
		channel := uint16(<-c.released)
		if c.isTemporarilyFree(channel) {
			return ChannelID(channel)
		}
	}
}

func (c *channelManager) Release(channel ChannelID) {
	atomic.StoreUint32(&c.locks[channel], 0)
	select {
	case c.released <- channel:
	default:
	}
}
