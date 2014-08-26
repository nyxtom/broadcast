package server

var pong = "PONG"

func CmdPing(data interface{}, client *NetworkClient) error {
	client.WriteString(pong)
	client.Flush()
	return nil
}
