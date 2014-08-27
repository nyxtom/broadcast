package stats

import (
	"errors"
	"fmt"
	"time"

	"github.com/nyxtom/broadcast/server"
)

type Metrics interface {
	Counter(name string) (int, error)
	CounterBy(name string, count int) (int, error)
	FlushCounters() error

	Counters() (map[string]*Counter, error)

	Incr(name string) (int, error)
	IncrBy(name string, count int) (int, error)

	Decr(name string) (int, error)
	DecrBy(name string, count int) (int, error)

	Del(name string) (int, error)

	Exists(name string) (int, error)

	Get(name string) (int, error)

	Set(name string, value int) (int, error)
	SetNx(name string, value int) (int, error)
}

type StatsBackend struct {
	server.Backend

	quit  chan struct{}
	timer *time.Ticker
	mem   Metrics
}

func (stats *StatsBackend) FlushInt(i int, err error, client *server.NetworkClient) error {
	if err != nil {
		return err
	}
	client.WriteInt64(int64(i))
	client.Flush()
	return nil
}

func (stats *StatsBackend) Set(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) < 2 {
		client.WriteError(errors.New("SET takes at least 2 parameters (i.e. key to set and value to set to)"))
		client.Flush()
		return nil
	} else {
		key := d[0].(string)
		value := d[1].(int64)
		_, err := stats.mem.Set(key, int(value))
		return err
	}
}

func (stats *StatsBackend) SetNx(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) < 2 {
		client.WriteError(errors.New("SETNX takes at least 2 parameters (i.e. key to set and value to set to, if not already set)"))
		client.Flush()
		return nil
	} else {
		key := d[0].(string)
		value := d[1].(int64)
		_, err := stats.mem.SetNx(key, int(value))
		return err
	}
}

func (stats *StatsBackend) Get(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("GET takes at least 1 parameter (i.e. key to get)"))
		client.Flush()
		return nil
	} else {
		var key string
		switch d[0].(type) {
		case []byte:
			key = string(d[0].([]byte))
			fmt.Println(key)
			fmt.Println(d[0].([]byte))
		case string:
			key = d[0].(string)
		default:
			key = fmt.Sprintf("%v", d[0])
		}
		i, err := stats.mem.Get(key)
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
		key := fmt.Sprintf("%v", d[0])
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
		key := d[0].(string)
		_, err := stats.mem.Del(key)
		return err
	}
}

func (stats *StatsBackend) Incr(data interface{}, client *server.NetworkClient) error {
	d, _ := data.([]interface{})
	if len(d) == 0 {
		client.WriteError(errors.New("INCR takes at least 1 parameter (i.e. key to increment)"))
		client.Flush()
		return nil
	} else {
		key := d[0].(string)
		values := d[1:]
		if len(values) > 0 {
			value := int(values[0].(int64))
			_, err := stats.mem.IncrBy(key, value)
			return err
		} else {
			_, err := stats.mem.Incr(key)
			return err
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
		key := d[0].(string)
		values := d[1:]
		if len(values) > 0 {
			value := int(values[0].(int64))
			_, err := stats.mem.DecrBy(key, value)
			return err
		} else {
			_, err := stats.mem.Decr(key)
			return err
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
		key := d[0].(string)
		values := d[1:]
		if len(values) > 0 {
			value := int(values[0].(int64))
			_, err := stats.mem.CounterBy(key, value)
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

	commandHelp := []server.Command{
		server.Command{"COUNT", "Increments a key that resets itself to 0 on each flush routine.", "COUNT foo [124]", true},
		server.Command{"COUNTERS", "Returns the list of active counters.", "", false},
		server.Command{"INCR", "Increments a key by the specified value or by default 1.", "INCR key [1]", true},
		server.Command{"DECR", "Decrements a key by the specified value or by default 1.", "DECR key [1]", true},
		server.Command{"DEL", "Deletes a key from the values or counters list or both.", "DEL key", true},
		server.Command{"EXISTS", "Determines if the given key exists from the values.", "EXISTS key", false},
		server.Command{"GET", "Gets the specified key from the values.", "GET key", false},
		server.Command{"SET", "Sets the specified key to the specified value in values.", "SET key 1234", true},
		server.Command{"SETNX", "Sets the specified key to the given value only if the key is not already set.", "SETNX key 1234", true},
	}
	commands := []server.Handler{
		backend.Count,
		backend.Counters,
		backend.Incr,
		backend.Decr,
		backend.Del,
		backend.Exists,
		backend.Get,
		backend.Set,
		backend.SetNx,
	}

	for i, _ := range commandHelp {
		app.RegisterCommand(commandHelp[i], commands[i])
	}

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
