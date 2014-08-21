package server

// handlerFunc is the actual function declaration that is provided argument data, client, and server
type handlerFunc func([][]byte, *BroadcastClient, *BroadcastServer) error

// BroadcastHandler constructs the requirements and execution around a specific command in the server
type BroadcastHandler struct {
	length int         // length is the number of arguments that the handler requires from a request
	name   string      // name of the broadcast handler itself
	fn     handlerFunc // fn is the callback handler that executes on the given data
}
