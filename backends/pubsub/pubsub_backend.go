package pubsub

import (
	"sync"

	"github.com/nyxtom/broadcast/server"
)

type PubSubBackend struct {
	server.Backend

	app    *server.BroadcastServer
	topics map[string]*TopicChannel
}

var empty struct{}

type TopicChannel struct {
	sync.Mutex

	size    int
	clients map[string]struct{}
}

// subscribe will add the given protocol client to the channel to subscribe to
func (b *PubSubBackend) subscribe(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) < 1 {
		return nil
	} else {
		for _, k := range d {
			key := string(k)
			if topic, ok := b.topics[key]; ok {
				topic.Lock()
				id := client.Address()
				if _, ok = topic.clients[id]; !ok {
					topic.clients[id] = empty
					topic.size++
				}
				topic.Unlock()
			} else {
				topic := new(TopicChannel)
				topic.clients = make(map[string]struct{})
				topic.size = 1
				topic.clients[client.Address()] = empty
				b.topics[key] = topic
			}
		}

		return nil
	}
}

func (b *PubSubBackend) unsubscribe(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) < 1 {
		return nil
	} else {
		for _, k := range d {
			key := string(k)
			if topic, ok := b.topics[key]; ok {
				topic.Lock()
				id := client.Address()
				if _, ok = topic.clients[id]; ok {
					delete(topic.clients, id)
					topic.size--
				}
				topic.Unlock()
			}
		}

		return nil
	}
}

// publish will process messages and send them should the channel exist and be subscribed to
func (b *PubSubBackend) publish(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) < 2 {
		return nil
	} else {
		key := string(d[0])
		message := d[1:]

		if topic, ok := b.topics[key]; ok {
			topic.Lock()
			defer topic.Unlock()
			if topic.size > 0 {
				deletions := make([]string, 0)
				for c, _ := range topic.clients {
					if sClient, ok := b.app.GetClient(c); ok {
						go func() {
							sClient.WriteBulk(message)
							sClient.Flush()
						}()
					} else {
						deletions = append(deletions, c)
					}
				}

				// remove any stragglers
				for _, c := range deletions {
					delete(topic.clients, c)
				}
			}
		}

		return nil
	}
}

func RegisterBackend(app *server.BroadcastServer) (server.Backend, error) {
	backend := new(PubSubBackend)
	app.RegisterCommand(server.Command{"PUBLISH", "Publishes to a specified topic given the data/arguments", "PUBLISH topic message", true}, backend.publish)
	app.RegisterCommand(server.Command{"SUBSCRIBE", "Subscribes to a specified topic", "SUBSCRIBE topic [topic ...]", true}, backend.subscribe)
	app.RegisterCommand(server.Command{"UNSUBSCRIBE", "Unsubscribes from a specified topic", "UNSUBSCRIBE topic [topic ...]", true}, backend.unsubscribe)
	backend.app = app
	backend.topics = make(map[string]*TopicChannel)
	return backend, nil
}

func (b *PubSubBackend) Load() error {
	return nil
}

func (b *PubSubBackend) Unload() error {
	return nil
}
