package buffer_pool

import "github.com/valyala/bytebufferpool"

func Obtain(size uint64) *bytebufferpool.ByteBuffer {
	b := ObtainUnsafe(size)
	b.Reset()
	return b
}

func ObtainUnsafe(size uint64) *bytebufferpool.ByteBuffer {
	b := bytebufferpool.Get()
	b.B = append(b.B, make([]byte, size)...)
	return b
}

func Release(b *bytebufferpool.ByteBuffer) {
	bytebufferpool.Put(b)
}
