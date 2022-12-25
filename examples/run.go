package main

import (
	"fmt"
	"github.com/sockety/sockety-go"
	"sync"
	"sync/atomic"
	"time"
)

var i uint32 = 0

func onNewClient(c sockety.Conn) {
	id := atomic.AddUint32(&i, 1)

	fmt.Println(fmt.Sprintf("[Server] Connected client #%d", id))
	for m := range c.Messages() {
		fmt.Println("[Server] Received:", m)
	}

	fmt.Println(fmt.Sprintf("[Server] Disconnected client #%d", id))
}

func onServerError(e error) {
	panic(e)
}

func runClient(i int, wg *sync.WaitGroup) {
	endAt := time.Now().Add(time.Second)
	var client sockety.Conn
	var err error
	for {
		client, err = sockety.Dial("tcp", ":3333", sockety.ConnOptions{})
		if err == nil || endAt.Before(time.Now()) {
			break
		}
	}
	if err != nil {
		panic(err)
	}
	fmt.Println(fmt.Sprintf("[Client#%d] Connected", i))
	go func() {
		for m := range client.Messages() {
			fmt.Println(fmt.Sprintf("[Client#%d] Received:", i), m)
		}
	}()

	// Pass simple messages
	for j := 0; j < 5000; j++ {
		err = sockety.NewMessageDraft("ping").PassTo(client)
		if err != nil {
			panic(err)
		}
		fmt.Println(fmt.Sprintf("[Client#%d] Sent message", i))
	}
	client.Close()
	fmt.Println(fmt.Sprintf("[Client#%d] Closed", i))
	wg.Done()
}

func main() {
	// Set up server
	server := sockety.NewServer(&sockety.ServerOptions{
		HandleError: onServerError,
	})
	err := server.Listen("tcp", ":3333")
	if err != nil {
		panic(err)
	}
	var serverWg sync.WaitGroup
	go func() {
		for {
			c, err := server.Accept()
			if err != nil {
				panic(err)
			}
			serverWg.Add(1)
			onNewClient(c)
			serverWg.Done()
		}
	}()

	// Set up clients
	clientsCount := 1
	var clientWg sync.WaitGroup
	clientWg.Add(clientsCount)
	for i := 1; i <= clientsCount; i++ {
		go runClient(i, &clientWg)
	}
	clientWg.Wait()
	fmt.Println("Message handler ended for all connections")

	// Wait for processing done
	serverWg.Wait()
	fmt.Println("Message handler ended for all connections")
}
