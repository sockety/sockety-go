package sockety

import (
	"github.com/google/uuid"
	"github.com/sockety/sockety-go/internal/buffer_pool"
	"github.com/valyala/bytebufferpool"
)

type packet struct {
	b *bytebufferpool.ByteBuffer
	o offset
}

func newPacket(signature uint8, size offset) packet {
	if size <= MaxUint8 {
		buf := buffer_pool.ObtainUnsafe(uint64(size) + 2)
		buf.B[0] = signature | packetSizeUint8Bits
		buf.B[1] = uint8(size)
		return packet{b: buf, o: 2}
	} else if size <= MaxUint16 {
		buf := buffer_pool.ObtainUnsafe(uint64(size) + 3)
		buf.B[0] = signature | packetSizeUint16Bits
		writeBytes.Uint16(buf.B, 1, uint16(size))
		return packet{b: buf, o: 3}
	} else if size <= MaxUint24 {
		buf := buffer_pool.ObtainUnsafe(uint64(size) + 4)
		buf.B[0] = signature | packetSizeUint24Bits
		writeBytes.Uint24(buf.B, 1, Uint24(size))
		return packet{b: buf, o: 4}
	} else {
		buf := buffer_pool.ObtainUnsafe(uint64(size) + 5)
		buf.B[0] = signature | packetSizeUint32Bits
		writeBytes.Uint32(buf.B, 1, uint32(size))
		return packet{b: buf, o: 5}
	}
}

//func newPacketWithIndex(signature uint8, size offset) packet {
//	// TODO
//}

// Writing to packet: not thread-safe

func (p packet) Uint8(value uint8) packet {
	p.o = writeBytes.Uint8(p.b.B, p.o, value)
	return p
}

func (p packet) Byte(value byte) packet {
	p.o = writeBytes.Byte(p.b.B, p.o, value)
	return p
}

func (p packet) Uint16(value uint16) packet {
	p.o = writeBytes.Uint16(p.b.B, p.o, value)
	return p
}

func (p packet) Uint24(value Uint24) packet {
	p.o = writeBytes.Uint24(p.b.B, p.o, value)
	return p
}

func (p packet) Uint32(value uint32) packet {
	p.o = writeBytes.Uint32(p.b.B, p.o, value)
	return p
}

func (p packet) Uint48(value Uint48) packet {
	p.o = writeBytes.Uint48(p.b.B, p.o, value)
	return p
}

func (p packet) Uint(value Uint48, bytesLength uint8) packet {
	p.o = writeBytes.Uint(p.b.B, p.o, value, bytesLength)
	return p
}

func (p packet) String(value string) packet {
	p.o = writeBytes.String(p.b.B, p.o, value)
	return p
}

func (p packet) Bytes(value []byte) packet {
	p.o = writeBytes.Bytes(p.b.B, p.o, value)
	return p
}

func (p packet) UUID(value uuid.UUID) packet {
	p.o = writeBytes.UUID(p.b.B, p.o, value)
	return p
}

// Utilities

func (p packet) Free() {
	buffer_pool.Release(p.b)
}

func (p packet) Valid() bool {
	return offset(len(p.b.B)) == p.o
}

func (p packet) GetBytes() []byte {
	return p.b.B
}
