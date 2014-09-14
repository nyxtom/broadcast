package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
)

type ProtocolClient interface {
	Close()
	IsClosed() bool
	Address() string
	WaitExit() chan struct{}

	Initialize(conn *net.TCPConn, bufferSize int)
	Flush() error

	WriteLen(prefix byte, n int) error
	WriteString(s string) error
	WriteByte(b byte) error
	WriteBytes(b []byte) error
	WriteInt64(n int64) error
	WriteFloat64(n float64) error
	WriteBool(b bool) error
	WriteError(e error) error
	WriteNull() error
	WriteBulk(data [][]byte) error
	WriteInterface(arg interface{}) error
	WriteArray(args []interface{}) error
	WriteJson(arg interface{}) error
	WriteCommand(cmd string, args []interface{}) error

	ReadInterface() (interface{}, error)
	ReadLine() ([]byte, error)
	ReadLineInvariant() ([]byte, error)
	ReadPayload() ([]byte, error)
	ReadBulkPayload() ([][]byte, error)

	ParseByte(b []byte) (byte, error)
	ParseString(b []byte) (string, error)
	ParseInt64(b []byte) (int64, error)
	ParseFloat64(b []byte) (float64, error)
	ParseBool(b []byte) (bool, error)
	ParseError(b []byte) (error, error)
}

type BufferClient struct {
	sync.Mutex

	Reader *bufio.Reader
	Writer *bufio.Writer
}

type NetworkClient struct {
	BufferClient

	Addr   string        // remote address identifier
	Closed bool          // closed boolean identifier
	Conn   *net.TCPConn  // network connection associated with this client
	Quit   chan struct{} // channel for when the client exits
}

// Close will shutdown any latent network connections and clear the client out
func (netClient NetworkClient) Close() {
	netClient.Lock()
	defer netClient.Unlock()

	if netClient.Conn == nil {
		return
	}

	netClient.Closed = true
	netClient.Conn.Close()
	netClient.Conn = nil
	close(netClient.Quit)
}

func (netClient NetworkClient) IsClosed() bool {
	return netClient.Closed
}

func (netClient NetworkClient) Address() string {
	return netClient.Addr
}

func (netClient NetworkClient) WaitExit() chan struct{} {
	return netClient.Quit
}

func NewNetworkClient(conn *net.TCPConn) (*NetworkClient, error) {
	c, err := NewNetworkClientSize(conn, 128)
	return c, err
}

func NewNetworkClientSize(conn *net.TCPConn, bufferSize int) (*NetworkClient, error) {
	client := new(NetworkClient)
	client.Initialize(conn, bufferSize)
	return client, nil
}

func (client *NetworkClient) Initialize(conn *net.TCPConn, bufferSize int) {
	client.Conn = conn
	client.Reader = bufio.NewReaderSize(conn, bufferSize)
	client.Writer = bufio.NewWriterSize(conn, bufferSize)
	client.Addr = conn.RemoteAddr().String()
	client.Quit = make(chan struct{})
}

func (client *BufferClient) Flush() error {
	return client.Writer.Flush()
}

// WriteLen will write the given prefix and integer to the command line
func (client *BufferClient) WriteLen(prefix byte, n int) error {
	client.Writer.WriteByte(prefix)
	client.Writer.Write(strconv.AppendInt(nil, int64(n), 10))
	_, err := client.Writer.Write(Delims)
	return err
}

/// WriteString will write the length of the string followed by the string data
func (client *BufferClient) WriteString(s string) error {
	client.Writer.WriteByte('+')
	client.Writer.WriteString(s)
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteByte(b byte) error {
	client.Writer.WriteByte('&')
	client.Writer.WriteByte(b)
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteBytes(b []byte) error {
	client.WriteLen('$', len(b))
	client.Writer.Write(b)
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteInt64(n int64) error {
	client.Writer.WriteByte(':')
	client.Writer.Write(strconv.AppendInt(nil, n, 10))
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteFloat64(n float64) error {
	client.Writer.WriteByte('.')
	client.Writer.Write(strconv.AppendFloat(nil, n, 'g', -1, 64))
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteBool(b bool) error {
	client.Writer.WriteByte('?')
	if b {
		client.Writer.WriteByte('1')
	} else {
		client.Writer.WriteByte('0')
	}
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteError(e error) error {
	client.Writer.WriteByte('-')
	if e != nil {
		client.Writer.WriteString("ERR " + e.Error())
	} else {
		client.Writer.WriteString("ERR ")
	}
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteNull() error {
	client.Writer.WriteByte('$')
	client.Writer.Write(NullBulk)
	_, err := client.Writer.Write(Delims)
	return err
}

func (client *BufferClient) WriteBulk(data [][]byte) error {
	client.WriteLen('*', len(data))
	for _, v := range data {
		client.WriteBytes(v)
	}
	return nil
}

func (client *BufferClient) WriteInterface(arg interface{}) error {
	var err error
	switch arg := arg.(type) {
	case string:
		err = client.WriteString(arg)
	case int:
		err = client.WriteInt64(int64(arg))
	case int64:
		err = client.WriteInt64(arg)
	case float64:
		err = client.WriteFloat64(arg)
	case bool:
		err = client.WriteBool(arg)
	case byte:
		err = client.WriteByte(arg)
	case []byte:
		err = client.WriteBytes(arg)
	case nil:
		err = client.WriteNull()
	default:
		var buf bytes.Buffer
		fmt.Fprint(&buf, arg)
		err = client.WriteBytes(buf.Bytes())
	}

	return err
}

func (client *BufferClient) WriteArray(args []interface{}) error {
	err := client.WriteLen('*', len(args))
	for _, arg := range args {
		if err != nil {
			return err
		}
		err = client.WriteInterface(arg)
	}

	return err
}

func (client *BufferClient) WriteJson(arg interface{}) error {
	b, err := json.Marshal(arg)
	if err != nil {
		return err
	}

	client.Writer.WriteByte('~')
	client.Writer.WriteString("json")
	client.Writer.Write(Delims)
	return client.WriteBytes(b)
}

func (client *BufferClient) WriteCommand(cmd string, args []interface{}) error {
	argsmod := make([]interface{}, len(args)+1)
	argsmod[0] = []byte(strings.ToUpper(cmd))
	for i, v := range args {
		argsmod[i+1] = v
	}
	return client.WriteArray(argsmod)
}

// ReadPayload is a formatted read off of a buffer client where the
// payload is described by the $bytelength\r\n[...bytes...]\r\n
func (client *BufferClient) ReadPayload() ([]byte, error) {
	line, err := client.ReadLine()
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	if line[0] != '$' {
		return nil, errReadRequest
	}

	n, err := client.ParseInt64(line[1:])
	if err != nil {
		return nil, err
	} else if n == -1 {
		return nil, nil
	} else {
		buffer := make([]byte, n)
		_, err := io.ReadFull(client.Reader, buffer)
		if err != nil {
			return nil, err
		}

		line, err := client.ReadLine()
		if err != nil {
			return nil, err
		} else if len(line) != 0 {
			return nil, errBadBulkFormat
		}

		return buffer, nil
	}

	return nil, errReadRequest
}

// ReadBulkPayload is a format where the bulk arguments are described as a set
// of payload arguments where each payload is read by ReadPayload and finally
// returned as an array of byte arrays.
func (client *BufferClient) ReadBulkPayload() ([][]byte, error) {
	line, err := client.ReadLine()
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	switch line[0] {
	case '*':
		{
			n, err := client.ParseInt64(line[1:])
			if err != nil {
				return nil, err
			}

			r := make([][]byte, n)
			for i := range r {
				r[i], err = client.ReadPayload()
				if err != nil {
					return nil, err
				}
			}

			return r, nil
		}
	}

	return nil, errReadRequest
}

// ReadInterface will read the described payloads as the appropriate interpreted
// typed-syntax for which they describe and return it as an interface{}. The protocol
// is described through the above prefixed delimiters through the use of various
// interface related methods (i.e. WriteString, WriteInt64...etc). ReadInterface
// is the generic use case implemented for broadcast-clients when needing to return
// errors, integers or payloads that aren't in the pure form of bytes for easy interpretation.
func (client *BufferClient) ReadInterface() (interface{}, error) {
	// read a line off of the client as we are provided a new transmission
	line, err := client.ReadLine()
	if err != nil {
		return nil, err
	} else if len(line) < 2 {
		return nil, errReadRequest
	}

	switch line[0] {
	case '$':
		{
			n, err := client.ParseInt64(line[1:])
			if err != nil {
				return nil, err
			} else if n == -1 {
				return nil, nil
			} else {
				buffer := make([]byte, n)
				_, err := io.ReadFull(client.Reader, buffer)
				if err != nil {
					return nil, err
				}

				if line, err := client.ReadLine(); err != nil {
					return nil, err
				} else if len(line) != 0 {
					return nil, errBadBulkFormat
				}

				return buffer, nil
			}
		}
	case '*':
		{
			n, err := client.ParseInt64(line[1:])
			if err != nil {
				return nil, err
			}

			r := make([]interface{}, n)
			for i := range r {
				r[i], err = client.ReadInterface()
				if err != nil {
					return nil, err
				}
			}

			return r, nil
		}
	case '&':
		return client.ParseByte(line[1:])
	case '+':
		return client.ParseString(line[1:])
	case ':':
		return client.ParseInt64(line[1:])
	case '.':
		return client.ParseFloat64(line[1:])
	case '?':
		return client.ParseBool(line[1:])
	case '-':
		return client.ParseError(line[1:])
	case '~':
		{
			structure, err := client.ParseString(line[1:])
			if err != nil {
				return nil, err
			}

			r, err := client.ReadInterface()
			if err != nil {
				return nil, err
			}

			if structure == "json" {
				var result map[string]interface{}
				err := json.Unmarshal(r.([]byte), &result)
				if err != nil {
					return nil, err
				}

				return result, nil
			}

			return nil, errReadRequest
		}
	}
	return nil, errReadRequest
}

func (client *BufferClient) ParseByte(b []byte) (byte, error) {
	if len(b) == 0 {
		return 0, errors.New("malformed byte")
	}
	return b[0], nil
}

func (client *BufferClient) ParseString(b []byte) (string, error) {
	return string(b), nil
}

func (client *BufferClient) ParseInt64(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return strconv.ParseInt(string(b), 10, 64)
}

func (client *BufferClient) ParseFloat64(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return strconv.ParseFloat(string(b), 64)
}

func (client *BufferClient) ParseBool(b []byte) (bool, error) {
	if len(b) == 0 {
		return false, nil
	}
	return b[0] == '1', nil
}

func (client *BufferClient) ParseError(b []byte) (error, error) {
	if len(b) == 0 {
		return nil, errors.New("malformed error")
	}

	return errors.New(string(b)), nil
}

func (client *BufferClient) ReadLine() ([]byte, error) {
	packet, err := client.Reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}

	i := len(packet) - 2
	if i < 0 || packet[i] != '\r' {
		return nil, errLineFormat
	}

	return packet[:i], nil
}

func (client *BufferClient) ReadLineInvariant() ([]byte, error) {
	packet, err := client.Reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}

	i := len(packet) - 2
	if i < 0 {
		return nil, errLineFormat
	}

	if packet[i] == '\r' || packet[i] == ' ' {
		return packet[:i], nil
	} else {
		return packet[:i+1], nil
	}
}
