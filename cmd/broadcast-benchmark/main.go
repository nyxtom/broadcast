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

func waitBenchAsync(cmd string, args ...interface{}) {
	defer wg.Done()

	c := client.Get()
	defer client.CloseConnection(c)
	for i := 0; i < loop; i++ {
		err := c.DoAsync(cmd, args...)
		if err != nil {
			fmt.Printf("do %s error %s", cmd, err.Error())
			return
		}
	}
}

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
		waitBench("SET", string(n), n)
	}

	bench("SET", f)
}

func benchPing() {
	f := func() {
		waitBench("PING")
	}

	bench("PING", f)
}

func benchIncr() {
	n := rand.Int()
	f := func() {
		waitBench("INCR", string(n), 1)
	}

	bench("INCR", f)
}

func benchDecr() {
	n := rand.Int()
	f := func() {
		waitBench("DECR", string(n), 1)
	}

	bench("DECR", f)
}

func benchGet() {
	f := func() {
		waitBench("GET", "foo")
	}

	bench("GET", f)
}

func benchDel() {
	f := func() {
		waitBench("DEL", "foo")
	}

	bench("DEL", f)
}

func benchCount() {
	f := func() {
		waitBenchAsync("COUNT", "foo", 1)
	}

	bench("COUNT", f)
}

func benchSetNx() {
	n := rand.Int()
	f := func() {
		waitBench("SETNX", string(n), n)
	}

	bench("SETNX", f)
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
	benchPing()
	benchGet()
	benchDel()
	benchIncr()
	benchDecr()
	benchCount()
	benchSetNx()
}
