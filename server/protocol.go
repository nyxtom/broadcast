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
	HandleConnection(conn net.Conn) (*NetworkClient, error)
	RunClient(client *NetworkClient)
	Initialize(ctx *BroadcastContext) error
}

type DefaultBroadcastServerProtocol struct {
	ctx    *BroadcastContext
	dataIn chan interface{}
}

func NewDefaultBroadcastServerProtocol() *DefaultBroadcastServerProtocol {
	return new(DefaultBroadcastServerProtocol)
}

func (p *DefaultBroadcastServerProtocol) Initialize(ctx *BroadcastContext) error {
	p.ctx = ctx
	return nil
}

// HandleConnection will create several routines for handling a new network connection to the broadcast server.
// This method will create a simple client, spawn both write and read routines where appropriate, handle
// disconnects, and finalize the client connection when the server is disposing
func (p *DefaultBroadcastServerProtocol) HandleConnection(conn net.Conn) (*NetworkClient, error) {
	return NewNetworkClient(conn)
}

// Run will begin reading from the buffer reader until the client has either disconnected
// or any other panic routine has occured as a result of this routine, when we receive
// data, the client's data handler function should accomodate callback routines.
func (p *DefaultBroadcastServerProtocol) RunClient(client *NetworkClient) {
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

	for !client.closed {
		data, err := client.ReadInterface()
		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- BroadcastEvent{"error", "read error", err, nil}
			}
			client.Close()
			return
		}

		err = p.handleData(data, client)
		if err != nil {
			if err == errQuit {
				client.WriteString(OK)
				client.Flush()
				client.Close()
			} else {
				p.ctx.Events <- BroadcastEvent{"error", "accept error", err, nil}
				client.WriteError(err)
				client.Flush()
			}
		}
	}
}

func (p *DefaultBroadcastServerProtocol) handleData(data interface{}, client *NetworkClient) error {
	switch data := data.(type) {
	case []interface{}:
		{
			cmd := ""
			args := make([]interface{}, 0)
			if len(data) > 0 {
				if b, ok := data[0].([]uint8); ok {
					cmd = strings.ToUpper(string(b))
				} else {
					cmd = strings.ToUpper(data[0].(string))
				}
				args = data[1:]
			}

			if cmd == CMDQUIT {
				return errQuit
			}

			handler, ok := p.ctx.Commands[cmd]
			if !ok {
				return errCmdNotFound
			}

			return handler(args, client)
		}
	}

	return nil
}
