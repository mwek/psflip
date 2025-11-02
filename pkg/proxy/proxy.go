package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

var ErrServerClosed = errors.New("proxy: Server closed")

type TCPProxy struct {
	ctx    context.Context
	cancel context.CancelFunc
	stop   atomic.Bool
	wg     sync.WaitGroup
}

func New() *TCPProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &TCPProxy{
		ctx:    ctx,
		cancel: cancel,
		stop:   atomic.Bool{},
		wg:     sync.WaitGroup{},
	}
}

func (tp *TCPProxy) Stop() {
	tp.stop.Store(true)
	tp.cancel()
	tp.wg.Wait()
}

func (tp *TCPProxy) stopping() bool {
	return tp.stop.Load()
}

func (tp *TCPProxy) Add(l net.Listener, network, dst string) func() error {
	tp.wg.Add(1)
	return func() error {
		return tp.serve(l, network, dst)
	}
}

func (tp *TCPProxy) serve(l net.Listener, network, dst string) error {
	defer l.Close()
	defer tp.wg.Done()

	// Shutdown the listener when we are shutting down.
	go func() {
		<-tp.ctx.Done()
		l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if tp.stopping() {
				return ErrServerClosed
			}
			return err
		}

		go proxyConn(conn, network, dst)
	}
}

func proxyConn(src net.Conn, network, dst string) {
	defer src.Close()

	d, err := net.Dial(network, dst)
	if err != nil {
		return
	}
	defer d.Close()

	// Copy data between source and destination. The destination is responsible for gracefully closing the connection.
	// If graceful shutdown does not work (i.e. the child process is killed), the kernel will forcefully close the
	// connection after psflip exits.
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() { defer wg.Done(); io.Copy(d, src) }()
	go func() { defer wg.Done(); io.Copy(src, d) }()
	wg.Wait()
}
