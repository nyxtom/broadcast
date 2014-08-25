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

+ broadcast-server can listen on tcp or udp
+ supports reading and writing: int64, float64, string, byte, []byte,
  error, and bool.
+ registered command callbacks will receive typed data as it was parsed
  (i.e. ADD will receive an array of numbers which might be floats/ints)
+ broadcast-cli connects to the server to see a list of routines supported
  by the server and caches them locally for easy lookup & auto-completion
+ cluster-aware, broadcast-server can easily distribute load using a
  hashring configuration. (i.e. broadcast-cluster)
+ stats-aware, broadcast has built in stats tracking to not only monitor 
  it's own state and report it out, but is able to handle generic stat
  routines through extended modules (i.e. broadcast-stats)
+ supported clients including: Golang, node.js, C/C++, Python

## Build and Install

Installation is super easy if you already have a workspace defined with
golang installed. Otherwise, you can create the workspace and run make.

```
go get github.com/nyxtom/broadcast
```

## Broadcast-Stats: statsd example server

Broadcast-stats is a server that runs on top of the broadcast library.
This simple server will store an in-memory model of stat data, while
responding to various stat commands registered when the server starts up.
Commands can be retrieved from any broadcast-cli appropriately.
