package fatal

type Sink chan error

func New() Sink {
	return Sink(make(chan error, 1))
}

func (sink Sink) Post(err error) {
	select {
	case sink <- err:
	default:
	}
}
