package server

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
)

type BroadcastClient struct {
	conn     *net.Conn // connection associated with the client
	id       string    // id of the client, (usually the remote address associated with the client)
	reader   *bufio.Reader
	closed   bool                // closed is the boolean for when the client has already been closed
	events   chan BroadcastEvent // events is a channel for emitted logs that occurs on the client
	cmdFnErr chan error
}

// close will ensure that all channels are closed, the network connection is shutdown and the state of this client is closed
func (client *BroadcastClient) Close() {
	if client.closed {
		return
	}

	client.closed = true
	close(client.events)
	close(client.cmdFnErr)
	client.conn.Close()
}

// read will use a buffered reader to read off data from the connection of the client
func (client *BroadcastClient) Read(commands *map[byte]BroadcastHandler) (*BroadcastHandler, [][]byte, error) {
	// read a line off of the client as we are provided a new transmission
	line, err := client.readLine()
	if err != nil {
		return nil, nil, err
	} else if len(line) == 0 || line[0] != '*' {
		return nil, nil, errReadRequest
	} else if cmdHandler, ok := commands[line[1]]; !ok {
		return nil, nil, errReadRequest
	}

	// cmdHandler should provide us with the basis for the number of arguments we need to read
	request := make([][]byte, 0, cmdHandler.length)

	// Typically a transmission of a request from a client will come in the form below:
	//     $arg1-bytelength\r\n
	//     bytes....\r\n
	//     $arg2-bytelength\r\n
	//     bytes....\r\n
	for i := 0; i < argLen; i++ {
		//$byteLength\r\n
		if line, err := client.readLine(); err != nil {
			return nil, nil, err
		}

		// empty lines are of no use to us
		if len(line) == 0 {
			return nil, nil, err
		} else if line[0] == '$' {
			// handle the response data
			if n, err := strconv.Atoi(string(line[1 : len(line)-1])); err != nil {
				return nil, nil, err
			} else if n == -1 {
				request = append(req, nil)
			} else {
				// create a buffer of the given length n bytes
				// then read from the stream completely up to the given n bytes
				buffer := make([]byte, n)
				if _, err := io.ReadFull(client.reader, buffer); err != nil {
					return nil, nil, err
				}

				// once we have completed reading from the buffer length, we can
				// read to the next line appropriately (i.e. \r\n)
				if line, err := client.readLine(); err != nil {
					return nil, nil, err
				} else if len(line) != 0 {
					return nil, nil, errors.New("bad bulk string format")
				}

				// append the buffer we just read to the list of request values
				request = append(request, buffer)
			}
		} else {
			return nil, nil, errReadRequest
		}
	}

	// once we have completed reading for the number of arguments, we should be done
	return cmdHandler, request, nil
}

// readline will ensure that we are reading a specific buffer off of the reader
func (client *BroadcastClient) readLine() ([]byte, error) {
	packet, err := client.reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}

	i := len(packet) - 2
	if i < 0 || packet[i] != '\r' {
		return nil, errLineFormat
	}

	return packet[:i], nil
}
