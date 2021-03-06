package main

import (
	"errors"
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
	"github.com/nyxtom/broadcast-graph"
	"github.com/nyxtom/broadcast/backends/bdefault"
	"github.com/nyxtom/broadcast/backends/pubsub"
	"github.com/nyxtom/broadcast/backends/stats"
	"github.com/nyxtom/broadcast/protocols/line"
	"github.com/nyxtom/broadcast/protocols/redis"
	"github.com/nyxtom/broadcast/server"
)

type Configuration struct {
	port            int           // port of the server
	host            string        // host of the server
	bprotocol       string        // broadcast protocol configuration
	backend_default BackendConfig // bdefault backend configuration
	backend_stats   BackendConfig // stats backend configuration
	backend_pubsub  BackendConfig // pubsub backend configuration
	backend_bgraph  BackendConfig // bgraph backend configuration
}

type BackendConfig struct {
	enabled bool // enabled setting for backend config
}

func main() {
	// Leverage all cores available
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Parse out flag parameters
	var host = flag.String("h", "127.0.0.1", "Broadcast server host to bind to")
	var port = flag.Int("p", 7331, "Broadcast server port to bind to")
	var bprotocol = flag.String("bprotocol", "redis", "Broadcast protocol configuration")
	var backend_default = flag.Bool("backend_default", true, "Broadcast default backend enabled")
	var backend_stats = flag.Bool("backend_stats", false, "Broadcast stats backend enabled setting")
	var backend_pubsub = flag.Bool("backend_pubsub", false, "Broadcast pubsub backend enabled setting")
	var backend_bgraph = flag.Bool("backend_bgraph", false, "Broadcast graph backend enabled setting")
	var configFile = flag.String("config", "", "Broadcast server configuration file (/etc/broadcast.conf)")
	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	cfg := &Configuration{*port, *host, *bprotocol, BackendConfig{*backend_default}, BackendConfig{*backend_stats}, BackendConfig{*backend_pubsub}, BackendConfig{*backend_bgraph}}
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

	// locate the protocol specified (if there is one)
	var serverProtocol server.BroadcastServerProtocol
	if cfg.bprotocol == "" {
		serverProtocol = server.NewDefaultBroadcastServerProtocol()
	} else if cfg.bprotocol == "redis" {
		serverProtocol = redisProtocol.NewRedisProtocol()
	} else if cfg.bprotocol == "line" {
		serverProtocol = lineProtocol.NewLineProtocol()
	} else {
		fmt.Println(errors.New("Invalid protocol " + cfg.bprotocol + " specified"))
		return
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
	app, err := server.ListenProtocol(cfg.port, cfg.host, serverProtocol)
	if err != nil {
		fmt.Println(err)
		return
	}

	// load the default backend should it be enabled
	if cfg.backend_default.enabled {
		backend, err := bdefault.RegisterBackend(app)
		if err != nil {
			fmt.Println(err)
			return
		}
		app.LoadBackend(backend)
	}

	// load the stats backend should it be enabled
	if cfg.backend_stats.enabled {
		backend, err := stats.RegisterBackend(app)
		if err != nil {
			fmt.Println(err)
			return
		}
		app.LoadBackend(backend)
	}

	// load the pubsub backend should it be enabled
	if cfg.backend_pubsub.enabled {
		backend, err := pubsub.RegisterBackend(app)
		if err != nil {
			fmt.Println(err)
			return
		}
		app.LoadBackend(backend)
	}

	// load the bgraph backend should it be enabled
	if cfg.backend_bgraph.enabled {
		backend, err := bgraph.RegisterBackend(app)
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

	// accept incomming connections!
	app.AcceptConnections()
}
