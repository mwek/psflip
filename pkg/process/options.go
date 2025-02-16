package process

import "os"

type Option func(*options)

type options struct {
	env   []string
	files []*os.File
}

// Env passess extra environment to the process
func Env(env ...string) Option {
	return func(o *options) {
		o.env = append(o.env, env...)
	}
}

func Files(f ...*os.File) Option {
	return func(o *options) {
		o.files = append(o.files, f...)
	}
}
