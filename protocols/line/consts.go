package lineProtocol

import "errors"

var errReadRequest = errors.New("invalid request format")
var errBadBulkFormat = errors.New("bad bulk string format")
var errLineFormat = errors.New("bad response line format")
var errInvalidProtocol = errors.New("invalid protocol")
var errCmdNotFound = errors.New("invalid command format")
var errQuit = errors.New("client quit")
var splitBulkDelim = []byte(" ")
var packetLengthByte = byte('$')
var lineDelims = []byte("\r\n")
