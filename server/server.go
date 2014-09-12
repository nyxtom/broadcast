package server

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strconv"
)

// BroadcastServer represents a construct for the application as a whole including
// the various address, protocol, network listener, connected clients, and overall
// server state that can be used for either reporting, or communicating with services.
type BroadcastServer struct {
	port     int                       // port to listen on
	host     string                    // host to bind to
	addr     string                    // address to bind to
	bit      string                    // 32-bit vs 64-bit version
	pid      int                       // pid of the broadcast server
	listener net.Listener              // listener for the broadcast server
	clients  map[string]ProtocolClient // clients is a map of all the connected clients to the server
	ctx      *BroadcastContext
	backends []Backend               // registered backends with the broadcast server
	protocol BroadcastServerProtocol // server protocol for handling connections
	Closed   bool                    // closed is the boolean for when the application has already been closed
	Quit     chan struct{}           // quit is a simple channel signal for when the application quits
	Events   chan BroadcastEvent     // events is a channel for when emitted data occurs in the application
	Name     string                  // canonical name of the broadcast server
	Version  string                  // version of the broadcast server
	Header   string                  // header for the broadcast server
}

type BroadcastServerStatus struct {
	NumGoroutines int               // number of go-routines running
	NumCpu        int               // number of cpu's running
	NumCgoCall    int64             // number of cgo calls
	Memory        *runtime.MemStats // memory statistics running
	NumClients    int               // number of connected clients
}

type Backend interface {
	Load() error
	Unload() error
}

// Listen will use the given address parameters to construct a simple server that listens for incoming clients
func Listen(port int, host string) (*BroadcastServer, error) {
	return ListenProtocol(port, host, NewDefaultBroadcastServerProtocol())
}

// ListenProtocol uses the address parameters and the specified protocol to construct the broadcast server
func ListenProtocol(port int, host string, protocol BroadcastServerProtocol) (*BroadcastServer, error) {
	app := new(BroadcastServer)
	app.port = port
	app.host = host
	app.addr = host + ":" + strconv.Itoa(port)
	app.bit = BroadcastBit
	app.pid = os.Getpid()

	// listen on the given protocol/port/host
	listener, err := net.Listen("tcp", app.addr)
	if err != nil {
		return nil, err
	}

	app.listener = listener
	app.ctx = NewBroadcastContext()
	app.clients = make(map[string]ProtocolClient)
	app.backends = make([]Backend, 0)
	app.protocol = protocol

	app.Closed = false
	app.Quit = make(chan struct{})
	app.Events = app.ctx.Events
	app.Name = "Broadcast"

	app.Version = BroadcastVersion
	app.Header = LogoHeader
	return app, nil
}

// Load will load the backend service
func (app *BroadcastServer) LoadBackend(backend Backend) error {
	app.backends = append(app.backends, backend)
	return backend.Load()
}

// Status will return the current state of the system and process
func (app *BroadcastServer) Status() (*BroadcastServerStatus, error) {
	return app.ctx.Status()
}

// Help will output the current context help commands
func (app *BroadcastServer) Help() (map[string]Command, error) {
	return app.ctx.CommandHelp, nil
}

// RegisterCommand takes a simple command structure and handler to assign both the help info and the handler itself
func (app *BroadcastServer) RegisterCommand(cmd Command, handler Handler) {
	app.ctx.RegisterCommand(cmd, handler)
}

// Register will bind a particular byte/mark to a specific command handler (thus registering command handlers)
func (app *BroadcastServer) Register(cmd string, handler Handler) {
	app.ctx.Register(cmd, handler)
}

// RegisterHelp will only register that the command exists in some form (without a handler which may be processed another way)
func (app *BroadcastServer) RegisterHelp(cmd Command) {
	app.ctx.RegisterHelp(cmd)
}

// Address will return a string representation of the full server address (i.e. host:port)
func (app *BroadcastServer) Address() string {
	return app.addr
}

// Close will end any open network connections, issue last minute commands and flush any transient data
func (app *BroadcastServer) Close() {
	if app.Closed {
		return
	}

	app.Events <- BroadcastEvent{"close", "broadcast server is closing.", nil, nil}
	app.Closed = true
	for _, client := range app.clients {
		client.Close()
		app.ctx.ClientSize--
	}
	for _, backend := range app.backends {
		backend.Unload()
	}
	app.listener.Close()
	close(app.Quit)
}

// AcceptConnections will use the network listener for incoming clients in order to handle those connections
// in an async manner. This will setup routines for both reading and writing to a connected client
func (app *BroadcastServer) AcceptConnections() {
	app.Events <- BroadcastEvent{"info", fmt.Sprintf(app.Header, app.Name, app.Version, app.bit, app.port, app.pid), nil, nil}
	app.Events <- BroadcastEvent{"info", "listening for incoming connections on " + app.Address(), nil, nil}

	err := app.protocol.Initialize(app.ctx)
	if err != nil {
		app.Events <- BroadcastEvent{"error", "accept error", err, nil}
		return
	}

	// accept connections, handle them via the protocol and run them
	for !app.Closed {
		connection, err := app.listener.Accept()
		if err != nil {
			app.Events <- BroadcastEvent{"error", "accept error", err, nil}
			continue
		}

		// Ensure that the connection is handled appropriately
		client, err := app.protocol.HandleConnection(connection)
		if err != nil {
			connection.Close()
			app.Events <- BroadcastEvent{"error", "accept error", err, nil}
			continue
		}

		app.clients[client.Address()] = client
		app.ctx.ClientSize++

		go func() {
			<-client.WaitExit()
			delete(app.clients, client.Address())
			app.ctx.ClientSize--
		}()

		go app.protocol.RunClient(client)
	}
}
