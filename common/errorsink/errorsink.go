package errorsink

type ErrorSink chan error

func New() ErrorSink {
	return ErrorSink(make(chan error, 1))
}

func (sink ErrorSink) Post(err error) {
	select {
	case sink <- err:
	default:
	}
}
