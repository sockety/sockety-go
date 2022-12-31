package sockety

import (
	"errors"
	"github.com/google/uuid"
)

// TODO: Consider validating Uint24/Uint48 if that will not hurt performance

// Define reader type

type streamReaderSource interface {
	Len() offset
	MayHave(size offset) bool
	ReadByte() (byte, error)
	Read(target []byte) (offset, error)
}

type streamReader[T any] interface {
	Get(source streamReaderSource) (T, error)
}

// Define helper utilities

var ErrNotReady = errors.New("stream reader source has not enough bytes")

// Define readers for every type

// Uint8

type streamReaderUint8 struct{}

func newStreamReaderUint8() *streamReaderUint8 {
	return &streamReaderUint8{}
}

func (s *streamReaderUint8) Get(source streamReaderSource) (uint8, error) {
	if !source.MayHave(1) {
		return 0, ErrNotReady
	}
	v, err := source.ReadByte()
	if err != nil {
		return 0, err
	}
	return v, nil
}

type streamReaderUnsafeBytes struct {
	buffer []byte
	offset uint16
	size   uint16
}

// Bytes - unsafe way, to get bytes for all other primitives
// TODO: Consider consuming directly bytes from *Source when all are available immediately

func newStreamReaderUnsafeBytes(size uint16) *streamReaderUnsafeBytes {
	return &streamReaderUnsafeBytes{
		buffer: make([]byte, size),
		size:   size,
	}
}

// TODO: Consider resizing back to lower size afterwards?
func (s *streamReaderUnsafeBytes) Resize(size uint16) {
	current := uint16(len(s.buffer))
	if size > current {
		s.buffer = append(s.buffer, make([]byte, size-current)...)
	}
	s.size = size
}

func (s *streamReaderUnsafeBytes) Get(source streamReaderSource) ([]byte, error) {
	if !source.MayHave(1) {
		return nil, ErrNotReady
	}

	var n offset
	var err error
	if source.MayHave(offset(s.size - s.offset)) {
		n, err = source.Read(s.buffer[s.offset:s.size])
	} else {
		n, err = source.Read(s.buffer[s.offset : offset(s.offset)+source.Len()])
	}

	s.offset += uint16(n)

	if err != nil {
		return nil, err
	} else if s.offset < s.size {
		return nil, ErrNotReady
	}

	s.offset = 0
	return s.buffer, nil
}

// Uint16

type streamReaderUint16 struct {
	bytes *streamReaderUnsafeBytes
}

func newStreamReaderUint16() *streamReaderUint16 {
	return &streamReaderUint16{
		bytes: newStreamReaderUnsafeBytes(2),
	}
}

func (s *streamReaderUint16) Get(source streamReaderSource) (uint16, error) {
	bytes, err := s.bytes.Get(source)
	if err != nil {
		return 0, err
	}
	return readBytes.Uint16(bytes, 0), nil
}

// Uint24

type streamReaderUint24 struct {
	bytes *streamReaderUnsafeBytes
}

func newStreamReaderUint24() *streamReaderUint24 {
	return &streamReaderUint24{
		bytes: newStreamReaderUnsafeBytes(3),
	}
}

func (s *streamReaderUint24) Get(source streamReaderSource) (Uint24, error) {
	bytes, err := s.bytes.Get(source)
	if err != nil {
		return 0, err
	}
	return readBytes.Uint24(bytes, 0), nil
}

// Uint24

type streamReaderUint32 struct {
	bytes *streamReaderUnsafeBytes
}

func newStreamReaderUint32() *streamReaderUint32 {
	return &streamReaderUint32{
		bytes: newStreamReaderUnsafeBytes(3),
	}
}

func (s *streamReaderUint32) Get(source streamReaderSource) (uint32, error) {
	bytes, err := s.bytes.Get(source)
	if err != nil {
		return 0, err
	}
	return readBytes.Uint32(bytes, 0), nil
}

// Uint48

type streamReaderUint48 struct {
	bytes *streamReaderUnsafeBytes
}

func newStreamReaderUint48() *streamReaderUint48 {
	return &streamReaderUint48{
		bytes: newStreamReaderUnsafeBytes(6),
	}
}

func (s *streamReaderUint48) Get(source streamReaderSource) (Uint48, error) {
	bytes, err := s.bytes.Get(source)
	if err != nil {
		return 0, err
	}
	return readBytes.Uint48(bytes, 0), nil
}

// UUID

type streamReaderUUID struct {
	bytes *streamReaderUnsafeBytes
}

func newStreamReaderUUID() *streamReaderUUID {
	return &streamReaderUUID{
		bytes: newStreamReaderUnsafeBytes(16),
	}
}

func (s *streamReaderUUID) Get(source streamReaderSource) (uuid.UUID, error) {
	bytes, err := s.bytes.Get(source)
	if err != nil {
		return uuid.Nil, err
	}
	return readBytes.UUID(bytes, 0), nil
}

// String

type streamReaderString struct {
	bytes *streamReaderUnsafeBytes
}

func newStreamReaderString(size uint16) *streamReaderString {
	return &streamReaderString{
		bytes: newStreamReaderUnsafeBytes(size),
	}
}

func (s *streamReaderString) Resize(size uint16) {
	s.bytes.Resize(size)
}

func (s *streamReaderString) Get(source streamReaderSource) (string, error) {
	bytes, err := s.bytes.Get(source)
	if err != nil {
		return "", err
	}
	return readBytes.String(bytes, 0, uint32(s.bytes.size)), nil
}
