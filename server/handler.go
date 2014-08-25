package server

// Handler is the actual function declaration that is provided argument data, client, and server
type Handler func(interface{}, *NetworkClient) error

// Command describes a command handler with name, description, usage
type Command struct {
	Name        string // name of the command
	Description string // description of the command
	Usage       string // example usage of the command
}
