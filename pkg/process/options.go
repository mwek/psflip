package process

import "os"

type Option func(*options)

type options struct {
	env   []string
	dir   string
	files []*os.File
}

// Env passess extra environment to the process
func Env(env ...string) Option {
	return func(o *options) {
		o.env = append(o.env, env...)
	}
}

// Dir sets the working directory for the process
func Dir(dir string) Option {
	return func(o *options) {
		o.dir = dir
	}
}

func Files(f ...*os.File) Option {
	return func(o *options) {
		o.files = append(o.files, f...)
	}
}
