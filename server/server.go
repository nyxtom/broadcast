package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// BroadcastServer represents a construct for the application as a whole including
// the various address, protocol, network listener, connected clients, and overall
// server state that can be used for either reporting, or communicating with services.
type BroadcastServer struct {
	protocol    string
	port        int                       // port to listen on
	host        string                    // host to bind to
	addr        string                    // address to bind to
	version     string                    // version of the broadcast server
	bit         string                    // 32-bit vs 64-bit version
	pid         int                       // pid of the broadcast server
	listener    net.Listener              // listener for the broadcast server
	clients     map[string]*NetworkClient // clients is a map of all the connected clients to the server
	commands    map[string]Handler        // commands is a map of all the available commands executable by the server
	commandHelp map[string]Command        // command help includes name, description and usage
	size        int                       // size is the number of total clients connected to the server
	backends    []Backend                 // registered backends with the broadcast server
	Closed      bool                      // closed is the boolean for when the application has already been closed
	Quit        chan struct{}             // quit is a simple channel signal for when the application quits
	Events      chan BroadcastEvent       // events is a channel for when emitted data occurs in the application
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

// Listen will use the given address parameters to construct a simple server that listens for incomming clients
func Listen(port int, host string) (*BroadcastServer, error) {
	app := new(BroadcastServer)
	app.protocol = "tcp"
	app.port = port
	app.host = host
	app.addr = host + ":" + strconv.Itoa(port)
	app.version = BroadcastVersion
	app.bit = BroadcastBit
	app.pid = os.Getpid()

	// listen on the given protocol/port/host
	listener, err := net.Listen(app.protocol, app.addr)
	if err != nil {
		return nil, err
	}

	app.listener = listener
	app.clients = make(map[string]*NetworkClient)
	app.commands = make(map[string]Handler)
	app.commandHelp = make(map[string]Command)
	app.size = 0
	app.backends = make([]Backend, 0)

	app.Closed = false
	app.Quit = make(chan struct{})
	app.Events = make(chan BroadcastEvent)
	return app, nil
}

func (app *BroadcastServer) LoadBackend(backend Backend) error {
	app.backends = append(app.backends, backend)
	return backend.Load()
}

func (app *BroadcastServer) Status() (*BroadcastServerStatus, error) {
	status := new(BroadcastServerStatus)
	status.NumGoroutines = runtime.NumGoroutine()
	status.NumCpu = runtime.NumCPU()
	status.NumCgoCall = runtime.NumCgoCall()
	status.NumClients = app.size
	status.Memory = new(runtime.MemStats)
	runtime.ReadMemStats(status.Memory)
	return status, nil
}

func (app *BroadcastServer) CmdInfo(data interface{}, client *NetworkClient) error {
	status, err := app.Status()
	if err != nil {
		return err
	}

	client.WriteJson(status)
	client.Flush()
	return nil
}

func (app *BroadcastServer) CmdHelp(data interface{}, client *NetworkClient) error {
	client.WriteJson(app.commandHelp)
	client.Flush()
	return nil
}

// RegisterCommand takes a simple command structure and handler to assign both the help info and the handler itself
func (app *BroadcastServer) RegisterCommand(cmd Command, handler Handler) {
	app.Register(cmd.Name, handler)
	app.commandHelp[strings.ToUpper(cmd.Name)] = cmd
}

// Register will bind a particular byte/mark to a specific command handler (thus registering command handlers)
func (app *BroadcastServer) Register(cmd string, handler Handler) {
	app.commands[strings.ToUpper(cmd)] = handler
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
		app.size--
	}
	for _, backend := range app.backends {
		backend.Unload()
	}
	app.listener.Close()
	close(app.Quit)
}

// AcceptConnections will use the network listener for incomming clients in order to handle those connections
// in an async manner. This will setup routines for both reading and writing to a connected client
func (app *BroadcastServer) AcceptConnections() {
	app.Events <- BroadcastEvent{"info", fmt.Sprintf(LogoHeader, app.version, app.bit, app.port, app.pid), nil, nil}
	app.Events <- BroadcastEvent{"info", "broadcast server started listening on " + app.Address(), nil, nil}

	for !app.Closed {
		connection, err := app.listener.Accept()
		if err != nil {
			app.Events <- BroadcastEvent{"error", "accept error", err, nil}
			continue
		}

		// Ensure that the connection is handled appropriately
		client, err := app.handleConnection(connection)
		if err != nil {
			connection.Close()
			app.size--
			app.Events <- BroadcastEvent{"error", "accept error", err, nil}
			continue
		}

		//app.Events <- BroadcastEvent{"accept", fmt.Sprintf("client %s connected to server", client.addr), nil, nil}
		go func() {
			<-client.Quit
			//app.Events <- BroadcastEvent{"disconnect", fmt.Sprintf("client %s disconnected from server", client.addr), nil, nil}
			delete(app.clients, client.addr)
			app.size--
		}()
		go app.runClient(client)
	}
}

// handleConnection will create several routines for handling a new network connection to the broadcast server.
// This method will create a simple client, spawn both write and read routines where appropriate, handle
// disconnects, and finalize the client connection when the server is disposing
func (app *BroadcastServer) handleConnection(conn net.Conn) (*NetworkClient, error) {
	client, err := NewNetworkClient(conn)
	if err != nil {
		return nil, err
	}

	app.clients[client.addr] = client
	app.size++
	return client, nil
}

// Run will begin reading from the buffer reader until the client has either disconnected
// or any other panic routine has occured as a result of this routine, when we receive
// data, the client's data handler function should accomodate callback routines.
func (app *BroadcastServer) runClient(client *NetworkClient) {
	// defer panics to the loggable event routine
	defer func() {
		if e := recover(); e != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[0:n]
			app.Events <- BroadcastEvent{"fatal", "client run panic", errors.New(fmt.Sprintf("%v", e)), buf}
		}

		client.Close()
		return
	}()

	for !client.closed {
		data, err := client.Read()
		if err != nil {
			if err != io.EOF {
				app.Events <- BroadcastEvent{"error", "read error", err, nil}
			}
			client.Close()
			return
		}

		err = app.handle(data, client)
		if err != nil {
			app.Events <- BroadcastEvent{"error", "command error", err, nil}
		}
	}
}

func (app *BroadcastServer) handle(data interface{}, client *NetworkClient) error {
	switch data := data.(type) {
	case []interface{}:
		{
			cmd := ""
			args := make([]interface{}, 0)
			if len(data) > 0 {
				cmd = strings.ToUpper(data[0].(string))
				args = data[1:]
			}

			if cmd == CMDQUIT {
				client.WriteString(OK)
				client.Flush()
				client.Close()
				return nil
			}

			handler, ok := app.commands[cmd]
			if !ok {
				client.WriteError(errCmdNotFound)
				client.Flush()
				return errCmdNotFound
			}

			err := handler(args, client)
			if err != nil {
				client.WriteError(err)
				client.Flush()
				return err
			}
		}
	}
	return nil
}
