package protocols

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"

	"github.com/nyxtom/broadcast/server"
)

var errReadRequest = errors.New("invalid request format")
var errBadBulkFormat = errors.New("bad bulk string format")
var errLineFormat = errors.New("bad response line format")
var packetLengthByte = byte('$')
var argLengthByte = byte('*')
var delims = []byte("\r\n")

// RedisProtocol is the network client protocol implementation
// that conforms to the redis packet standards. This will
// implement the necessary read handlers, buffer allocation and
// ultimately the callbacks to which all data is transmitted
type RedisProtocolClient struct {
	server.NetworkClient
}

func NewRedisProtocolClient(conn net.Conn) *RedisProtocolClient {
	return NewRedisProtocolClientSize(conn, 128)
}

func NewRedisProtocolClientSize(conn net.Conn, bufferSize int) *RedisProtocolClient {
	client := new(RedisProtocolClient)
	client.Conn = conn
	client.Reader = bufio.NewReaderSize(conn, bufferSize)
	client.Writer = bufio.NewWriterSize(conn, bufferSize)
	client.Addr = conn.RemoteAddr().String()
	client.Quit = make(chan struct{})
	return client
}

// WriteBulk will take an array of byte arrays to write out
// specific variable length bulk data. The first packet of the
// payload will specify the length of the arguments, the next N
// argument packets will be a combination of the length of each
// argument's byte array, followed by the bytes themself.
func (proto *RedisProtocolClient) WriteBulk(data [][]byte) error {
	proto.Writer.WriteByte(argLengthByte)
	proto.Writer.Write(strconv.AppendInt(nil, int64(len(data)), 10))
	proto.Writer.Write(delims)
	for _, v := range data {
		proto.Writer.WriteByte(packetLengthByte)
		proto.Writer.Write(strconv.AppendInt(nil, int64(len(v)), 10))
		proto.Writer.Write(delims)
		proto.Writer.Write(v)
		proto.Writer.Write(delims)
	}
	return nil
}

// parseInt64 is a common conversion from []byte to int64
func parseInt64(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return strconv.ParseInt(string(b), 10, 64)
}

// readPacket will look for the packetLength byte at the beginning of a
// given line to determine how many of the next n bytes need to be read
func (proto *RedisProtocolClient) readPacket() ([]byte, error) {
	line, err := proto.readLine()
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	if line[0] != packetLengthByte {
		return nil, errReadRequest
	}

	n, err := parseInt64(line[1:])
	if err != nil {
		return nil, err
	} else if n == -1 {
		return nil, nil
	} else {
		buffer := make([]byte, n)
		_, err := io.ReadFull(proto.Reader, buffer)
		if err != nil {
			return nil, err
		}

		line, err := proto.readLine()
		if err != nil {
			return nil, err
		} else if len(line) != 0 {
			return nil, errBadBulkFormat
		}

		return buffer, nil
	}

	return nil, errReadRequest
}

// Read ensures that we are overwriting the default read behavior
// and returning an array of byte arrays used in the redis protocol
// we should always see an argument length byte followed by bulk data
func (proto *RedisProtocolClient) readBulk() ([][]byte, error) {
	line, err := proto.readLine()
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	switch line[0] {
	case argLengthByte:
		{
			n, err := parseInt64(line[1:])
			if err != nil {
				return nil, err
			}

			r := make([][]byte, n)
			for i := range r {
				r[i], err = proto.readPacket()
				if err != nil {
					return nil, err
				}
			}

			return r, nil
		}
	}

	return nil, errReadRequest
}

// readLine will look for packets in the read buffer that are delimited
// by a \r\n. All payloads are transmitted in redis using the following:
// *arglength
// $bytelength
// bytes
// $bytelength
// bytes...etc
func (proto *RedisProtocolClient) readLine() ([]byte, error) {
	packet, err := proto.Reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}

	i := len(packet) - 2
	if i < 0 || packet[i] != '\r' {
		return nil, errLineFormat
	}

	return packet[:i], nil
}
