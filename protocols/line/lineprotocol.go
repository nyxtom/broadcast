package lineProtocol

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"

	"github.com/nyxtom/broadcast/server"
)

type LineProtocol struct {
	ctx *server.BroadcastContext
}

func NewLineProtocol() *LineProtocol {
	return new(LineProtocol)
}

func (p *LineProtocol) Initialize(ctx *server.BroadcastContext) error {
	p.ctx = ctx
	return nil
}

func (p *LineProtocol) Name() string {
	return "line"
}

func (p *LineProtocol) HandleConnection(conn *net.TCPConn) (server.ProtocolClient, error) {
	return NewLineProtocolClientSize(conn, 128)
}

func (p *LineProtocol) RunClient(client server.ProtocolClient) {
	c, ok := client.(*LineProtocolClient)
	if !ok {
		client.WriteError(errInvalidProtocol)
		client.Close()
		return
	}

	defer func() {
		if e := recover(); e != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[0:n]
			p.ctx.Events <- server.BroadcastEvent{"fatal", "client run panic", errors.New(fmt.Sprintf("%v", e)), buf}
		}

		c.Close()
		return
	}()

	reqErr := client.RequestErrorChan()
	for {
		data, err := c.readBulk()

		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- server.BroadcastEvent{"error", "read error", err, nil}
			}
			return
		}

		err = p.handleData(data, c, reqErr)
		if err != nil {
			if err == errQuit {
				return
			} else {
				p.ctx.Events <- server.BroadcastEvent{"error", "accept error", err, nil}
				c.WriteError(err)
				c.Flush()
			}
		}
	}
}

func (p *LineProtocol) handleData(data [][]byte, client *LineProtocolClient, reqErr chan error) error {
	cmd := strings.ToUpper(string(data[0]))
	switch {
	case cmd == "QUIT":
		return errQuit
	default:
		handler, ok := p.ctx.Commands[cmd]
		if !ok {
			return errCmdNotFound
		}

		var err error
		go func() {
			reqErr <- handler(data[1:], client)
		}()
		err = <-reqErr
		return err
	}
}
