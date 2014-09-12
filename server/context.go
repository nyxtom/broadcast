package server

import (
	"runtime"
	"strings"
)

type BroadcastContext struct {
	Commands    map[string]Handler  // commands is a map of all the available commands executable by the server
	CommandHelp map[string]Command  // command help includes name, description and usage
	Events      chan BroadcastEvent // events for the context of the broadcast server
	ClientSize  int                 // number of connected clients
}

// RegisterCommand takes a simple command structure and handler to assign both the help info and the handler itself
func (ctx *BroadcastContext) RegisterCommand(cmd Command, handler Handler) {
	ctx.Register(cmd.Name, handler)
	ctx.CommandHelp[strings.ToUpper(cmd.Name)] = cmd
}

// Register will bind a particular byte/mark to a specific command handler (thus registering command handlers)
func (ctx *BroadcastContext) Register(cmd string, handler Handler) {
	ctx.Commands[strings.ToUpper(cmd)] = handler
}

// RegisterHelp will only register that the command exists in some form (without a handler which may be processed another way)
func (ctx *BroadcastContext) RegisterHelp(cmd Command) {
	ctx.CommandHelp[strings.ToUpper(cmd.Name)] = cmd
}

func (ctx *BroadcastContext) Help() (map[string]Command, error) {
	return ctx.CommandHelp, nil
}

// Status will return the current status of this system
func (ctx *BroadcastContext) Status() (*BroadcastServerStatus, error) {
	status := new(BroadcastServerStatus)
	status.NumGoroutines = runtime.NumGoroutine()
	status.NumCpu = runtime.NumCPU()
	status.NumCgoCall = runtime.NumCgoCall()
	status.NumClients = ctx.ClientSize
	status.Memory = new(runtime.MemStats)
	runtime.ReadMemStats(status.Memory)
	return status, nil
}

func NewBroadcastContext() *BroadcastContext {
	ctx := new(BroadcastContext)
	ctx.Commands = make(map[string]Handler)
	ctx.CommandHelp = make(map[string]Command)
	ctx.Events = make(chan BroadcastEvent)
	return ctx
}
