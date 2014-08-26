package broadcast

import (
	"container/list"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/nyxtom/broadcast/server"
)

// BroadcastServerCli represents a construct for connecting to a broadcast server
type Client struct {
	sync.Mutex

	protocol    string
	port        int
	host        string
	addr        string
	maxIdle     int
	connections *list.List
}

func NewClient(port int, host string, maxIdle int) (*Client, error) {
	client := new(Client)
	client.protocol = "tcp"
	client.port = port
	client.host = host
	client.addr = host + ":" + strconv.Itoa(port)
	client.connections = list.New()
	client.maxIdle = maxIdle
	return client, nil
}

func (client *Client) Do(cmd string, args ...interface{}) (interface{}, error) {
	c := client.get()
	reply, err := c.Do(cmd, args...)
	client.put(c)
	return reply, err
}

func (client *Client) Close() {
	client.Lock()
	defer client.Unlock()
	for client.connections.Len() > 0 {
		e := client.connections.Front()
		c := e.Value.(*ClientConnection)
		client.connections.Remove(e)
		c.finalize()
	}
}

func (client *Client) CloseConnection(conn *ClientConnection) {
	client.put(conn)
}

func (client *Client) Get() *ClientConnection {
	return client.get()
}

func (client *Client) get() *ClientConnection {
	client.Lock()
	defer client.Unlock()
	if client.connections.Len() == 0 {
		c := new(ClientConnection)
		c.addr = client.addr
		c.protocol = client.protocol
		return c
	} else {
		e := client.connections.Front()
		c := e.Value.(*ClientConnection)
		client.connections.Remove(e)
		return c
	}
}

func (client *Client) put(c *ClientConnection) {
	client.Lock()
	defer client.Unlock()
	if client.connections.Len() >= client.maxIdle {
		c.finalize()
	} else {
		c.lastActive = time.Now()
		client.connections.PushFront(c)
	}
}

type ClientConnection struct {
	sync.Mutex

	protocol   string
	addr       string
	netClient  *server.NetworkClient
	lastActive time.Time
}

func (c *ClientConnection) Do(cmd string, args ...interface{}) (interface{}, error) {
	// ensure that we are connected
	if err := c.connect(); err != nil {
		return nil, err
	}

	// execute/write the appropriate command
	if err := c.netClient.WriteCommand(cmd, args); err != nil {
		c.finalize()
		return nil, err
	}

	// flush the command out to the server itself
	if err := c.netClient.Flush(); err != nil {
		c.finalize()
		return nil, err
	}

	// read the reply from the server
	if reply, err := c.netClient.Read(); err != nil {
		c.finalize()
		return nil, err
	} else {
		return reply, nil
	}
}

func (c *ClientConnection) connect() error {
	if c.netClient != nil {
		return nil
	}

	conn, err := net.Dial(c.protocol, c.addr)
	if err != nil {
		return err
	}

	c.netClient, err = server.NewNetworkClient(conn)
	if err != nil {
		return err
	}

	return nil
}

func (c *ClientConnection) finalize() {
	c.Lock()
	defer c.Unlock()
	if c.netClient != nil {
		c.netClient.Close()
		c.netClient = nil
	}
}
