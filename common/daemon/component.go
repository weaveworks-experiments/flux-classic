package daemon

// A component is simply something that can be stopped
type Component interface {
	Stop()
}

// A StartFunc starts a component
type StartFunc func(sink ErrorSink) Component
