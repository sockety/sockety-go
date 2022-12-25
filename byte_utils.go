package sockety

import (
	"encoding/binary"
	"github.com/google/uuid"
	"unsafe"
)

// Read bytes

var readBytes readBytesInternal

type readBytesInternal struct{}

func (readBytesInternal) Uint8(b []byte, o offset) uint8 {
	return b[o]
}

func (readBytesInternal) Uint16(b []byte, o offset) uint16 {
	return binary.LittleEndian.Uint16(b[o:])
}

func (readBytesInternal) Uint24(b []byte, o offset) Uint24 {
	_ = b[o+2] // bounds check hint to compiler; see golang.org/issue/14808
	return Uint24(b[o]) | (Uint24(b[o+1]) << 8) | (Uint24(b[o+2]) << 16)
}

func (readBytesInternal) Uint32(b []byte, o offset) uint32 {
	return binary.LittleEndian.Uint32(b[o:])
}

func (readBytesInternal) Uint48(b []byte, o offset) Uint48 {
	_ = b[o+5] // bounds check hint to compiler; see golang.org/issue/14808
	return Uint48(b[o]) | (Uint48(b[o+1]) << 8) | (Uint48(b[o+2]) << 16) | (Uint48(b[o+3]) << 24) | (Uint48(b[o+4]) << 32) | (Uint48(b[o+5]) << 40)
}

func (readBytesInternal) UUID(b []byte, o offset) uuid.UUID {
	result, err := uuid.FromBytes(b[o : o+16])
	if err != nil {
		panic("should not happen")
	}
	return result
}

func (readBytesInternal) String(b []byte, o offset, bytesLength uint32) string {
	b = b[o:bytesLength]
	return *(*string)(unsafe.Pointer(&b))
}

// Write bytes

var writeBytes writeBytesInternal

type writeBytesInternal struct{}

func (writeBytesInternal) Uint8(b []byte, o offset, value uint8) offset {
	b[o] = value
	return o + 1
}

func (writeBytesInternal) Byte(b []byte, o offset, value byte) offset {
	b[o] = value
	return o + 1
}

func (writeBytesInternal) Bytes(b []byte, o offset, value []byte) offset {
	nextOffset := o + offset(len(value))
	_ = b[nextOffset-1]
	copy(b[o:], value)
	return nextOffset
}

func (writeBytesInternal) Uint16(b []byte, o offset, value uint16) offset {
	binary.LittleEndian.PutUint16(b[o:], value)
	return o + 2
}

func (writeBytesInternal) Uint24(b []byte, o offset, value Uint24) offset {
	_ = b[o+2] // bounds check hint to compiler; see golang.org/issue/14808
	b[o] = byte(value)
	b[o+1] = byte(value >> 8)
	b[o+2] = byte(value >> 16)
	return o + 3
}

func (writeBytesInternal) Uint32(b []byte, o offset, value uint32) offset {
	binary.LittleEndian.PutUint32(b[o:], value)
	return o + 4
}

func (writeBytesInternal) Uint48(b []byte, o offset, value Uint48) offset {
	_ = b[o+5] // bounds check hint to compiler; see golang.org/issue/14808
	b[o] = byte(value)
	b[o+1] = byte(value >> 8)
	b[o+2] = byte(value >> 16)
	b[o+3] = byte(value >> 24)
	b[o+4] = byte(value >> 32)
	b[o+5] = byte(value >> 40)
	return o + 6
}

func (w writeBytesInternal) Uint(b []byte, o offset, value Uint48, bytesLength uint8) offset {
	switch bytesLength {
	case 1:
		return w.Uint8(b, o, uint8(value))
	case 2:
		return w.Uint16(b, o, uint16(value))
	case 3:
		return w.Uint24(b, o, Uint24(value))
	case 4:
		return w.Uint32(b, o, uint32(value))
	case 6:
		return w.Uint48(b, o, value)
	default:
		panic("supported uint bytes are 1-4 and 6")
	}
}

func (writeBytesInternal) UUID(b []byte, o offset, value uuid.UUID) offset {
	_ = b[o+15] // bounds check hint to compiler; see golang.org/issue/14808
	*(*uuid.UUID)(unsafe.Pointer(&b[o])) = value
	return o + 16
}

func (writeBytesInternal) String(b []byte, o offset, value string) offset {
	strLen := len(value)
	strOffset := o + offset(strLen)
	_ = b[strOffset-1]
	copy(b[o:], *(*[]byte)(unsafe.Pointer(
		&struct {
			string
			Cap int
		}{value, strLen},
	)))
	return strOffset
}
