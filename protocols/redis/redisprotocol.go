package protocols

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"

	"github.com/nyxtom/broadcast/server"
)

var errCmdNotFound = errors.New("invalid command format")
var errQuit = errors.New("client quit")

type RedisProtocol struct {
	ctx *server.BroadcastContext
}

func (p *RedisProtocol) Initialize(ctx *server.BroadcastContext) error {
	p.ctx = ctx
	return nil
}

func (p *RedisProtocol) HandleConnection(conn net.Conn) (server.ProtocolClient, error) {
	return NewRedisProtocolClient(conn), nil
}

func (p *RedisProtocol) RunClient(client server.ProtocolClient) {
	c, ok := client.(*RedisProtocolClient)
	if !ok {
		return
	}

	// defer panics to the loggable event routine
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

	for !c.Closed {
		data, err := c.readBulk()
		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- server.BroadcastEvent{"error", "read error", err, nil}
			}
			c.Close()
			return
		}

		err = p.handleData(data, c)
		if err != nil {
			if err == errQuit {
				c.WriteString("OK")
				c.Flush()
				c.Close()
				return
			} else {
				p.ctx.Events <- server.BroadcastEvent{"error", "accept error", err, nil}
				c.WriteError(err)
				c.Flush()
			}
		}
	}
}

func (p *RedisProtocol) cmdHelp(client *RedisProtocolClient) error {
	help, _ := p.ctx.Help()
	client.WriteJson(help)
	client.Flush()
	return nil
}

func (p *RedisProtocol) cmdInfo(client *RedisProtocolClient) error {
	status, _ := p.ctx.Status()
	client.WriteJson(status)
	client.Flush()
	return nil
}

func (p *RedisProtocol) handleData(data [][]byte, client *RedisProtocolClient) error {
	switch {
	case bytes.Equal(data[0], []byte("QUIT")):
		return errQuit
	case bytes.Equal(data[0], []byte("CMDS")):
		return p.cmdHelp(client)
	case bytes.Equal(data[0], []byte("INFO")):
		return p.cmdInfo(client)
	default:
		return errCmdNotFound
	}
}
