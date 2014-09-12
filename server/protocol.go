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
	HandleConnection(conn net.Conn) (ProtocolClient, error)
	RunClient(client ProtocolClient)
	Initialize(ctx *BroadcastContext) error
}

type DefaultBroadcastServerProtocol struct {
	ctx *BroadcastContext
}

func NewDefaultBroadcastServerProtocol() *DefaultBroadcastServerProtocol {
	return new(DefaultBroadcastServerProtocol)
}

func (p *DefaultBroadcastServerProtocol) Initialize(ctx *BroadcastContext) error {
	p.ctx = ctx
	p.ctx.RegisterHelp(Command{"PING", "Pings the server for a response", "", false})
	p.ctx.RegisterHelp(Command{"ECHO", "Echos back a message sent", "ECHO \"hello world\"", false})
	p.ctx.RegisterHelp(Command{"INFO", "Current server status and information", "", false})
	p.ctx.RegisterHelp(Command{"CMDS", "List of available commands supported by the server", "", false})
	return nil
}

// HandleConnection will create several routines for handling a new network connection to the broadcast server.
// This method will create a simple client, spawn both write and read routines where appropriate, handle
// disconnects, and finalize the client connection when the server is disposing
func (p *DefaultBroadcastServerProtocol) HandleConnection(conn net.Conn) (ProtocolClient, error) {
	return NewNetworkClient(conn)
}

// Run will begin reading from the buffer reader until the client has either disconnected
// or any other panic routine has occured as a result of this routine, when we receive
// data, the client's data handler function should accomodate callback routines.
func (p *DefaultBroadcastServerProtocol) RunClient(client ProtocolClient) {
	c, ok := client.(*NetworkClient)
	if !ok {
		return
	}

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

	for !c.Closed {
		data, err := c.ReadInterface()
		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- BroadcastEvent{"error", "read error", err, nil}
			}
			return
		}

		err = p.handleData(data, c)
		if err != nil {
			if err == errQuit {
				c.WriteString(OK)
				c.Flush()
				return
			} else {
				p.ctx.Events <- BroadcastEvent{"error", "accept error", err, nil}
				c.WriteError(err)
				c.Flush()
			}
		}
	}
}

func (p *DefaultBroadcastServerProtocol) help(client *NetworkClient) error {
	help, _ := p.ctx.Help()
	client.WriteJson(help)
	client.Flush()
	return nil
}

func (p *DefaultBroadcastServerProtocol) info(client *NetworkClient) error {
	status, _ := p.ctx.Status()
	client.WriteJson(status)
	client.Flush()
	return nil
}

func (p *DefaultBroadcastServerProtocol) ping(client *NetworkClient) error {
	client.WriteString(PONG)
	client.Flush()
	return nil
}

func (p *DefaultBroadcastServerProtocol) echo(data []interface{}, client *NetworkClient) error {
	if len(data) == 0 {
		client.WriteString("")
		client.Flush()
		return nil
	} else {
		client.WriteString(data[0].(string))
		client.Flush()
		return nil
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

			switch cmd {
			case CMDQUIT:
				return errQuit
			case CMDINFO:
				return p.info(client)
			case CMDCMDS:
				return p.help(client)
			case CMDPING:
				return p.ping(client)
			case CMDECHO:
				return p.echo(args, client)
			default:
				handler, ok := p.ctx.Commands[cmd]
				if !ok {
					return errCmdNotFound
				}

				return handler(args, client)
			}
		}
	}

	return nil
}
