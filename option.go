package servergroup

import (
	"time"
)

const defaultTerminationTimeout = 10 * time.Second

var _ Option = (*funcOption)(nil)

// Option controls how Group.Start behaves.
type Option interface {
	apply(*option)
}

type option struct {
	isTerminationTimeoutSet bool
	terminationTimeout      time.Duration
}

type funcOption struct {
	f func(*option)
}

func (fo *funcOption) apply(o *option) {
	fo.f(o)
}

func newFuncOption(f func(*option)) *funcOption {
	return &funcOption{
		f: f,
	}
}

func newOption(opts ...Option) *option {
	o := &option{}
	for _, opt := range opts {
		opt.apply(o)
	}

	if !o.isTerminationTimeoutSet {
		o.terminationTimeout = defaultTerminationTimeout
	}

	return o
}

// WithTerminationTimeout returns an Option that specifies the timeout duration for the termination.
func WithTerminationTimeout(duration time.Duration) Option {
	return newFuncOption(func(o *option) {
		o.isTerminationTimeoutSet = true
		o.terminationTimeout = duration
	})
}
