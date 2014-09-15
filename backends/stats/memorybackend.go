package stats

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var empty struct{}

type MemoryBackend struct {
	sync.Mutex

	counters          map[string]*Counter
	values            map[string]int64
	sets              map[string]map[string]struct{}
	setLock           sync.Mutex
	maxCounterHistory int
	lastTimeStamp     time.Time
}

type Counter struct {
	Value int64 // total for the given counter
	Rate  *CounterRate
}

type CounterRate struct {
	RatePerSecond float64   // avg rate per second
	AvgHistory    []float64 // avg rate per second history
}

func NewMemoryBackend() (*MemoryBackend, error) {
	mem := new(MemoryBackend)
	mem.counters = make(map[string]*Counter)
	mem.values = make(map[string]int64)
	mem.sets = make(map[string]map[string]struct{})
	mem.maxCounterHistory = 100
	mem.lastTimeStamp = time.Now()
	return mem, nil
}

func (mem *MemoryBackend) FlushCounters() error {
	mem.Lock()
	defer mem.Unlock()
	timePrev := mem.lastTimeStamp
	mem.lastTimeStamp = time.Now()
	d := mem.lastTimeStamp.Sub(timePrev).Seconds()
	for _, v := range mem.counters {
		value := v.Value
		ratePrev := v.Rate.RatePerSecond
		v.Rate.RatePerSecond = float64(value) / d
		v.Rate.AvgHistory = append(v.Rate.AvgHistory, ratePrev)
		l := len(v.Rate.AvgHistory)
		if l > mem.maxCounterHistory {
			v.Rate.AvgHistory = v.Rate.AvgHistory[l-mem.maxCounterHistory:]
		}
		v.Value = 0
	}
	return nil
}

func (mem *MemoryBackend) Counters() (map[string]*Counter, error) {
	mem.Lock()
	defer mem.Unlock()
	return mem.counters, nil
}

func (mem *MemoryBackend) Counter(name string) (int64, error) {
	return mem.CounterBy(name, 1)
}

func (mem *MemoryBackend) CounterBy(name string, count int64) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	if v, ok := mem.counters[name]; ok {
		v.Value += count
		return v.Value, nil
	} else {
		mem.counters[name] = &Counter{count, &CounterRate{0, make([]float64, 0)}}
		return count, nil
	}
}

func (mem *MemoryBackend) Incr(name string) (int64, error) {
	return mem.IncrBy(name, 1)
}

func (mem *MemoryBackend) IncrBy(name string, count int64) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	if v, ok := mem.values[name]; ok {
		v = v + count
		mem.values[name] = v
		return v, nil
	} else {
		mem.values[name] = count
		return count, nil
	}
}

func (mem *MemoryBackend) Decr(name string) (int64, error) {
	return mem.DecrBy(name, 1)
}

func (mem *MemoryBackend) DecrBy(name string, count int64) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	if v, ok := mem.values[name]; ok {
		v = v - count
		mem.values[name] = v
		return v, nil
	} else {
		mem.values[name] = -1 * count
		return count, nil
	}
}

func (mem *MemoryBackend) Del(name string) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	deleted := 0
	if _, ok := mem.values[name]; ok {
		delete(mem.values, name)
		deleted++
	}
	if _, ok := mem.counters[name]; ok {
		delete(mem.counters, name)
		deleted++
	}

	return int64(deleted), nil
}

func (mem *MemoryBackend) Exists(name string) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	_, ok := mem.values[name]
	if ok {
		return 1, nil
	} else {
		return 0, nil
	}
}

var ErrNotFound = errors.New("key not found")

func (mem *MemoryBackend) Get(name string) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	v, ok := mem.values[name]
	if ok {
		return v, nil
	} else {
		return 0, ErrNotFound
	}
}

func (mem *MemoryBackend) Set(name string, value int64) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	mem.values[name] = value
	return 1, nil
}

func (mem *MemoryBackend) SetNx(name string, value int64) (int64, error) {
	mem.Lock()
	defer mem.Unlock()
	if _, ok := mem.values[name]; !ok {
		mem.values[name] = value
		return 1, nil
	} else {
		return -1, nil
	}
}

func (mem *MemoryBackend) SAdd(name string, value string) (int64, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	set, ok := mem.sets[name]
	if !ok {
		set = make(map[string]struct{})
		mem.sets[name] = set
	}

	_, ok = set[value]
	if ok {
		return 0, nil
	} else {
		set[value] = empty
		return 1, nil
	}
}

func (mem *MemoryBackend) SRem(name string, value string) (int64, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	set, ok := mem.sets[name]
	if !ok {
		return -1, nil
	}

	_, ok = set[value]
	if ok {
		delete(set, value)
		return 1, nil
	} else {
		return 0, nil
	}
}

func (mem *MemoryBackend) SCard(name string) (int64, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	set, ok := mem.sets[name]
	if ok {
		return int64(len(set)), nil
	} else {
		return 0, nil
	}
}

func (mem *MemoryBackend) SMembers(name string) (map[string]struct{}, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	set, ok := mem.sets[name]
	if ok {
		return set, nil
	} else {
		return nil, nil
	}
}

func (mem *MemoryBackend) SDiff(resultSet map[string]struct{}, name string) (map[string]struct{}, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	set, ok := mem.sets[name]
	if !ok {
		return resultSet, nil
	} else {
		// resultSet: a, b, c
		// set: a
		// output: -> b, c

		// resultSet: a, b
		// set: a, c, k
		// output: -> b
		results := make(map[string]struct{})
		for k, _ := range resultSet {
			if _, ok = set[k]; !ok {
				results[k] = empty
			}
		}

		return results, nil
	}
}

func (mem *MemoryBackend) SIsMember(name string, value string) (int64, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	set, ok := mem.sets[name]
	if ok {
		_, ok = set[value]
		if ok {
			return 1, nil
		}
	}

	return 0, nil
}

func (mem *MemoryBackend) SUnion(names []string) (map[string]struct{}, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	results := make(map[string]struct{})
	for _, v := range names {
		if result, ok := mem.sets[v]; ok {
			for k, _ := range result {
				results[k] = empty
			}
		}
	}

	return results, nil
}

func (mem *MemoryBackend) SInter(names []string) (map[string]struct{}, error) {
	mem.setLock.Lock()
	defer mem.setLock.Unlock()

	values := make([]map[string]struct{}, len(names))
	minimalSetIndex := 0
	for i, v := range names {
		result, ok := mem.sets[v]
		if !ok {
			return nil, nil
		} else {
			values[i] = result

			if len(result) < len(values[minimalSetIndex]) {
				minimalSetIndex = i
			}
		}
	}

	minimalSet := values[minimalSetIndex]
	results := make(map[string]struct{})
	for k, _ := range minimalSet {
		value := true
		for i, v := range values {
			if i == minimalSetIndex {
				continue
			}

			if _, ok := v[k]; !ok {
				value = false
				break
			}
		}

		if value {
			results[k] = empty
		}
	}

	return results, nil
}

func (mem *MemoryBackend) Keys(pattern string) ([]string, error) {
	mem.Lock()
	defer mem.Unlock()

	if pattern == "" || pattern == "*" {
		results := make([]string, len(mem.values))
		iter := 0
		for k, _ := range mem.values {
			results[iter] = k
		}
		return results, nil
	} else {
		patterns := strings.Split(pattern, "*")
		results := make([]string, 0)
		if len(patterns) == 1 {
			for k, _ := range mem.values {
				if k == pattern {
					results = append(results, k)
				}
			}
		} else {
			for k, _ := range mem.values {
				found := true
				iter := 0
				s := k
				for iter < len(patterns) {
					if strings.HasPrefix(s, patterns[iter]) {
						s = strings.TrimLeft(k, patterns[iter])
						iter++
						if iter < len(patterns) {
							index := strings.Index(k, patterns[iter])
							iter++
							if index >= 0 {
								s = k[index:]
							} else {
								found = false
								break
							}
						} else {
							break
						}
					} else {
						found = false
						break
					}
				}

				if found {
					results = append(results, k)
				}
			}
		}

		return results, nil
	}
}
