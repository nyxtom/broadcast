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
+ pluggable protocols (redis protocol implemented)
+ supports reading and writing: int64, float64, string, byte, []byte,
  error, and bool.
+ registered command callbacks will receive typed data as it was parsed
  (i.e. ADD will receive an array of numbers which might be floats/ints)
+ broadcast-cli connects to the server to see a list of routines supported
  by the server and caches them locally for easy lookup & auto-completion
+ stats-aware, broadcast has built in stats tracking to not only monitor 
  it's own state and report it out, but is able to handle generic stat
  routines through extended modules (i.e. broadcast-stats)
+ broadcast-benchmark for benchmarking various async/non-async commands

## TODO
+ configuration/support for listening on udp
+ cluster-aware configuration, allow broadcast-server to inspect commands 
  and determine whether both the command or the first key of the arguments
  is handled by a different server altogether using a hashring config.
+ add additional language clients (node.js, python, c#) 
+ simple pub/sub protocol

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

Commands can be async (fire and forget) or non-async (i.e. the callback
will write a response to the command line. Specifying the type of async 
command allows the client library to know whether to read directly off of
the response when sending a command or to simple fire and forget. This is
useful if we are dealing with a lot of messages and we don't particularly
care about the response. For instance, we could perform the following:

```
$ broadcast-cli -h="127.0.0.1" -p=7331
127.0.0.1:7331> INCR foo 234

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

Echo is another simple command that is automaticall registered on
start-up by the broadcast-server via the default backend.

```
package server

func (b *DefaultBackend) echo(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteString("")
		client.Flush()
		return nil
	} else {
		if len(d) == 1 {
			client.WriteInterface(d[0])
		} else {
			client.WriteArray(d)
		}
		client.Flush()
		return nil
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

### Broadcast-Cli

broadcast-cli is the command line tool we can use to connect to any
broadcast-server. Primarily the cli is meant to issue commands and serves
as an example of how to write a broadcast client in golang. The client
library being used to implement the cli is located in *client/go/broadcast*.

```
$ broadcast-cli -h="127.0.0.1" -p=7331
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


[45206] 27 Aug 14 13:27 MDT # broadcast server started listening on 127.0.0.1:7331
```

```
$ broadcast-cli -h="127.0.0.1" -p=7331
127.0.0.1:7331> CMDS
SETNX
 Sets the specified key to the given value only if the key is not already set.
 usage: SETNX key 1234

EXISTS
 Determines if the given key exists from the values.
 usage: EXISTS key

GET
 Gets the specified key from the values.
 usage: GET key

DECR
 Decrements a key by the specified value or by default 1.
 usage: DECR key [1]

DEL
 Deletes a key from the values or counters list or both.
 usage: DEL key

PING
 Pings the server for a response

CMDS
 List of available commands supported by the server

COUNT
 Increments a key that resets itself to 0 on each flush routine.
 usage: COUNT foo [124]

COUNTERS
 Returns the list of active counters.

SET
 Sets the specified key to the specified value in values.
 usage: SET key 1234

ECHO
 Echos back a message sent
 usage: ECHO "hello world"

INCR
 Increments a key by the specified value or by default 1.
 usage: INCR key [1]

INFO
 Current server status and information

KEYS
 Returns the list of keys available or by pattern
 usage: KEYS [pattern]

127.0.0.1:7331>
```
