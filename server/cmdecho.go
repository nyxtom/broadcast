package server

func CmdEcho(data interface{}, client *NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteString("")
		client.Flush()
		return nil
	} else {
		client.WriteString(d[0].(string))
		client.Flush()
		return nil
	}
}
