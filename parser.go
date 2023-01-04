package sockety

import (
	"errors"
	"fmt"
	"github.com/sockety/sockety-go/internal/cast"
	"io"
)

type ParserOptions struct {
	Channels   uint16
	BufferSize uint32
}

const (
	subParserModeHeader = iota
	subParserModeData
	subParserModeStream
)

type parser struct {
	channelsCount  uint16
	currentChannel *parserChannel
	channels       map[uint16]*parserChannel
	reader         *bufferedReader

	// Temporary items
	sub           *limitedBufferedReader
	subMode       uint8
	packetsSize8  *streamReaderUint8
	packetsSize16 *streamReaderUint16
	packetsSize24 *streamReaderUint24
	packetsSize32 *streamReaderUint32
}

type ParserResult interface {
	message | Response | FastReply | GoAway
}

type socketyHeader struct {
	Channels uint16
}

type Parser interface {
	Read() (ParserResult, error)
}

func NewParser(source io.Reader, options ParserOptions) Parser {
	// Read & validate options
	channelsCount := getDefault(options.Channels, MaxChannels)
	if err := validateChannelsCount(channelsCount); err != nil {
		panic(err)
	}

	// Prepare buffered reader
	buffer := make([]byte, getDefault(options.BufferSize, DefaultReadBufferSize))
	reader := newBufferedReader(source, buffer).(*bufferedReader)

	// Build zero-channel
	channel0 := newParserChannel()
	channels := map[uint16]*parserChannel{0: channel0}

	// Validate options
	return &parser{
		channelsCount:  channelsCount,
		channels:       channels,
		currentChannel: channel0,
		reader:         reader,

		sub:           limitBufferedReader(reader, 0).(*limitedBufferedReader),
		packetsSize8:  newStreamReaderUint8(),
		packetsSize16: newStreamReaderUint16(),
		packetsSize24: newStreamReaderUint24(),
		packetsSize32: newStreamReaderUint32(),
	}
}

func (p *parser) ReadHeader() (*socketyHeader, error) {
	// Read first header byte
	header, err := p.reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// Validate the first byte
	if header&controlByteConstantBitsBitmask != controlByteConstantBits {
		return nil, errors.New("sockety.parser.ReadHeader: invalid control bits")
	}

	// Decide how to read (constant, uint8 or uint16le)
	size := header & controlByteChannelsBitmask
	switch size {
	case controlByteChannelsSingleBits:
		return &socketyHeader{Channels: 1}, nil
	case controlByteChannelsUint8Bits:
		v, err := newStreamReaderUint8().Get(p.reader)
		if err != nil {
			return nil, err
		}
		return &socketyHeader{Channels: uint16(v)}, nil
	case controlByteChannelsUint16Bits:
		v, err := newStreamReaderUint16().Get(p.reader)
		if err != nil {
			return nil, err
		} else if v > MaxChannels {
			return nil, errors.New("sockety.parser.ReadHeader: invalid number of channels received")
		}
		return &socketyHeader{Channels: v}, nil
	//case controlByteChannelsMaxBits:
	default:
		return &socketyHeader{Channels: MaxChannels}, nil
	}
}

func (p *parser) channel(id uint16) (*parserChannel, error) {
	// Disallow invalid channel ID
	if id < 0 || id >= p.channelsCount {
		return nil, fmt.Errorf("channel ID should be between 0 and %d", p.channelsCount-1)
	}

	// Reuse existing channel if it is available
	if val, ok := p.channels[id]; ok {
		return val, nil
	}

	// Create and save new channel
	channel := newParserChannel()
	p.channels[id] = channel
	return channel, nil
}

func getPacketSize(p *parser, signature uint8, r BufferedReader) (uint32, error) {
	switch signature & packetSizeBitmask {
	case packetSizeUint8Bits:
		return cast.ToUint32(p.packetsSize8.Get(r))
	case packetSizeUint16Bits:
		return cast.ToUint32(p.packetsSize16.Get(r))
	case packetSizeUint24Bits:
		return cast.ToUint32(p.packetsSize24.Get(r))
	case packetSizeUint32Bits:
		return p.packetsSize32.Get(r)
	}
	panic("impossible path")
}

func (p *parser) Read() (ParserResult, error) {
	if !p.reader.MayHave(1) {
		return nil, io.EOF
	}

	if p.sub.size > 0 {
		if p.sub.Len() == 0 {
			err := p.sub.Preload()
			if err != nil {
				return nil, err
			}
		}
		switch p.subMode {
		case subParserModeHeader:
			return p.currentChannel.Process(p.sub)
		case subParserModeData:
			return nil, p.currentChannel.ProcessData(p.sub)
		case subParserModeStream:
			return nil, p.currentChannel.ProcessStream(p.sub)
		default:
			panic("impossible path")
		}
	}

	for {
		signature, err := p.reader.ReadByte()
		if err != nil {
			return nil, err
		}

		switch signature & packetBitmask {
		case packetHeartbeatBits:
			// TODO: Handle timeout
		case packetGoAwayBits:
			// TODO: Handle go away
			return nil, errors.New("go away packet not implemented yet")
		case packetAbortBits:
			// TODO: Abort
			return nil, errors.New("abort packet not implemented yet")
		case packetChannelLowBits:
			p.currentChannel, err = p.channel(uint16(signature & 0b00001111))
			if err != nil {
				return nil, err
			}
		case packetChannelHighBits:
			next, err := p.reader.ReadByte()
			if err != nil {
				return nil, err
			}
			p.currentChannel, err = p.channel((uint16(signature&0b00001111) << 8) | uint16(next))
			if err != nil {
				return nil, err
			}
		case packetMessageBits:
			expectsResponse := signature&expectsResponseBits == expectsResponseBits
			hasStream := signature&hasStreamBits == hasStreamBits
			packetSize, err := getPacketSize(p, signature, p.reader)
			if err != nil {
				return nil, err
			}

			// TODO: Run channel processor all the time (?) - Think how to close it, handle errors, etc
			err = p.currentChannel.InitMessage(expectsResponse, hasStream)
			if err != nil {
				return nil, err
			}

			p.subMode = subParserModeHeader
			p.sub.size = offset(packetSize)
			return p.currentChannel.Process(p.sub)
		case packetResponseBits:
			return nil, errors.New("response packet not implemented yet")
		case packetContinueBits:
			return nil, errors.New("continue packet not implemented yet")
		case packetFastReplyLowBits:
			return nil, errors.New("fast reply low packet not implemented yet")
		case packetFastReplyHighBits:
			return nil, errors.New("fast reply high packet not implemented yet")
		case packetDataBits:
			packetSize, err := getPacketSize(p, signature, p.reader)
			if err != nil {
				return nil, err
			}

			p.subMode = subParserModeData
			p.sub.size = offset(packetSize)
			return nil, p.currentChannel.ProcessData(p.sub)
		case packetStreamBits:
			packetSize, err := getPacketSize(p, signature, p.reader)
			if err != nil {
				return nil, err
			}

			p.subMode = subParserModeStream
			p.sub.size = offset(packetSize)
			return nil, p.currentChannel.ProcessStream(p.sub)
		case packetStreamEndBits:
			return nil, p.currentChannel.EndStream()
		case packetFileBits:
			return nil, errors.New("file packet not implemented yet")
		case packetFileEndBits:
			return nil, errors.New("file end packet not implemented yet")
		default:
			return nil, fmt.Errorf("unknown packet signature: %d", signature)
		}
	}
}
