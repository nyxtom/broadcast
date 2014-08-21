package main

import (
	"flag"
	"runtime"
)

func main() {
	// Leverage all cores available
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Parse out flag parameters
	var port = flag.String("port", "7818", "Broadcast server port to bind to")
	var mgmtPort = flag.String("mgmt", "7819", "Broacast management server port address")

}
