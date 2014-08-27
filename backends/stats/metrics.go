package stats

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/nyxtom/broadcast/server"
)

type Metrics interface {
	Counter(name string) (int64, error)
	CounterBy(name string, count int64) (int64, error)
	FlushCounters() error

	Counters() (map[string]*Counter, error)

	Incr(name string) (int64, error)
	IncrBy(name string, count int64) (int64, error)

	Decr(name string) (int64, error)
	DecrBy(name string, count int64) (int64, error)

	Del(name string) (int64, error)

	Exists(name string) (int64, error)

	Get(name string) (int64, error)

	Set(name string, value int64) (int64, error)
	SetNx(name string, value int64) (int64, error)
}

type StatsBackend struct {
	server.Backend

	quit  chan struct{}
	timer *time.Ticker
	mem   Metrics
}

func (stats *StatsBackend) FlushInt(i int64, err error, client *server.NetworkClient) error {
	if err != nil {
		return err
	}
	client.WriteInt64(i)
	client.Flush()
	return nil
}

func (stats *StatsBackend) FlushNil(client *server.NetworkClient) error {
	client.WriteNull()
	client.Flush()
	return nil
}

func (stats *StatsBackend) readString(d interface{}) (string, error) {
	switch d := d.(type) {
	case []byte:
		return string(d), nil
	case string:
		return d, nil
	default:
		return fmt.Sprintf("%v", d), nil
	}
}

func (stats *StatsBackend) readInt64(d interface{}) (int64, error) {
	switch d := d.(type) {
	case []byte:
		return strconv.ParseInt(string(d), 10, 64)
	case int64:
		return d, nil
	case int32:
		return int64(d), nil
	case int16:
		return int64(d), nil
	case byte:
		return int64(d), nil
	case string:
		return strconv.ParseInt(d, 10, 64)
	}

	return 0, errors.New("invalid type")
}

func (stats *StatsBackend) readStringInt64(d []interface{}) (string, int64, error) {
	key, err := stats.readString(d[0])
	if err != nil {
		return "", 0, err
	}
	value, err := stats.readInt64(d[1])
	if err != nil {
		return "", 0, err
	}

	return key, value, nil
}

func (stats *StatsBackend) Set(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) < 2 {
		client.WriteError(errors.New("SET takes at least 2 parameters (i.e. key to set and value to set to)"))
		client.Flush()
		return nil
	} else {
		key, value, err := stats.readStringInt64(d)
		if err != nil {
			return err
		}
		i, err := stats.mem.Set(key, value)
		return stats.FlushInt(i, err, client)
	}
}

func (stats *StatsBackend) SetNx(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) < 2 {
		client.WriteError(errors.New("SETNX takes at least 2 parameters (i.e. key to set and value to set to, if not already set)"))
		client.Flush()
		return nil
	} else {
		key, value, err := stats.readStringInt64(d)
		if err != nil {
			return err
		}
		i, err := stats.mem.SetNx(key, value)
		return stats.FlushInt(i, err, client)
	}
}

func (stats *StatsBackend) Get(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("GET takes at least 1 parameter (i.e. key to get)"))
		client.Flush()
		return nil
	} else {
		key, err := stats.readString(d[0])
		if err != nil {
			return err
		}
		i, err := stats.mem.Get(key)
		if err == ErrNotFound {
			return stats.FlushNil(client)
		}
		return stats.FlushInt(i, err, client)
	}
}

func (stats *StatsBackend) Exists(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("EXISTS takes at least 1 parameter (i.e. key to find)"))
		client.Flush()
		return nil
	} else {
		key, err := stats.readString(d[0])
		if err != nil {
			return err
		}
		i, err := stats.mem.Exists(key)
		return stats.FlushInt(i, err, client)
	}
}

func (stats *StatsBackend) Del(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("DEL takes at least 1 parameter (i.e. key to delete)"))
		client.Flush()
		return nil
	} else {
		i := int64(0)
		for _, k := range d {
			key, err := stats.readString(k)
			if err != nil {
				return err
			}
			i2, err := stats.mem.Del(key)
			if err != nil {
				return err
			} else {
				i += i2
			}
		}
		return stats.FlushInt(i, nil, client)
	}
}

func (stats *StatsBackend) Incr(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("INCR takes at least 1 parameter (i.e. key to increment)"))
		client.Flush()
		return nil
	} else {
		key, err := stats.readString(d[0])
		if err != nil {
			return err
		}
		values := d[1:]
		if len(values) > 0 {
			value, err := stats.readInt64(values[0])
			if err != nil {
				return err
			}
			i, err := stats.mem.IncrBy(key, value)
			return stats.FlushInt(i, err, client)
		} else {
			i, err := stats.mem.Incr(key)
			return stats.FlushInt(i, err, client)
		}
	}
}

func (stats *StatsBackend) Decr(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("DECR takes at least 1 parameter (i.e. key to increment)"))
		client.Flush()
		return nil
	} else {
		key, err := stats.readString(d[0])
		if err != nil {
			return err
		}
		values := d[1:]
		if len(values) > 0 {
			value, err := stats.readInt64(values[0])
			if err != nil {
				return err
			}
			i, err := stats.mem.DecrBy(key, value)
			return stats.FlushInt(i, err, client)
		} else {
			i, err := stats.mem.Decr(key)
			return stats.FlushInt(i, err, client)
		}
	}
}

func (stats *StatsBackend) Count(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("COUNTER takes at least 1 parameter (i.e. key to increment)"))
		client.Flush()
		return nil
	} else {
		key, err := stats.readString(d[0])
		if err != nil {
			return err
		}
		values := d[1:]
		if len(values) > 0 {
			value, err := stats.readInt64(values[0])
			if err != nil {
				return err
			}
			_, err = stats.mem.CounterBy(key, value)
			return err
		} else {
			_, err := stats.mem.Counter(key)
			return err
		}
	}
}

func (stats *StatsBackend) Counters(data interface{}, client *server.NetworkClient) error {
	results, err := stats.mem.Counters()
	if err != nil {
		client.WriteError(err)
		client.Flush()
		return nil
	}

	client.WriteJson(results)
	client.Flush()
	return nil
}

func RegisterBackend(app *server.BroadcastServer) (server.Backend, error) {
	backend := new(StatsBackend)
	mem, err := NewMemoryBackend()
	if err != nil {
		return nil, err
	}

	backend.mem = mem

	app.RegisterCommand(server.Command{"COUNT", "Increments a key that resets itself to 0 on each flush routine.", "COUNT foo [124]", true}, backend.Count)
	app.RegisterCommand(server.Command{"COUNTERS", "Returns the list of active counters.", "", false}, backend.Counters)
	app.RegisterCommand(server.Command{"INCR", "Increments a key by the specified value or by default 1.", "INCR key [1]", false}, backend.Incr)
	app.RegisterCommand(server.Command{"DECR", "Decrements a key by the specified value or by default 1.", "DECR key [1]", false}, backend.Decr)
	app.RegisterCommand(server.Command{"DEL", "Deletes a key from the values or counters list or both.", "DEL key", false}, backend.Del)
	app.RegisterCommand(server.Command{"EXISTS", "Determines if the given key exists from the values.", "EXISTS key", false}, backend.Exists)
	app.RegisterCommand(server.Command{"GET", "Gets the specified key from the values.", "GET key", false}, backend.Get)
	app.RegisterCommand(server.Command{"SET", "Sets the specified key to the specified value in values.", "SET key 1234", false}, backend.Set)
	app.RegisterCommand(server.Command{"SETNX", "Sets the specified key to the given value only if the key is not already set.", "SETNX key 1234", false}, backend.SetNx)
	return backend, nil
}

func (stats *StatsBackend) Load() error {
	stats.quit = make(chan struct{})
	stats.timer = time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-stats.timer.C:
				stats.mem.FlushCounters()
			case <-stats.quit:
				stats.timer.Stop()
				return
			}
		}
	}()
	return nil
}

func (stats *StatsBackend) Unload() error {
	close(stats.quit)
	return nil
}
