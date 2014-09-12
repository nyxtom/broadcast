package redisProtocol

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/nyxtom/broadcast/server"
)

type RedisProtocolClient struct {
	server.NetworkClient
}

func NewRedisProtocolClient(conn net.Conn) (*RedisProtocolClient, error) {
	c, err := NewRedisProtocolClientSize(conn, 128)
	return c, err
}

func NewRedisProtocolClientSize(conn net.Conn, bufferSize int) (*RedisProtocolClient, error) {
	client := new(RedisProtocolClient)
	client.Initialize(conn, bufferSize)
	return client, nil
}

func (client *RedisProtocolClient) WriteCommand(cmd string, args []interface{}) error {
	err := client.WriteLen('*', len(args)+1)
	client.WriteBytes([]byte(strings.ToUpper(cmd)))
	for _, v := range args {
		var buf bytes.Buffer
		fmt.Fprint(&buf, v)
		client.WriteBytes(buf.Bytes())
	}
	client.Flush()
	return err
}
