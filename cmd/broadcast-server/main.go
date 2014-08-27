package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/nyxtom/broadcast/backends/stats"
	"github.com/nyxtom/broadcast/server"
)

type Configuration struct {
	port  int    // port of the server
	host  string // host of the server
	stats bool   // stats backend configuration enabled/disabled
}

func main() {
	// Leverage all cores available
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Parse out flag parameters
	var host = flag.String("h", "127.0.0.1", "Broadcast server host to bind to")
	var port = flag.Int("p", 7331, "Broadcast server port to bind to")
	var statsBackend = flag.Bool("stats", true, "Broadcast server stats backend")
	var configFile = flag.String("config", "", "Broadcast server configuration file (/etc/broadcast.conf)")
	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	cfg := &Configuration{*port, *host, *statsBackend}
	if len(*configFile) == 0 {
		fmt.Printf("[%d] %s # WARNING: no config file specified, using the default config\n", os.Getpid(), time.Now().Format(time.RFC822))
	} else {
		data, err := ioutil.ReadFile(*configFile)
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = toml.Decode(string(data), cfg)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			fmt.Println(err)
			return
		}
		pprof.StartCPUProfile(f)
	}

	// create a new broadcast server
	app, err := server.Listen(cfg.port, cfg.host)
	if err != nil {
		fmt.Println(err)
		return
	}

	// setup backends
	if cfg.stats {
		backend, err := stats.RegisterBackend(app)
		if err != nil {
			fmt.Println(err)
			return
		}
		app.LoadBackend(backend)
	}

	// wait for all events to fire so we can log them
	pid := os.Getpid()
	go func() {
		for !app.Closed {
			event := <-app.Events
			t := time.Now()
			delim := "#"
			if event.Level == "error" {
				delim = "ERROR:"
			}
			msg := fmt.Sprintf("[%d] %s %s %s", pid, t.Format(time.RFC822), delim, event.Message)
			if event.Err != nil {
				msg += fmt.Sprintf(" %v", event.Err)
			}

			fmt.Println(msg)
		}
	}()

	go func() {
		<-app.Quit
		pprof.StopCPUProfile()
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
	app.RegisterCommand(server.Command{"PING", "Pings the server for a response", "", false}, server.CmdPing)
	app.RegisterCommand(server.Command{"ECHO", "Echos back a message sent", "ECHO \"hello world\"", false}, server.CmdEcho)
	app.RegisterCommand(server.Command{"INFO", "Current server status and information", "", false}, app.CmdInfo)
	app.RegisterCommand(server.Command{"CMDS", "List of available commands supported by the server", "", false}, app.CmdHelp)

	// accept incomming connections!
	app.AcceptConnections()
}
