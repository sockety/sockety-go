package sockety

import (
	"errors"
	"io"
)

type bufferedReader struct {
	source    io.Reader
	eof       bool
	buffer    []byte
	available offset
	start     offset
}

type BufferedReader interface {
	Len() offset
	MayHave(size offset) bool
	ReadByte() (byte, error)
	Read(target []byte) (offset, error)
	Preload() error
}

func newBufferedReader(source io.Reader, buffer []byte) BufferedReader {
	if err := validateReadBufferSize(offset(len(buffer))); err != nil {
		panic(err)
	}

	return &bufferedReader{
		source: source,
		buffer: buffer,
	}
}

func (b *bufferedReader) loadBytes() error {
	bytes, err := b.source.Read(b.buffer[b.start+b.available:])
	if err != nil {
		return err
	}
	b.available += offset(bytes)
	return nil
}

func (b *bufferedReader) Preload() error {
	if b.available > 0 {
		return nil
	} else if b.eof {
		return io.EOF
	}
	return b.loadBytes()
}

func (b *bufferedReader) ensure(size offset) error {
	// TODO: Think if there is more performant way
	if offset(len(b.buffer))-b.start < size {
		b.buffer = append(b.buffer[b.start:], b.buffer[:b.start]...)
		b.start = 0
	}

	for b.available < size {
		err := b.loadBytes()
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *bufferedReader) collect(size offset) {
	b.start += size
	b.available -= size
	if b.available == 0 {
		b.start = 0
	}
}

func (b *bufferedReader) Len() offset {
	return b.available
}

func (b *bufferedReader) MayHave(size offset) bool {
	if b.eof {
		return b.available >= size
	}
	return true
}

func (b *bufferedReader) ReadByte() (byte, error) {
	err := b.ensure(1)
	if err != nil {
		return 0, err
	}
	result := b.buffer[b.start]
	b.collect(1)
	return result, nil
}

func (b *bufferedReader) Read(target []byte) (offset, error) {
	// Set up
	o := offset(0)
	l := offset(len(target))

	// Read rest
	for o < l {
		left := l - o

		// Read current available bytes
		if b.available >= left {
			copy(target[o:], b.buffer[b.start:b.start+left])
			b.collect(left)
			return l, nil
		} else if b.available > 0 {
			copy(target[o:], b.buffer[b.start:b.start+b.available])
			o += b.available
			b.collect(b.available)
		}

		err := b.loadBytes()
		if err != nil {
			return o, err
		}
	}
	return o, nil
}

type limitedBufferedReader struct {
	b    BufferedReader
	size offset
}

func limitBufferedReader(reader BufferedReader, size offset) BufferedReader {
	return &limitedBufferedReader{
		b:    reader,
		size: size,
	}
}

func (b *limitedBufferedReader) collect(size offset) error {
	if size > b.size {
		return errors.New("sockety.limitedBufferedReader.collect: out of bounds")
	}
	b.size -= size
	return nil
}

func (b *limitedBufferedReader) Len() offset {
	l := b.b.Len()
	if b.size > l {
		return l
	}
	return b.size
}

func (b *limitedBufferedReader) MayHave(size offset) bool {
	return size <= b.size && b.b.MayHave(size)
}

func (b *limitedBufferedReader) ReadByte() (byte, error) {
	err := b.collect(1)
	if err != nil {
		return 0, err
	}
	return b.b.ReadByte()
}

func (b *limitedBufferedReader) Read(target []byte) (offset, error) {
	err := b.collect(offset(len(target)))
	if err != nil {
		return 0, err
	}
	return b.b.Read(target)
}

func (b *limitedBufferedReader) Preload() error {
	if b.size == 0 {
		return io.EOF
	}
	return b.b.Preload()
}
