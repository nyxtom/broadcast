package server

// BroadcastEvent represents a simple construct for when 'things' occur in the application on certain levels
type BroadcastEvent struct {
	level   string // level represents a string used for the event level or type
	message string // message associated with the event occurring
	err     error  // error potentially associated with this event
	buf     []byte // buffer of a potential runtime stack
}
