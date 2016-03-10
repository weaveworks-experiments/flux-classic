package daemon

type ErrorSink chan error

func NewErrorSink() ErrorSink {
	return ErrorSink(make(chan error, 1))
}

// Post an error.  Posting a nil error is a no-op, for convenience.
func (sink ErrorSink) Post(err error) {
	if err != nil {
		select {
		case sink <- err:
		default:
		}
	}
}
