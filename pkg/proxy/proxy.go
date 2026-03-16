package proxy

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

// connRW allows to close the read and write sides of a connection.
type connRW interface {
	CloseRead() error
	CloseWrite() error
}

var ErrServerClosed = errors.New("proxy: Server closed")

type TCPProxy struct {
	ctx    context.Context
	cancel context.CancelFunc
	stop   atomic.Bool
	estWg  sync.WaitGroup
	connWg sync.WaitGroup
}

func New() *TCPProxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &TCPProxy{
		ctx:    ctx,
		cancel: cancel,
		stop:   atomic.Bool{},
		estWg:  sync.WaitGroup{},
		connWg: sync.WaitGroup{},
	}
}

func (tp *TCPProxy) Stop() {
	tp.stop.Store(true)
	tp.cancel()
	tp.estWg.Wait()
}

func (tp *TCPProxy) Wait() {
	tp.connWg.Wait()
}

func (tp *TCPProxy) stopping() bool {
	return tp.stop.Load()
}

func (tp *TCPProxy) Add(l net.Listener, network, dst string) func() error {
	tp.estWg.Add(1)
	return func() error {
		return tp.serve(l, network, dst)
	}
}

func (tp *TCPProxy) serve(l net.Listener, network, dst string) error {
	defer l.Close()
	defer tp.estWg.Done()

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

		tp.connWg.Add(1)
		go tp.proxyConn(conn, network, dst)
	}
}

func (tp *TCPProxy) proxyConn(src net.Conn, network, dst string) {
	defer tp.connWg.Done()
	defer src.Close()

	d, err := net.Dial(network, dst)
	if err != nil {
		return
	}
	defer d.Close()

	// We only support TCPConn and UnixConn. Both implement the connRW interface.
	if _, ok := src.(connRW); !ok {
		panic("source connection does not support CloseRead and CloseWrite")
	}
	if _, ok := d.(connRW); !ok {
		panic("destination connection does not support CloseRead and CloseWrite")
	}

	// Copy data between source and destination. The destination is responsible for gracefully closing the connection.
	// If graceful shutdown does not work (i.e. the child process is killed), the kernel will forcefully close the
	// connection after psflip exits.
	wg := sync.WaitGroup{}
	wg.Add(2)
	var stream = func(src, dst net.Conn) {
		defer wg.Done()
		srcRW := src.(connRW)
		dstRW := dst.(connRW)
		defer srcRW.CloseRead()
		defer dstRW.CloseWrite()
		n, err := io.Copy(dst, src)
		if err != nil {
			log.Printf("error copying data after %d bytes: %v", n, err)
		}
	}
	go stream(d, src)
	go stream(src, d)
	wg.Wait()
}
