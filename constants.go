package sockety

// Maximum values for different unsigned integer sizes

const (
	MaxUint4  = 15
	MaxUint8  = 255
	MaxUint12 = 4095
	MaxUint16 = 65_535
	MaxUint24 = 16_777_215
	MaxUint32 = 4_294_967_295
	MaxUint48 = 281_474_976_710_655
)

// Maximum values for different data

const (
	maxChannelId = MaxUint12
	MaxChannels  = maxChannelId + 1
)

const (
	minReadBufferSize = 16 // for
)

// Defaults

const (
	DefaultReadBufferSize = 4_096
)

// Control byte

const (
	controlByteConstantBitsBitmask = 0b11111100
	controlByteConstantBits        = 0b11100000
)

const (
	controlByteChannelsBitmask    = 0b00000011
	controlByteChannelsSingleBits = 0b00000000
	controlByteChannelsUint8Bits  = 0b00000001
	controlByteChannelsUint16Bits = 0b00000010
	controlByteChannelsMaxBits    = 0b00000011
)

// Packet codes

const (
	packetBitmask           = 0b11110000
	packetHeartbeatBits     = 0b10100000
	packetGoAwayBits        = 0b10110000
	packetAbortBits         = 0b10010000
	packetChannelLowBits    = 0b00000000
	packetChannelHighBits   = 0b00010000
	packetFastReplyLowBits  = 0b00110000
	packetFastReplyHighBits = 0b01000000
	packetDataBits          = 0b11100000
	packetFileBits          = 0b11000000
	packetFileEndBits       = 0b11010000
	packetStreamBits        = 0b0111000
	packetStreamEndBits     = 0b10000000
	packetMessageBits       = 0b00100000
	packetResponseBits      = 0b01010000
	packetContinueBits      = 0b01100000
)

// Packet size bits

const (
	packetSizeBitmask    = 0b00001100
	packetSizeUint8Bits  = 0b00000000
	packetSizeUint16Bits = 0b00000100
	packetSizeUint24Bits = 0b00001000
	packetSizeUint32Bits = 0b00001100
)

// Specific bits

const (
	hasStreamBits       = 0b00000010
	expectsResponseBits = 0b00000001
)

const (
	indexSizeBitmask    = 0b00000011
	indexSizeFirstBits  = 0b00000000
	indexSizeUint8Bits  = 0b00000001
	indexSizeUint16Bits = 0b00000010
	indexSizeUint24Bits = 0b00000011
)

const (
	dataSizeBitmask    = 0b11000000
	dataSizeNoneBits   = 0b00000000
	dataSizeUint8Bits  = 0b01000000
	dataSizeUint16Bits = 0b10000000
	dataSizeUint48Bits = 0b11000000
)

const (
	filesCountBitmask    = 0b00110000
	filesCountNoneBits   = 0b00000000
	filesCountUint8Bits  = 0b00010000
	filesCountUint16Bits = 0b00100000
	filesCountUint24Bits = 0b00110000
)

const (
	fileSizeBitmask    = 0b00001100
	fileSizeUint16Bits = 0b00000000
	fileSizeUint24Bits = 0b00000100
	fileSizeUint32Bits = 0b00001000
	fileSizeUint48Bits = 0b00001100
)

const (
	nameSizeBitmask    = 0b00000010
	nameSizeUint8Bits  = 0b00000000
	nameSizeUint16Bits = 0b00000010
)
