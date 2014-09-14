package bgraphProtocol

import (
	"bytes"
	"fmt"
	"net"
	"strings"

	"github.com/nyxtom/broadcast/server"
)

type BGraphProtocolClient struct {
	server.NetworkClient
}

func NewBGraphProtocolClient(conn *net.TCPConn) (*BGraphProtocolClient, error) {
	return NewBGraphProtocolClientSize(conn, 128)
}

func NewBGraphProtocolClientSize(conn *net.TCPConn, bufferSize int) (*BGraphProtocolClient, error) {
	client := new(BGraphProtocolClient)
	client.Initialize(conn, bufferSize)
	return client, nil
}

func (proto *BGraphProtocolClient) readBulk() ([][]byte, error) {
	line, err := proto.ReadLine()
	fmt.Println(string(line))
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	/*
		if line[0] != packetLengthByte {
			return nil, errReadRequest
		}

			n, err := proto.ParseInt64(line[1:])
			if err == nil {
				buffer := make([]byte, n)
				_, err := io.ReadFull(proto.Reader, buffer)
				if err != nil {
					return nil, err
				}

				line, err := proto.ReadLine()
				if err != nil {
					return nil, err
				} else if len(line) != 0 {
					return nil, errBadBulkFormat
				}

	*/
	return bytes.Split(line, splitBulkDelim), nil
	//	}

	//	return nil, err
}

func (client *BGraphProtocolClient) WriteCommand(cmd string, args []interface{}) error {
	// $packetlength\r\n
	// cmd arg1 arg2 arg3..etc\r\n
	buffer := bytes.NewBuffer(nil)
	buffer.WriteString(strings.ToUpper(cmd))
	for _, v := range args {
		var buf bytes.Buffer
		fmt.Fprint(&buf, v)
		buffer.Write(splitBulkDelim)
		buffer.Write(buf.Bytes())
	}
	//client.WriteLen(packetLengthByte, buffer.Len())
	client.Writer.Write(buffer.Bytes())
	client.Writer.Write(lineDelims)
	client.Flush()
	return nil
}
