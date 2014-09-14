package bgraphProtocol

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"

	"github.com/nyxtom/broadcast/server"
)

type BGraphProtocol struct {
	ctx *server.BroadcastContext
}

func NewBGraphProtocol() *BGraphProtocol {
	return new(BGraphProtocol)
}

func (p *BGraphProtocol) Initialize(ctx *server.BroadcastContext) error {
	p.ctx = ctx
	return nil
}

func (p *BGraphProtocol) HandleConnection(conn *net.TCPConn) (server.ProtocolClient, error) {
	return NewBGraphProtocolClientSize(conn, 128)
}

func (p *BGraphProtocol) RunClient(client server.ProtocolClient) {
	c, ok := client.(*BGraphProtocolClient)
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

	for {
		data, err := c.readBulk()

		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- server.BroadcastEvent{"error", "read error", err, nil}
			}
			return
		}

		err = p.handleData(data, c)
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

func (p *BGraphProtocol) handleData(data [][]byte, client *BGraphProtocolClient) error {
	cmd := strings.ToUpper(string(data[0]))
	switch {
	case cmd == "QUIT":
		return errQuit
	default:
		handler, ok := p.ctx.Commands[cmd]
		if !ok {
			return errCmdNotFound
		}

		return handler(data[1:], client)
	}
}
