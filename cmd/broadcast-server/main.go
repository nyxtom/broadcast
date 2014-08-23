package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/nyxtom/broadcast/server"
)

func main() {
	// Leverage all cores available
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()

	// Parse out flag parameters
	var host = flag.String("h", "127.0.0.1", "Broadcast server host to bind to")
	var port = flag.Int("p", 7331, "Broadcast server port to bind to")

	// create a new broadcast server
	app, err := server.Listen(*port, *host)
	if err != nil {
		fmt.Println(err)
		return
	}

	// wait for all events to fire so we can log them
	address := app.Address()
	go func() {
		for !app.Closed {
			event := <-app.Events
			msg := fmt.Sprintf("[%s] %s: %s", address, event.Level, event.Message)
			if event.Err != nil {
				msg += fmt.Sprintf(" %v", event.Err)
			}

			fmt.Println(msg)
		}
	}()

	go func() {
		<-app.Quit
		os.Exit(0)
	}()

	// attach to any signals that would cause our app to close
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt)

	go func() {
		<-sc
		app.Close()
	}()

	// register all relevant commands
	app.Register("PING", func(data interface{}, client *server.NetworkClient) error {
		client.WriteString("PONG")
		client.Flush()
		return nil
	})
	app.Register("ECHO", func(data interface{}, client *server.NetworkClient) error {
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
	})
	app.Register("ADD", func(data interface{}, client *server.NetworkClient) error {
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
	})

	// accept incomming connections!
	app.AcceptConnections()
}
