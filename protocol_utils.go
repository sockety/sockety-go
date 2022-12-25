package sockety

import (
	"fmt"
	"golang.org/x/exp/constraints"
)

// Protocol header

func createProtocolHeader(channels uint16) []byte {
	if channels == 0 || channels > MaxChannels {
		panic("invalid number of maximum channels")
	} else if channels == 1 {
		return []byte{
			controlByteConstantBits | controlByteChannelsSingleBits,
		}
	} else if channels <= 256 {
		return []byte{
			controlByteConstantBits | controlByteChannelsUint8Bits,
			byte(channels),
		}
	} else if channels < MaxChannels {
		return []byte{
			controlByteConstantBits | controlByteChannelsUint16Bits,
			byte(channels & 0x00ff),
			byte(channels >> 8),
		}
	} else {
		return []byte{
			controlByteConstantBits | controlByteChannelsMaxBits,
		}
	}
}

func createSwitchChannelPacket(channel ChannelID) []byte {
	if channel <= MaxUint4 {
		return []byte{packetChannelLowBits | uint8(channel)}
	} else {
		return []byte{
			byte(packetChannelHighBits | (channel >> 8)),
			byte(channel & 0x00ff),
		}
	}
}

// Validation

func validateChannelsCount(channelsCount uint16) error {
	if channelsCount < 1 || channelsCount > MaxChannels {
		return fmt.Errorf("protocol_utils.validateChannelsCount: 1-%d allowed, %d passed", MaxChannels, channelsCount)
	}
	return nil
}

func validateReadBufferSize(size offset) error {
	if size < minReadBufferSize {
		return fmt.Errorf("protocol_utils.validateReadBufferSize: minimum %d bytes expected", minReadBufferSize)
	}
	return nil
}

// Calculating bytes

func getMinSizeBytes[T constraints.Integer](size T) uint8 {
	if size < 0 {
		panic("size should not be below 0")
	} else if size == 0 {
		return 0
	} else if uint64(size) <= MaxUint8 {
		return 1
	} else if uint64(size) <= MaxUint16 {
		return 2
	} else if uint64(size) <= MaxUint24 {
		return 3
	} else if uint64(size) <= MaxUint32 {
		return 4
	} else if uint64(size) <= MaxUint48 {
		return 6
	}
	panic("maximum supported size is uint48")
}

func getNameSizeBytes[T constraints.Integer](size T) uint8 {
	switch getMinSizeBytes(size) {
	case 0:
		panic("name should not be empty")
	case 1:
		return 1
	case 2:
		return 2
	default:
		panic("too big name - maximum is uint24")
	}
}

func getDataSizeBytes[T constraints.Integer](size T) uint8 {
	switch getMinSizeBytes(size) {
	case 0:
		return 0
	case 1:
		return 1
	case 2:
		return 2
	case 3, 4, 6:
		return 6
	default:
		panic("too big data - maximum is uint48")
	}
}

func getFilesCountBytes[T constraints.Integer](size T) uint8 {
	switch getMinSizeBytes(size) {
	case 0:
		return 0
	case 1:
		return 1
	case 2:
		return 2
	case 3:
		return 3
	default:
		panic("too big files count - maximum is uint24")
	}
}

func getFilesSizeBytes[T constraints.Integer, U constraints.Integer](size T, filesCount U) uint8 {
	if filesCount == 0 {
		return 0
	}
	switch getMinSizeBytes(size) {
	case 0, 1, 2:
		return 2
	case 3:
		return 3
	case 4:
		return 4
	case 6:
		return 6
	default:
		panic("too big files count - maximum is uint24")
	}
}

// Handling flags

func getNameSizeFlag(bytes uint8) uint8 {
	switch bytes {
	case 1:
		return nameSizeUint8Bits
	default:
		return nameSizeUint16Bits
	}
}

func getDataSizeFlag(bytes uint8) uint8 {
	switch bytes {
	case 0:
		return dataSizeNoneBits
	case 1:
		return dataSizeUint8Bits
	case 2:
		return dataSizeUint16Bits
	default:
		return dataSizeUint48Bits
	}
}

func getFilesCountFlag(bytes uint8) uint8 {
	switch bytes {
	case 0:
		return filesCountNoneBits
	case 1:
		return filesCountUint8Bits
	case 2:
		return filesCountUint16Bits
	default:
		return filesCountUint24Bits
	}
}

func getFilesSizeFlag(bytes uint8) uint8 {
	switch bytes {
	case 0, 2:
		return fileSizeUint16Bits
	case 3:
		return fileSizeUint24Bits
	case 4:
		return fileSizeUint32Bits
	default:
		return fileSizeUint48Bits
	}
}
