package server

// Handler is the actual function declaration that is provided argument data, client, and server
type Handler func(interface{}, *NetworkClient) error
