package server

import "errors"

// error constants associated with the broadcast server
var errLineFormat = errors.New("bad response line format")
var errReadRequest = errors.New("invalid request protocol")
var errBadBulkFormat = errors.New("bad bulk string format")
var errCmdNotFound = errors.New("invalid command format")

var Delims = []byte("\r\n")
var NullBulk = []byte("-1")
var CMDQUIT = "QUIT"
var PONG = "PONG"
var OK = "OK"
var BroadcastVersion = "0.1.0"
var BroadcastBit = "64 bit"
var LogoHeader = `

  _                             Broadcast %s %s
   /_)__  _   _/_  _   __/_     
   /_)//_//_|/_//_ /_|_\ /      Port: %d
                                PID: %d

`
