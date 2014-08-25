package server

var pong = "PONG"

func CmdPing(data interface{}, client *NetworkClient) error {
	client.WriteJson(pong)
	client.Flush()
	return nil
}
