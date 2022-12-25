package sockety

import (
	"reflect"
)

func getDefault[T any](value T, defaultValue T) T {
	if reflect.ValueOf(value).IsZero() {
		return defaultValue
	}
	return value
}

func getErrorOnly[T any](_ T, err error) error {
	return err
}

func createReadChannel[T any](v T) <-chan T {
	ch := make(chan T, 1)
	ch <- v
	close(ch)
	return ch
}

func putOptional[T any](ch chan T, v T) {
	select {
	case ch <- v:
	default:
	}
}

func validateNetwork(network string) {
	switch network {
	case "tcp", "tcp4", "tcp6", "unix":
	default:
		panic("unsupported network type for Sockety")
	}
}
