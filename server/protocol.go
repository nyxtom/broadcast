package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
)

type BroadcastServerProtocol interface {
	HandleConnection(conn *net.TCPConn) (ProtocolClient, error)
	RunClient(client ProtocolClient)
	Initialize(ctx *BroadcastContext) error
	Name() string
}

type DefaultBroadcastServerProtocol struct {
	ctx *BroadcastContext
}

func NewDefaultBroadcastServerProtocol() *DefaultBroadcastServerProtocol {
	return new(DefaultBroadcastServerProtocol)
}

func (p *DefaultBroadcastServerProtocol) Name() string {
	return "interface"
}

func (p *DefaultBroadcastServerProtocol) Initialize(ctx *BroadcastContext) error {
	p.ctx = ctx
	return nil
}

// HandleConnection will create several routines for handling a new network connection to the broadcast server.
// This method will create a simple client, spawn both write and read routines where appropriate, handle
// disconnects, and finalize the client connection when the server is disposing
func (p *DefaultBroadcastServerProtocol) HandleConnection(conn *net.TCPConn) (ProtocolClient, error) {
	return NewNetworkClient(conn)
}

// Run will begin reading from the buffer reader until the client has either disconnected
// or any other panic routine has occured as a result of this routine, when we receive
// data, the client's data handler function should accomodate callback routines.
func (p *DefaultBroadcastServerProtocol) RunClient(client ProtocolClient) {
	// defer panics to the loggable event routine
	defer func() {
		if e := recover(); e != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[0:n]
			p.ctx.Events <- BroadcastEvent{"fatal", "client run panic", errors.New(fmt.Sprintf("%v", e)), buf}
		}

		client.Close()
		return
	}()

	for {
		data, err := client.ReadInterface()
		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- BroadcastEvent{"error", "read error", err, nil}
			}
			return
		}

		err = p.handleData(data, client)
		if err != nil {
			if err == errQuit {
				client.WriteString("OK")
				client.Flush()
				return
			} else {
				p.ctx.Events <- BroadcastEvent{"error", "accept error", err, nil}
				client.WriteError(err)
				client.Flush()
			}
		}
	}
}

func (p *DefaultBroadcastServerProtocol) handleData(data interface{}, client ProtocolClient) error {
	switch data := data.(type) {
	case []interface{}:
		{
			cmd := ""
			if len(data) > 0 {
				if b, ok := data[0].([]uint8); ok {
					cmd = strings.ToUpper(string(b))
				} else {
					cmd = strings.ToUpper(data[0].(string))
				}
			}

			switch cmd {
			case "QUIT":
				return errQuit
			default:
				handler, ok := p.ctx.Commands[cmd]
				if !ok {
					return errCmdNotFound
				}

				return handler(data[1:], client)
			}
		}
	}

	return nil
}
