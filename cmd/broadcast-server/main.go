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
	"github.com/nyxtom/broadcast/backends/bdefault"
	"github.com/nyxtom/broadcast/backends/stats"
	"github.com/nyxtom/broadcast/protocols/redis"
	"github.com/nyxtom/broadcast/server"
)

type Configuration struct {
	port             int           // port of the server
	host             string        // host of the server
	bprotocol        string        // broadcast protocol configuration
	backends_default BackendConfig // bdefault backend configuration
	backends_stats   BackendConfig // stats backend configuration
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
	var backends_default = flag.Bool("backends_default", true, "Broadcast default backend enabled")
	var backends_stats = flag.Bool("backends_stats", false, "Broadcast stats backend enabled setting")
	var configFile = flag.String("config", "", "Broadcast server configuration file (/etc/broadcast.conf)")
	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")

	flag.Parse()

	cfg := &Configuration{*port, *host, *bprotocol, BackendConfig{*backends_default}, BackendConfig{*backends_stats}}
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
	if cfg.backends_default.enabled {
		backend, err := bdefault.RegisterBackend(app)
		if err != nil {
			fmt.Println(err)
			return
		}
		app.LoadBackend(backend)
	}

	// load the stats backend should it be enabled
	if cfg.backends_stats.enabled {
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

	// accept incomming connections!
	app.AcceptConnections()
}
