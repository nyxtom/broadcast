package lineProtocol

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/nyxtom/broadcast/server"
)

type LineProtocolClient struct {
	server.NetworkClient
}

func NewLineProtocolClient(conn *net.TCPConn) (*LineProtocolClient, error) {
	return NewLineProtocolClientSize(conn, 128)
}

func NewLineProtocolClientSize(conn *net.TCPConn, bufferSize int) (*LineProtocolClient, error) {
	client := new(LineProtocolClient)
	client.Initialize(conn, bufferSize)
	return client, nil
}

func (proto *LineProtocolClient) readBulk() ([][]byte, error) {
	line, err := proto.ReadLineInvariant()
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	return bytes.Split(line, splitBulkDelim), nil
}

func (client *LineProtocolClient) WriteCommand(cmd string, args []interface{}) error {
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(strings.ToUpper(cmd))
	for _, v := range args {
		var buf bytes.Buffer
		fmt.Fprint(&buf, v)
		buffer.Write(splitBulkDelim)
		buffer.Write(buf.Bytes())
	}
	client.Writer.Write(buffer.Bytes())
	client.Writer.Write(lineDelims)
	client.Flush()
	return nil
}
