package benchmark

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/sockety/sockety-go"
	"io"
	"runtime"
	"sync/atomic"
	"testing"
)

// Utilities

type MockReadWriter struct {
	Reader io.Reader
	Writer io.Writer
}

func (m MockReadWriter) Write(b []byte) (int, error) {
	return m.Writer.Write(b)
}

func (m MockReadWriter) Read(b []byte) (int, error) {
	return m.Reader.Read(b)
}

type RepeatReader struct {
	remaining chan []byte
	data      chan []byte
	content   []byte
}

func newRepeatReader(header []byte, repeat []byte) *RepeatReader {
	ch := make(chan []byte)
	rm := make(chan []byte, 1)
	rm <- header
	return &RepeatReader{
		remaining: rm,
		data:      ch,
		content:   repeat,
	}
}

func newRepeatReaderWithBuffer(header []byte, repeat []byte, buffer uint32) *RepeatReader {
	ch := make(chan []byte, buffer)
	rm := make(chan []byte, 1)
	rm <- header
	return &RepeatReader{
		remaining: rm,
		data:      ch,
		content:   repeat,
	}
}

func (r *RepeatReader) GetRemaining() []byte {
	select {
	case x := <-r.remaining:
		return x
	default:
		return nil
	}
}

func (r *RepeatReader) Repeat() {
	r.data <- r.content
}

func (r *RepeatReader) Read(bytes []byte) (int, error) {
	rem := r.GetRemaining()
	if rem != nil {
		copy(bytes, rem)
		return len(rem), nil
	}
	data := <-r.data
	if len(data) <= len(bytes) {
		copy(bytes, data)
		return len(data), nil
	}
	copy(bytes, data[:len(bytes):len(bytes)])
	r.remaining <- data[len(bytes):]
	return len(bytes), nil
}

func CreatePool[T any](count int, create func() T) func() T {
	countRaw := uint32(count)
	indexRaw := uint32(0)
	pool := make([]T, countRaw)
	for i := uint32(0); i < countRaw; i++ {
		pool[i] = create()
	}

	return func() T {
		index := atomic.AddUint32(&indexRaw, 1)
		return pool[index%countRaw]
	}
}

func RunBenchmark(b *testing.B, concurrency []int, handler func(run func(fn func()))) {
	uuid.EnableRandPool()

	for _, c := range concurrency {
		if c < 1 {
			panic("invalid concurrency")
		} else if c == 1 {
			handler(func(fn func()) {
				b.ResetTimer()
				b.Run("NoConcurrency/CPUs", func(b2 *testing.B) {
					for i := 0; i < b2.N; i++ {
						fn()
					}
				})
			})
		} else {
			cpus := runtime.GOMAXPROCS(0)
			parallelism := c / cpus
			realConcurrency := cpus * parallelism
			name := fmt.Sprintf("Concurrency-%d/CPUs", realConcurrency)
			handler(func(fn func()) {
				b.ResetTimer()
				b.Run(name, func(b2 *testing.B) {
					b2.SetParallelism(parallelism)
					b2.ResetTimer()
					b2.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							fn()
						}
					})
				})
			})
		}
	}
}

func RunDefaultBenchmark(b *testing.B, handler func(run func(fn func()))) {
	RunBenchmark(b, []int{1, 10, 100, 1000}, handler)
}

// Settings

const readBufferSize = 4_096
const maxChannels = 4_096

// Test utilities

func createMockConn() sockety.Conn {
	target := MockReadWriter{
		Reader: bytes.NewReader([]byte{227}),
		Writer: io.Discard,
	}
	conn, err := sockety.NewConn(context.Background(), target, sockety.ConnOptions{
		ReadBufferSize: readBufferSize,
		Channels:       maxChannels,
		WriteChannels:  maxChannels,
	})
	if err != nil {
		panic(err)
	}
	return conn
}

func getMessageBytes(message sockety.Producer) []byte {
	// Compute example byte array for the message
	buffer := bytes.NewBuffer(make([]byte, 0))
	target := MockReadWriter{
		Reader: bytes.NewReader([]byte{227}),
		Writer: buffer,
	}
	conn, err := sockety.NewConn(context.Background(), target, sockety.ConnOptions{})
	if err != nil {
		panic(err)
	}
	err = conn.Pass(message)
	if err != nil {
		panic(err)
	}
	result := make([]byte, buffer.Len())
	_, err = buffer.Read(result)
	if err != nil {
		panic(err)
	}
	return result[1:]
}

func PrepareServer(handler func(c sockety.Conn)) sockety.Server {
	server := sockety.NewServer(&sockety.ServerOptions{
		ReadBufferSize: readBufferSize,
		Channels:       maxChannels,
		WriteChannels:  maxChannels,
		HandleError: func(err error) {
			panic(err)
		},
	})

	err := server.Listen("tcp", ":3333")
	if err != nil {
		panic(err)
	}

	go func() {
		for c := range server.Next() {
			go handler(c)
		}
	}()

	return server
}

func PrepareClient() sockety.Conn {
	client, err := sockety.Dial("tcp", ":3333", sockety.ConnOptions{
		WriteChannels:  maxChannels,
		Channels:       maxChannels,
		ReadBufferSize: readBufferSize,
	})
	if err != nil {
		panic(err)
	}

	go func() {
		for range client.Messages() {
		}
	}()

	return client
}

// Tests

func Benchmark_Parse_One(b *testing.B) {
	message := getMessageBytes(sockety.NewMessageDraft("ping"))

	RunBenchmark(b, []int{1}, func(run func(fn func())) {
		reader := newRepeatReader([]byte{227}, message)
		target := MockReadWriter{
			Reader: reader,
			Writer: io.Discard,
		}
		conn, err := sockety.NewConn(context.Background(), target, sockety.ConnOptions{
			ReadBufferSize: readBufferSize,
			Channels:       maxChannels,
			WriteChannels:  maxChannels,
		})
		if err != nil {
			panic(err)
		}
		go func() {
			for range conn.Messages() {
			}
		}()

		run(func() {
			reader.Repeat()
		})
	})
}

func Benchmark_Parse_PoolCPU(b *testing.B) {
	RunBenchmark(b, []int{1, runtime.GOMAXPROCS(0), runtime.GOMAXPROCS(0) * 2}, func(run func(fn func())) {
		reader := CreatePool(runtime.GOMAXPROCS(0), func() *RepeatReader {
			message := getMessageBytes(sockety.NewMessageDraft("ping"))
			reader := newRepeatReader([]byte{227}, message)
			target := MockReadWriter{
				Reader: reader,
				Writer: io.Discard,
			}
			conn, err := sockety.NewConn(context.Background(), target, sockety.ConnOptions{
				ReadBufferSize: readBufferSize,
				Channels:       maxChannels,
				WriteChannels:  maxChannels,
			})
			if err != nil {
				panic(err)
			}
			go func() {
				for range conn.Messages() {

				}
			}()
			return reader
		})

		run(func() {
			reader().Repeat()
		})
	})
}

func Benchmark_Build(b *testing.B) {
	RunBenchmark(b, []int{1, 10, 100}, func(run func(fn func())) {
		conn := createMockConn()
		message := sockety.NewMessageDraft("ping")

		run(func() {
			conn.Pass(message)
		})
	})
}

func Benchmark_Build_PoolCPU(b *testing.B) {
	RunDefaultBenchmark(b, func(run func(fn func())) {
		conn := CreatePool(runtime.GOMAXPROCS(0), createMockConn)
		message := sockety.NewMessageDraft("ping")

		run(func() {
			conn().Pass(message)
		})
	})
}

func Benchmark_Build_Live(b *testing.B) {
	RunBenchmark(b, []int{1, 10, 100}, func(run func(fn func())) {
		conn := createMockConn()
		run(func() {
			sockety.NewMessageDraft("ping").PassTo(conn)
		})
	})
}

func Benchmark_Build_Request(b *testing.B) {
	RunBenchmark(b, []int{1, 10, 100}, func(run func(fn func())) {
		conn := createMockConn()
		message := sockety.NewMessageDraft("ping")

		run(func() {
			req := conn.Request(message)
			req.Send()
		})
	})
}

func Benchmark_Send(b *testing.B) {
	RunBenchmark(b, []int{1, 10}, func(run func(fn func())) {
		server := PrepareServer(func(c sockety.Conn) {
			for range c.Messages() {
			}
		})
		defer server.Close()
		client := PrepareClient()
		message := sockety.NewMessageDraft("ping")

		run(func() {
			client.Pass(message)
		})
	})
}

func Benchmark_Send_PoolCPU(b *testing.B) {
	RunDefaultBenchmark(b, func(run func(fn func())) {
		server := PrepareServer(func(c sockety.Conn) {
			for range c.Messages() {
			}
		})
		defer server.Close()
		client := CreatePool(runtime.GOMAXPROCS(0), PrepareClient)
		message := sockety.NewMessageDraft("ping")

		run(func() {
			client().Pass(message)
		})
	})
}
