package stats

import (
	"errors"
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

	SAdd(name string, value string) (int64, error)
	SRem(name string, value string) (int64, error)
	SCard(name string) (int64, error)
	SMembers(name string) (map[string]struct{}, error)

	Keys(pattern string) ([]string, error)
}

type StatsBackend struct {
	server.Backend

	quit  chan struct{}
	timer *time.Ticker
	mem   Metrics
}

func (stats *StatsBackend) FlushInt(i int64, err error, client server.ProtocolClient) error {
	if err != nil {
		return err
	}
	client.WriteInt64(i)
	client.Flush()
	return nil
}

func (stats *StatsBackend) FlushNil(client server.ProtocolClient) error {
	client.WriteNull()
	client.Flush()
	return nil
}

func (stats *StatsBackend) readString(d []byte) (string, error) {
	return string(d), nil
}

func (stats *StatsBackend) readInt64(d []byte) (int64, error) {
	return strconv.ParseInt(string(d), 10, 64)
}

func (stats *StatsBackend) readStringInt64(d [][]byte) (string, int64, error) {
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

func (stats *StatsBackend) Set(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) SetNx(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Get(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Exists(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Del(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Incr(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Decr(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Count(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
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

func (stats *StatsBackend) Counters(data interface{}, client server.ProtocolClient) error {
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

func (stats *StatsBackend) Keys(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	key := ""
	if len(d) > 0 {
		pattern, err := stats.readString(d[0])
		if err != nil {
			return err
		}
		key = pattern
	}

	results, err := stats.mem.Keys(key)
	if err != nil {
		client.WriteError(err)
		client.Flush()
		return nil
	}
	client.WriteLen('*', len(results))
	for _, k := range results {
		client.WriteString(k)
	}
	client.Flush()
	return nil
}

func (stats *StatsBackend) SAdd(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	key := ""
	if len(d) < 2 {
		client.WriteError(errors.New("SADD takes at least 2 parameters (SADD key member)"))
		client.Flush()
		return nil
	} else {
		key = string(d[0])
		members := d[1:]
		result := int64(0)
		for _, v := range members {
			r, err := stats.mem.SAdd(key, string(v))
			if err != nil {
				return err
			} else {
				result += r
			}
		}

		return stats.FlushInt(result, nil, client)
	}
}

func (stats *StatsBackend) SRem(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	key := ""
	if len(d) < 2 {
		client.WriteError(errors.New("SREM takes at least 2 parameters (SREM key member)"))
		client.Flush()
		return nil
	} else {
		key = string(d[0])
		members := d[1:]
		result := int64(0)
		for _, v := range members {
			r, err := stats.mem.SRem(key, string(v))
			if err != nil {
				return err
			} else if r == -1 {
				return stats.FlushInt(r, nil, client)
			} else {
				result += r
			}
		}

		return stats.FlushInt(result, nil, client)
	}
}

func (stats *StatsBackend) SCard(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) < 1 {
		client.WriteError(errors.New("SCARD takes at least 1 parameter (SCARD key)"))
		client.Flush()
		return nil
	} else {
		results := make([]int64, len(d))
		for i, v := range d {
			r, err := stats.mem.SCard(string(v))
			if err != nil {
				return err
			} else {
				results[i] = r
			}
		}

		client.WriteLen('*', len(d))
		for _, v := range results {
			client.WriteInt64(v)
		}
		client.Flush()
		return nil
	}
}

func (stats *StatsBackend) SMembers(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) == 0 {
		client.WriteError(errors.New("SMEMBERS takes 1 parameter (SMEMBERS key)"))
		client.Flush()
		return nil
	} else {
		values, err := stats.mem.SMembers(string(d[0]))
		if err != nil {
			return err
		}

		if values == nil {
			client.WriteNull()
			client.Flush()
			return nil
		}

		client.WriteLen('*', len(values))
		for k, _ := range values {
			client.WriteString(k)
		}
		client.Flush()
		return nil
	}
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
	app.RegisterCommand(server.Command{"KEYS", "Returns the list of keys available or by pattern", "KEYS [pattern]", false}, backend.Keys)
	app.RegisterCommand(server.Command{"SADD", "Adds one or more members to a set", "SADD key member [member ...]", false}, backend.SAdd)
	app.RegisterCommand(server.Command{"SREM", "Removes one or more members from a set", "SREM key member [member ...]", false}, backend.SRem)
	app.RegisterCommand(server.Command{"SCARD", "Gets the number of members from a set", "SCARD key [key ...]", false}, backend.SCard)
	app.RegisterCommand(server.Command{"SMEMBERS", "Gets all the members in a set", "SMEMBERS key", false}, backend.SMembers)
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
