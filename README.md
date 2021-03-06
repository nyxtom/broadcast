# Broadcast

Broadcast is a high performance network daemon designed to run generic
command topologies. Typically, we all want to write servers that do lots
of fancy things, usually at a high velocity. Unfortunately, because of the
differing requirements in what we would like to support, much of the
underlying network code gets re-written for optimizations and other useful
features. Broadcast is an attempt to provide a very simple abstraction
layer that should allow new distributed servers to be built and deal
while being able to work directly with typed data.

## Features

+ broadcast-server can listen on tcp
+ pluggable protocols (redis, interface, line)
+ supports reading and writing: int64, float64, string, byte, []byte,
  error, and bool.
+ interface protocol will use registered command callbacks will receive typed data as it was parsed
  (i.e. SUM will receive an array of numbers which might be floats/ints)
+ broadcast-cli connects to the server to see a list of routines supported
  by the server and caches them locally for easy lookup & auto-completion
+ stats-aware, broadcast-server has built in stats backend that can be
  used to perform common stats commands similar to redis, counters similar
  to statsd, keys..etc. This backend works over the redis, interface and
  line protocols and are supported by broadcast-server.
+ broadcast-benchmark for benchmarking various async/non-async commands
+ commands can be written as fire-and-forget over the standard tcp stack
  that way clients know which commands need to be read immediately for
  replies. (i.e. COUNT foo will not return a reply).
+ pubsub backend for publishing/subscribing to topic channels

## TODO
+ cluster-aware configuration, allow broadcast-server to inspect commands 
  and determine whether both the command or the first key of the arguments
  is handled by a different server altogether using a hashring config.
+ add additional language clients (node.js, python, c#) 

## Build and Install

Installation is super easy if you already have a workspace defined with
golang installed. Otherwise, you can create the workspace and run make.

```
go get github.com/nyxtom/broadcast
```

## Broadcast-Stats: example server implementation with stats commands

Broadcast-stats is a server that runs on top of the broadcast library.
This simple server will store an in-memory model of stat data, while
responding to various stat commands registered when the server starts up.
Commands can be retrieved from any broadcast-cli appropriately.

```
broadcast-stats
```

The above command will load the backend from the location:

```
github.com/nyxtom/broadcast/backends/stats
```

An implementation of the stats backend will simply create an in-memory
solution for various commands, while registering all the available
commands for this particular backend with the app server. This is done via
the **RegisterBackend** executed by the broadcast-server executable. Note 
the implementation in **metrics.go**

```
func RegisterBackend(app *server.BroadcastServer) (server.Backend, error) {
	backend := new(StatsBackend)
	mem, err := NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	backend.mem = mem

	app.RegisterCommand(server.Command{"COUNT", "Increments a key that resets itself to 0 on each flush routine.", "COUNT foo [124]", true}, backend.Count)
	app.RegisterCommand(server.Command{"COUNTERS", "Returns the list of active counters.", "", false}, backend.Counters)
	app.RegisterCommand(server.Command{"INCR", "Increments a key by the specified value or by default 1.", "INCR key [1]", false}, backend.Incr)
	app.RegisterCommand(server.Command{"DECR", "Decrements a key by the specified value or by default 1.", "DECR key [1]", false}, backend.Decr)
	app.RegisterCommand(server.Command{"DEL", "Deletes a key from the values or counters list or both.", "DEL key", false}, backend.Del)
	app.RegisterCommand(server.Command{"EXISTS", "Determines if the given key exists from the values.", "EXISTS key", false}, backend.Exists)
	app.RegisterCommand(server.Command{"GET", "Gets the specified key from the values.", "GET key", false}, backend.Get)
	app.RegisterCommand(server.Command{"SET", "Sets the specified key to the specified value in values.", "SET key 1234", false}, backend.Set)
	app.RegisterCommand(server.Command{"SETNX", "Sets the specified key to the given value only if the key is not already set.", "SETNX key 1234", false}, backend.SetNx)
	app.RegisterCommand(server.Command{"KEYS", "Returns the list of keys available or by pattern", "KEYS [pattern]", false}, backend.Keys)
	return backend, nil
}
```

### Commands

```
package server

// Handler is the actual function declaration that is provided argument data, client, and server
type Handler func(interface{}, ProtocolClient) error

// Command describes a command handler with name, description, usage
type Command struct {
	Name        string // name of the command
	Description string // description of the command
	Usage       string // example usage of the command
	FireForget  bool   // true to ignore responses, false to wait for a response
}
```

Commands can be async (fire and forget) or non-async (i.e. the callback
will write a response to the command line. Specifying the type of async 
command allows the client library to know whether to read directly off of
the response when sending a command or to simple fire and forget. This is
useful if we are dealing with a lot of messages and we don't particularly
care about the response. For instance, we could perform the following:

```
$ broadcast-cli -h="127.0.0.1" -p=7331
127.0.0.1:7331> INCR foo 234
(integer) 234

127.0.0.1:7331> GET foo
(integer) 234

127.0.0.1:7331> SET foo 3
(integer) 1

127.0.0.1:7331> GET foo
(integer) 3

127.0.0.1:7331> COUNT test

127.0.0.1:7331> COUNTERS
test: Rate: AvgHistory:
  1)	(float) 0.000000
RatePerSecond: (float) 0.199962
Value: (float) 0.000000

127.0.0.1:7331> 
```

The server handlers are executed by the broadcast server whenever it encounters a command that was pre-registered. All callbacks 
will recieve a generic interface (generally this is an array but it might
be a single value depending on the use case), followed by a network client
that can be used to write back to the client response stream.

```
func (stats *StatsBackend) FlushInt(i int, err error, client server.ProtocolClient) error {
	if err != nil {
		return err
	}
	client.WriteInt64(int64(i))
	client.Flush()
	return nil
}

func (stats *StatsBackend) Exists(data interface{}, client server.ProtocolClient) error {
    d, _ := data.([][]byte)
    if len(d) == 0 {
        client.WriteError(errors.New("EXISTS takes at least 1 parameter (i.e. key to find)"))
        client.Flush()
        return nil
    } else {
        i, err := stats.mem.Exists(string(d[0]))
        return stats.FlushInt(i, err, client)
    }
}
```

Implementing a registered callback command is really simple to do once you get the hang of 
writing back to the client in whatever way you need to. You can even execute commands back 
to the client via WriteCommand.

## Broadcast-Server Simple Commands

### PING

Ping is a super simple command that is automatically registered by the
broadcast-server when it starts up. The implementation is below:

```
package server

var pong = "PONG"

func (b *DefaultBackend) ping(data interface{}, client server.ProtocolClient) error {
	client.WriteString(pong)
	client.Flush()
	return nil
}
```

And the broadcast-server will register this command on start-up like so:

```
app.RegisterCommand(server.Command{"PING", "Pings the server for a response", "", false}, backend.ping)
```

### ECHO

Echo is another simple command that is automatically registered on
start-up by the broadcast-server via the default backend.

```
package server

func (b *DefaultBackend) echo(data interface{}, client server.ProtocolClient) error {
	d, okInterface := data.([]interface{})
	if okInterface {
		if len(d) == 0 {
			client.WriteString("")
			client.Flush()
			return nil
		} else {
			if len(d) == 1 {
				fmt.Printf("%v", d[0])
				client.WriteInterface(d[0])
			} else {
				client.WriteArray(d)
			}
			client.Flush()
			return nil
		}
	} else {
		b, _ := data.([][]byte)
		if len(b) == 0 {
			client.WriteString("")
			client.Flush()
			return nil
		} else {
			if len(b) == 1 {
				client.WriteString(string(b[0]))
			} else {
				s := make([]string, len(b))
				for i, k := range b {
					s[i] = string(k)
				}
				client.WriteString(strings.Join(s, " "))
			}
			client.Flush()
			return nil
		}
	}
}
```

### SUM Example Command

Sum is a command that will add up all the given parameters that 
were passed into it by the client. Note how we can add both 
floats and integers by inspecting the type. This is a simple command 
that can be registered by the broadcast-server using the interface
protocol.

```
package main

import "errors"

func (b *CustomBackend) sum(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([]interface{})
	if len(d) < 1 {
		client.WriteError(errors.New("ADD takes at least 2 parameters"))
		client.Flush()
		return nil
	} else {
		sum := int64(0)
		sumf := float64(0)
		for _, a := range d {
			switch a := a.(type) {
			case int64:
				sum += a
			case float64:
				sumf += a
			}
		}
		if sumf != 0 {
			client.WriteFloat64(sumf + float64(sum))
		} else {
			client.WriteInt64(sum)
		}
		client.Flush()
		return nil
	}
}

func RegisterBackend(app *server.BroadcastServer) (server.Backend, error) {
	backend := new(CustomBackend)
	app.RegisterCommand(server.Command{"SUM", "Adds a set of numbers together", "SUM num num [num num ...]", false}, backend.sum)
	return backend, nil
}

func (b *CustomBackend) Load() error {
	return nil
}

func (b *CustomBackend) Unload() error {
	return nil
}
```

Then in the actual app-server itself, you can call the load backend to
ensure that our newly created custom backend is loaded. We will use the 
default broadcast server protocol which allows us to work directly with
interfaces instead of [][]byte data when it's written to and from clients
to and from servers.

```
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/nyxtom/broadcast/server"
)

var LogoHeader = `
	%s %s %s
	Port: %d
	PID: %d
`

func main() {
	// Parse out flag parameters
	var host = flag.String("h", "127.0.0.1", "Broadcast custom host to bind to")
	var port = flag.Int("p", 7331, "Broadcast custom port to bind to")

	// create a new broadcast server
	app, err := server.Listen(*port, *host)
	app.Header = ""
	app.Name = "Broadcast Custom"
	app.Version = "0.1"
	app.Header = LogoHeader
	if err != nil {
		fmt.Println(err)
		return
	}

	// setup custom backend as created above
	backend, err := RegisterBackend(app)
	if err != nil {
		fmt.Println(err)
		return
	}
	app.LoadBackend(backend)

	// wait for all events to fire so we can log them
	pid := os.Getpid()
	go func() {
		for !app.Closed {
			event := <-app.Events
			t := time.Now()
			delim := "#"
			if event.Level == "error" {
				delim = "ERROR:"
			}
			msg := fmt.Sprintf("[%d] %s %s %s", pid, t.Format(time.RFC822), delim, event.Message)
			if event.Err != nil {
				msg += fmt.Sprintf(" %v", event.Err)
			}

			fmt.Println(msg)
		}
	}()

	go func() {
		<-app.Quit
		pprof.StopCPUProfile()
		os.Exit(0)
	}()

	// attach to any signals that would cause our app to close
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt)

	go func() {
		<-sc
		app.Close()
	}()

	// accept incomming connections!
	app.AcceptConnections()
}
```

### Line Protocol

broadcast-server also implements a much simpler protocol which is in the
format of *CMD arg1 arg2...\n*. If you startup the broadcast server with
the following command line:

```
$ broadcast-server -h="127.0.0.1" -p=7331 -bprotocol="line"
[71443] 14 Sep 14 12:36 MDT # WARNING: no config file specified, using the default config
[71443] 14 Sep 14 12:36 MDT #

  _                             Broadcast 0.1.0 64 bit
   /_)__  _   _/_  _   __/_
   /_)//_//_|/_//_ /_|_\ /      Port: 7331
                                PID: 71443


[71443] 14 Sep 14 12:36 MDT # setting read/write protocol to line
[71443] 14 Sep 14 12:36 MDT # listening for incoming connections on 127.0.0.1:7331
```

Then you can access it via echo and netcat commands make this process much simpler.

```
$ echo "SET foo 9" | nc 127.0.0.1 7331
:1

$ echo "GET foo" | nc 127.0.0.1 7331
:9
```

### Broadcast-Cli

broadcast-cli is the command line tool we can use to connect to any
broadcast-server. Primarily the cli is meant to issue commands and serves
as an example of how to write a broadcast client in golang. The client
library being used to implement the cli is located in *client/go/broadcast*. 
broadcast-cli can be used to set the protocol based on the currently
implemented protocol clients via the *-bprotocol" command line flag.

```
$ broadcast-cli -h="127.0.0.1" -p=7331 -bprotocol="redis"
127.0.0.1:7331> CMDS
CMDS
 List of available commands supported by the server

ECHO
 Echos back a message sent
 usage: ECHO "hello world"

INFO
 Current server status and information

PING
 Pings the server for a response

127.0.0.1:7331> 
```

*broadcast-cli* also provides command completion and help via help
<command>. All commands are cached on start-up, to refresh the list of
available commands (in the event the server is restarted with new
commands) you will need to be sure to exit the cli as well.

With broadcast-stats, the available commands will shift to
something more along the lines of below:

```
$ broadcast-stats
[45206] 27 Aug 14 13:27 MDT # WARNING: no config file specified, using the default config
[45206] 27 Aug 14 13:27 MDT #

  _                         _                  Broadcast Stats 0.1 64 bit
   /_)__  _   _/_  _   __/_  /_'_/__ _/_ _
   /_)//_//_|/_//_ /_|_\ /   ._/ / /_|/ _\     Port: 7331
                                               PID: 45206


[45206] 27 Aug 14 13:27 MDT # setting read/write protocol to redis
[45206] 27 Aug 14 13:27 MDT # broadcast server started listening on 127.0.0.1:7331
```

```
$ broadcast-cli -h="127.0.0.1" -p=7331 -bprotocol="redis"
127.0.0.1:7331> CMDS
CMDS
 List of available commands supported by the server

COUNT
 Increments a key that resets itself to 0 on each flush routine.
 usage: COUNT foo [124]

COUNTERS
 Returns the list of active counters.

DECR
 Decrements a key by the specified value or by default 1.
 usage: DECR key [1]

DEL
 Deletes a key from the values or counters list or both.
 usage: DEL key

ECHO
 Echos back a message sent
 usage: ECHO "hello world"

EXISTS
 Determines if the given key exists from the values.
 usage: EXISTS key

GET
 Gets the specified key from the values.
 usage: GET key

INCR
 Increments a key by the specified value or by default 1.
 usage: INCR key [1]

INFO
 Current server status and information

KEYS
 Returns the list of keys available or by pattern
 usage: KEYS [pattern]

PING
 Pings the server for a response

PUBLISH
 Publishes to a specified topic given the data/arguments
 usage: PUBLISH topic message

SADD
 Adds one or more members to a set
 usage: SADD key member [member ...]

SCARD
 Gets the number of members from a set
 usage: SCARD key [key ...]

SDIFF
 Subtracts multiple sets
 usage: SDIFF key [key ...]

SET
 Sets the specified key to the specified value in values.
 usage: SET key 1234

SETNX
 Sets the specified key to the given value only if the key is not already set.
 usage: SETNX key 1234

SINTER
 Returns the members of the set resulting from the intersection of all the given sets
 usage: SINTER key [key ...]

SISMEMBER
 Returns if member is a member of the set
 usage: SISMEMBER key member [member ...]

SMEMBERS
 Gets all the members in a set
 usage: SMEMBERS key

SREM
 Removes one or more members from a set
 usage: SREM key member [member ...]

SUBSCRIBE
 Subscribes to a specified topic
 usage: SUBSCRIBE topic [topic ...]

SUNION
 Returns the members of the set resulting from the union of all the given sets
 usage: SINTER key [key ...]

UNSUBSCRIBE
 Unsubscribes from a specified topic
 usage: UNSUBSCRIBE topic [topic ...]

127.0.0.1:7331>
```

## License
The MIT License (MIT)

Copyright (c) 2014 Thomas Holloway

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
