package benchmark

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"github.com/google/uuid"
	"github.com/sockety/sockety-go"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Utilities

type BufferPool struct {
	b sync.Pool
}

func NewBufferPool(size uint32) *BufferPool {
	return &BufferPool{
		b: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
	}
}

func (p *BufferPool) Get() []byte {
	return p.b.Get().([]byte)
}

func (p *BufferPool) Put(b []byte) {
	p.b.Put(b)
}

type MockReadWriteCloser struct {
	closed atomic.Bool
	Reader io.Reader
	Writer io.Writer
}

func (m MockReadWriteCloser) Write(b []byte) (int, error) {
	return m.Writer.Write(b)
}

func (m MockReadWriteCloser) Read(b []byte) (int, error) {
	if m.closed.Load() {
		return 0, io.EOF
	}
	return m.Reader.Read(b)
}

func (m MockReadWriteCloser) Close() error {
	m.closed.Store(true)
	return nil
}

type RepeatReader struct {
	remaining chan []byte
	data      chan []byte
	content   []byte
}

func randomBytes(size uint32) []byte {
	data := make([]byte, size)
	_, err := rand.Read(data)
	if err != nil {
		panic(err)
	}
	return data
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

func (r *RepeatReader) pass(target []byte, data []byte) (int, error) {
	if len(data) <= len(target) {
		copy(target, data)
		return len(data), nil
	}
	copy(target, data[:len(target)])
	r.remaining <- data[len(target):]
	return len(target), nil
}

func (r *RepeatReader) Read(target []byte) (int, error) {
	rem := r.GetRemaining()
	if rem != nil {
		return r.pass(target, rem)
	}
	return r.pass(target, <-r.data)
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

func MemStatDiff[T interface{ uint32 | uint64 }](a, b T) int64 {
	return int64(a) - int64(b)
}

func ToKiB[T interface{ int64 | uint32 | uint64 }](b T) T {
	return b / 1024
}

func ToMiB[T interface{ int64 | uint32 | uint64 }](b T) T {
	return ToKiB(b) / 1024
}

func GetMemStats() runtime.MemStats {
	runtime.GC()
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}

func PrintMemStats(prevStats runtime.MemStats) {
	m := GetMemStats()
	fmt.Printf("\033[36mAlloc = %d KiB", ToKiB(MemStatDiff(m.Alloc, prevStats.Alloc)))
	fmt.Printf("\tInUse = %d KiB", ToKiB(MemStatDiff(m.HeapInuse, prevStats.HeapInuse)))
	fmt.Printf("\tTotalAlloc = %d MiB", ToMiB(MemStatDiff(m.TotalAlloc, prevStats.TotalAlloc)))
	fmt.Printf("\tNumGC = %d", MemStatDiff(m.NumGC-3, prevStats.NumGC))
	fmt.Printf("\t\033[1;30mSys = %d MiB\033[0m\n", ToMiB(m.Sys))
}

func RunBenchmark(b *testing.B, concurrency []int, handler func(run func(fn func()))) {
	uuid.EnableRandPool()

	for _, c := range concurrency {
		if c < 1 {
			panic("invalid concurrency")
		} else if c == 1 {
			var b2Copy *testing.B
			prevStats := GetMemStats()
			handler(func(fn func()) {
				b.ResetTimer()
				b.Run("NoConcurrency/CPUs", func(b2 *testing.B) {
					b2Copy = b2
					for i := 0; i < b2.N; i++ {
						fn()
					}
				})
			})
			if b2Copy != nil && !b2Copy.Skipped() {
				PrintMemStats(prevStats)
			}
		} else {
			cpus := runtime.GOMAXPROCS(0)
			parallelism := c / cpus
			realConcurrency := cpus * parallelism
			name := fmt.Sprintf("Concurrency-%d/CPUs", realConcurrency)
			prevStats := GetMemStats()
			var b2Copy *testing.B
			handler(func(fn func()) {
				b.ResetTimer()
				b.Run(name, func(b2 *testing.B) {
					b2Copy = b2
					b2.SetParallelism(parallelism)
					b2.ResetTimer()
					b2.RunParallel(func(pb *testing.PB) {
						for pb.Next() {
							fn()
						}
					})
				})
			})
			if b2Copy != nil && !b2Copy.Skipped() {
				PrintMemStats(prevStats)
			}
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
	target := MockReadWriteCloser{
		Reader: bytes.NewReader([]byte{227}),
		Writer: io.Discard,
	}
	conn, err := sockety.NewConn(context.Background(), target, &sockety.ConnOptions{
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
	target := MockReadWriteCloser{
		Reader: bytes.NewReader([]byte{227}),
		Writer: buffer,
	}
	conn, err := sockety.NewConn(context.Background(), target, &sockety.ConnOptions{})
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

func HandleMessages(handler func(m sockety.Message)) func() {
	progress := uint32(0)
	server := PrepareServer(func(c sockety.Conn) {
		for m := range c.Messages() {
			atomic.AddUint32(&progress, 1)
			go func(m sockety.Message) {
				handler(m)
				atomic.AddUint32(&progress, ^uint32(0))
			}(m)
		}
	})

	return func() {
		for {
			<-time.After(300 * time.Millisecond)
			left := atomic.LoadUint32(&progress)
			if left == 0 {
				break
			}
			fmt.Println("Waiting for finish of:", left)
		}
		server.Close()
	}
}

func PrepareClient() sockety.Conn {
	client, err := sockety.Dial("tcp", ":3333", &sockety.ConnOptions{
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
		target := MockReadWriteCloser{
			Reader: reader,
			Writer: io.Discard,
		}
		conn, err := sockety.NewConn(context.Background(), target, &sockety.ConnOptions{
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
			target := MockReadWriteCloser{
				Reader: reader,
				Writer: io.Discard,
			}
			conn, err := sockety.NewConn(context.Background(), target, &sockety.ConnOptions{
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

func Benchmark_Parse_1MB(b *testing.B) {
	message := getMessageBytes(sockety.NewMessageDraft("ping").RawData(randomBytes(1024 * 1024)))

	RunBenchmark(b, []int{1}, func(run func(fn func())) {
		reader := newRepeatReader([]byte{227}, message)
		target := MockReadWriteCloser{
			Reader: reader,
			Writer: io.Discard,
		}
		conn, err := sockety.NewConn(context.Background(), target, &sockety.ConnOptions{
			ReadBufferSize: readBufferSize,
			Channels:       maxChannels,
			WriteChannels:  maxChannels,
		})
		if err != nil {
			panic(err)
		}
		buf := make([]byte, 1024*1024)
		go func() {
			for m := range conn.Messages() {
				go io.ReadFull(m.Data(), buf)
			}
		}()

		run(func() {
			reader.Repeat()
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

func Benchmark_Build_1MB(b *testing.B) {
	RunBenchmark(b, []int{1, 10, 100}, func(run func(fn func())) {
		conn := createMockConn()
		data := randomBytes(1024 * 1024)
		message := sockety.NewMessageDraft("ping").RawData(data)

		run(func() {
			conn.Pass(message)
		})
	})
}

func Benchmark_Build_1MB_Stream(b *testing.B) {
	RunBenchmark(b, []int{1, 10, 100}, func(run func(fn func())) {
		conn := createMockConn()
		data := randomBytes(1024 * 1024)
		message := sockety.NewMessageDraft("ping").Stream()

		run(func() {
			req := conn.Request(message)
			go req.Send()
			stream := req.Stream()
			stream.Write(data)
			stream.Close()
			<-req.Done()
		})
	})
}

func Benchmark_Build_PoolCPU(b *testing.B) {
	RunBenchmark(b, []int{1, 10, 100}, func(run func(fn func())) {
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
		end := HandleMessages(func(m sockety.Message) {
		})
		defer end()
		client := CreatePool(runtime.GOMAXPROCS(0), PrepareClient)
		message := sockety.NewMessageDraft("ping")

		run(func() {
			client().Pass(message)
		})
	})
}

func Benchmark_Send_PoolCPU_1MB_Stream(b *testing.B) {
	RunDefaultBenchmark(b, func(run func(fn func())) {
		end := HandleMessages(func(m sockety.Message) {
			io.Copy(io.Discard, m.Stream())
		})
		defer end()
		data := randomBytes(1024 * 1024)
		client := CreatePool(runtime.GOMAXPROCS(0), PrepareClient)
		message := sockety.NewMessageDraft("ping").Stream()

		run(func() {
			req := client().Request(message)
			go req.Send()
			stream := req.Stream()
			stream.Write(data)
			stream.Close()
			<-req.Done()
		})
	})
}

func Benchmark_Send_PoolCPU_1MB_Data(b *testing.B) {
	RunDefaultBenchmark(b, func(run func(fn func())) {
		end := HandleMessages(func(m sockety.Message) {
			io.Copy(io.Discard, m.Data())
		})
		defer end()
		data := randomBytes(1024 * 1024)
		client := CreatePool(runtime.GOMAXPROCS(0), PrepareClient)
		message := sockety.NewMessageDraft("ping").RawData(data)

		run(func() {
			client().Pass(message)
		})
	})
}

func Benchmark_Send_PoolCPU_4MB_Data(b *testing.B) {
	RunDefaultBenchmark(b, func(run func(fn func())) {
		end := HandleMessages(func(m sockety.Message) {
			// m.Discard()
			io.Copy(io.Discard, m.Data())
		})
		defer end()
		data := randomBytes(4 * 1024 * 1024)
		client := CreatePool(runtime.GOMAXPROCS(0), PrepareClient)
		message := sockety.NewMessageDraft("ping").RawData(data)

		run(func() {
			client().Pass(message)
		})
	})
}
