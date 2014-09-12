package bdefault

import "github.com/nyxtom/broadcast/server"

type DefaultBackend struct {
	server.Backend

	app *server.BroadcastServer
}

var PONG = "PONG"

func (b *DefaultBackend) ping(data interface{}, client server.ProtocolClient) error {
	client.WriteString(PONG)
	client.Flush()
	return nil
}

func (b *DefaultBackend) info(data interface{}, client server.ProtocolClient) error {
	status, _ := b.app.Status()
	client.WriteJson(status)
	client.Flush()
	return nil
}

func (b *DefaultBackend) help(data interface{}, client server.ProtocolClient) error {
	help, _ := b.app.Help()
	client.WriteJson(help)
	client.Flush()
	return nil
}

func (b *DefaultBackend) echo(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteString("")
		client.Flush()
		return nil
	} else {
		if len(d) == 1 {
			client.WriteInterface(d[0])
		} else {
			client.WriteArray(d)
		}
		client.Flush()
		return nil
	}
}

func RegisterBackend(app *server.BroadcastServer) (server.Backend, error) {
	backend := new(DefaultBackend)
	app.RegisterCommand(server.Command{"PING", "Pings the server for a response", "", false}, backend.ping)
	app.RegisterCommand(server.Command{"ECHO", "Echos back a message sent", "ECHO \"hello world\"", false}, backend.echo)
	app.RegisterCommand(server.Command{"INFO", "Current server status and information", "", false}, backend.info)
	app.RegisterCommand(server.Command{"CMDS", "List of available commands supported by the server", "", false}, backend.help)
	backend.app = app
	return backend, nil
}

func (b *DefaultBackend) Load() error {
	return nil
}

func (b *DefaultBackend) Unload() error {
	return nil
}
