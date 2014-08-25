package server

import "errors"

func CmdSum(data interface{}, client *NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) < 1 {
		client.WriteError(errors.New("ADD takes at least 2 parameters"))
		client.Flush()
		return nil
	} else {
		sum := int64(0)
		sumf := float64(0)
		for _, a := range d {
			switch a := a.(type) {
			case int64:
				sum += a
			case float64:
				sumf += a
			}
		}
		if sumf != 0 {
			client.WriteFloat64(sumf + float64(sum))
		} else {
			client.WriteInt64(sum)
		}
		client.Flush()
		return nil
	}
}
