package redisProtocol

import (
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"

	"github.com/nyxtom/broadcast/server"
)

var errCmdNotFound = errors.New("invalid command format")
var errQuit = errors.New("client quit")

type RedisProtocol struct {
	ctx *server.BroadcastContext
}

func NewRedisProtocol() *RedisProtocol {
	return new(RedisProtocol)
}

func (p *RedisProtocol) Initialize(ctx *server.BroadcastContext) error {
	p.ctx = ctx
	return nil
}

func (p *RedisProtocol) Name() string {
	return "redis"
}

func (p *RedisProtocol) HandleConnection(conn *net.TCPConn) (server.ProtocolClient, error) {
	return NewRedisProtocolClientSize(conn, 128)
}

func (p *RedisProtocol) RunClient(client server.ProtocolClient) {
	// defer panics to the loggable event routine
	defer func() {
		if e := recover(); e != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			buf = buf[0:n]
			p.ctx.Events <- server.BroadcastEvent{"fatal", "client run panic", errors.New(fmt.Sprintf("%v", e)), buf}
		}

		client.Close()
		return
	}()

	reqErr := client.RequestErrorChan()
	for {
		data, err := client.ReadBulkPayload()
		if err != nil {
			if err != io.EOF {
				p.ctx.Events <- server.BroadcastEvent{"error", "read error", err, nil}
			}
			return
		}

		err = p.handleData(data, client, reqErr)
		if err != nil {
			if err == errQuit {
				client.WriteString("OK")
				client.Flush()
				return
			} else {
				p.ctx.Events <- server.BroadcastEvent{"error", "accept error", err, nil}
				client.WriteError(err)
				client.Flush()
			}
		}
	}
}

func (p *RedisProtocol) handleData(data [][]byte, client server.ProtocolClient, reqErr chan error) error {
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
