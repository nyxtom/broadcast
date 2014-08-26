package main

import (
	"flag"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/nyxtom/broadcast/client/go/broadcast"
)

var ip = flag.String("ip", "127.0.0.1", "broadcast-server ip")
var port = flag.Int("port", 7331, "broadcast-server port")
var number = flag.Int("n", 1000, "request number")
var clients = flag.Int("c", 50, "number of clients")

var wg sync.WaitGroup
var client *broadcast.Client

var loop = 0

func waitBench(cmd string, args ...interface{}) {
	defer wg.Done()

	c := client.Get()
	defer client.CloseConnection(c)
	for i := 0; i < loop; i++ {
		_, err := c.Do(cmd, args...)
		if err != nil {
			fmt.Printf("do %s error %s", cmd, err.Error())
			return
		}
	}
}

func bench(cmd string, f func()) {
	wg.Add(*clients)

	t1 := time.Now().UnixNano()
	for i := 0; i < *clients; i++ {
		go f()
	}

	wg.Wait()

	t2 := time.Now().UnixNano()
	delta := float64(t2-t1) / float64(time.Second)

	fmt.Printf("%s: %0.2f requests per second\n", cmd, (float64(*number) / delta))
}

func benchSet() {
	n := rand.Int()
	f := func() {
		waitBench("set", string(n), n)
	}

	bench("set", f)
}

func main() {
	flag.Parse()

	if *number <= 0 {
		panic("invalid number")
		return
	}

	if *clients <= 0 || *number < *clients {
		panic("invalid client number")
		return
	}

	loop = *number / *clients
	client, _ = broadcast.NewClient(*port, *ip, 1)
	benchSet()
}
