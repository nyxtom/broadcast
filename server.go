package server

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"runtime"
	"strconv"
	"time"
)

// error constants associated with the broadcast server
var errLineFormat = errors.New("bad response line format")
var errReadRequest = errors.New("invalid request protocol")

// BroadcastServer represents a construct for the application as a whole including
// the various address, protocol, network listener, connected clients, and overall
// server state that can be used for either reporting, or communicating with services.
type BroadcastServer struct {
	protocol string
	port     int                        // port to listen on
	host     string                     // host to bind to
	listener *net.Listener              // listener for the broadcast server
	closed   bool                       // closed is the boolean for when the application has already been closed
	quit     chan struct{}              // quit is a simple channel signal for when the application quits
	events   chan BroadcastEvent        // events is a channel for when emitted data occurs in the application
	clients  map[string]BroadcastClient // clients is a map of all the connected clients to the server
	commands map[byte]BroadcastHandler  // commands is a map of all the available commands executable by the server
	size     int                        // size is the number of total clients connected to the server
}

// Listen will use the given address parameters to construct a simple server that listens for incomming clients
func Listen(port int, host string) (*BroadcastServer, error) {
	app := &BroadcastServer{protocol, port, host}

	// listen on the given protocol/port/host
	listener, err := net.Listen(app.protocol, app.Address())
	if err != nil {
		return nil, err
	}

	app.protocol = "tcp"
	app.port = port
	app.host = host
	app.listener = listener
	app.closed = false
	app.quit = make(chan struct{})
	app.events = make(chan BroadcastEvent)
	app.clients = make(map[string]BroadcastClient)
	app.commands = make(map[byte]BroadcastHandler)
	app.size = 0
	return app, nil
}

// Address will return a string representation of the full server address (i.e. host:port)
func (app *BroadcastServer) Address() string {
	return app.host + ":" + strconv.FormatInt(app.port, 10)
}

// Close will end any open network connections, issue last minute commands and flush any transient data
func (app *BroadcastServer) Close() {
	if app.closed {
		return
	}

	app.events <- BroadcastEvent{"close", "broadcast server is closing.", nil}
	app.closed = true
	for _, client := range app.clients {
		client.close()
		app.size--
	}
	app.listener.Close()
	close(app.events)
	close(app.quit)
}

// AcceptConnections will use the network listener for incomming clients in order to handle those connections
// in an async manner. This will setup routines for both reading and writing to a connected client
func (app *BroadcastServer) AcceptConnections() {
	for !app.closed {
		connection, err := app.listener.Accept()
		if err != nil {
			app.events <- BroadcastEvent{"error", "accept error", err}
			continue
		}

		// Ensure that the connection is handled appropriately
		client := app.handleConnection(connection)

		// client event routine for logging purposes
		go func(c *BroadcastClient) {
			for !c.closed {
				event := <-c.events
				msg := fmt.Sprintf("[%s] %s: %s", c.id, event.level, event.message)
				if event.err != nil {
					msg += fmt.Sprintf(" %v", event.err)
				}

				log.Println(msg)
			}
		}(client)
	}
}

// handleConnection will create several routines for handling a new network connection to the broadcast server.
// This method will create a simple client, spawn both write and read routines where appropriate, handle
// disconnects, and finalize the client connection when the server is disposing
func (app *BroadcastServer) handleConnection(conn net.Conn) *BroadcastClient {
	client := &BroadcastClient{}
	client.id = connection.RemoteAddr().String()
	client.reader = bufio.NewReader(conn)
	client.closed = false
	client.events = make(chan BroadcastEvent)
	client.cmdFnErr = make(chan error)

	app.clients[client.id] = client
	app.size++

	go app.runClient(client)
	return client
}

// runClient will begin reading from the client connection until the client has either disconnected, a read error has occured
// or anything else that might defer the closing of this routine.
func (app *BroadcastServer) runClient(client *BroadcastClient) {
	// defer panics into a loggable recovery state
	defer func() {
		if e := recover(); e != nil {
			buf = make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[0:n]
			client.events <- &BroadcastEvent{"fatal", "client run panic", err, buf}
		}

		client.close()
	}()

	// start reading from the connection until this client is officially closed
	for !client.closed {
		cmd, data, err := client.Read(app.commands)
		if err != nil {
			client.events <- &BroadcastEvent{"error", "read error", err}
			client.Close()
			return
		}

		// execute the command safely, respond to any error events appropriately
		err := app.runCommand(cmd, data, client)
		if err != nil {
			client.events <- &BroadcastEvent{"error", "command error", err}
		}
	}
}

// runCommand will execute the given handler by applying the arguments data from the given client.
func (app *BroadcastServer) runCommand(cmd *BroadcastHandler, data [][]byte, client *BroadcastClient) error {
	var err error
	start := time.Now()

	// execute the command until we receive a response
	go func() {
		client.cmdFnErr <- cmd.fn(data, client, app)
	}()
	err <- client.cmdFnErr

	// log the duration of this command using the metrics handler
	duration := time.Since(start)

	return err
}
