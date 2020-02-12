package manifestival

import "github.com/go-logr/logr"

type options struct {
	recursive bool
	logger    logr.Logger
	client    Client
}

type option func(*options)

func Recursive(opts *options) {
	opts.recursive = true
}

func UseLogger(log logr.Logger) option {
	return func(o *options) {
		o.logger = log
	}
}

func UseClient(client Client) option {
	return func(o *options) {
		o.client = client
	}
}
