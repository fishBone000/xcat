package util

import (
	"errors"
	"net"
	"runtime"
	"sync"
)

type MultiListener struct {
	connChan chan net.Conn
	ls       []net.Listener
	errs     []error
	closed   chan struct{}
	mux      sync.Mutex
	addr     net.Addr
}

func ListenMultiple(network, addr string) (*MultiListener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		break
	default:
		return nil, net.UnknownNetworkError(network)
	}

	var ips []net.IP
	var port string
  var host string
	if addr == "" {
		if runtime.GOOS == "dragonfly" || runtime.GOOS == "openbsd" {
			ips = []net.IP{net.IPv4zero, net.IPv6zero}
		} else {
			ips = []net.IP{net.IPv4zero}
		}
		port = "0"
	} else {
		var err error
		host, port, err = net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		ips, err = net.LookupIP(host)
		if err != nil {
			return nil, err
		}
	}

	ls := make([]net.Listener, 0, len(ips))

	var err error
	for _, ip := range ips {
		var l net.Listener
		l, err = net.Listen(network, net.JoinHostPort(ip.String(), port))
		if err != nil {
			break
		}
		if port == "0" {
			_, port, _ = net.SplitHostPort(l.Addr().String())
		}

		ls = append(ls, l)
	}

	if err != nil {
		for _, l := range ls {
			CloseCloser(l)
		}
		return nil, err
	}

	ml := &MultiListener{
		connChan: make(chan net.Conn),
		ls:       ls,
		errs:     make([]error, len(ls)),
		closed:   make(chan struct{}),
    addr: NewStrAddr(network, net.JoinHostPort(host, port)),
	}
	for i, l := range ls {
		go func(i int, l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					ml.mux.Lock()
					defer ml.mux.Unlock()
					ml.errs[i] = err
					close(ml.closed)
					return
				}

				ml.connChan <- c
			}
		}(i, l)
	}

	return ml, nil
}

func (ml *MultiListener) Accept() (net.Conn, error) {
	select {
	case c := <-ml.connChan:
		return c, nil
	case <-ml.closed:
		ml.mux.Lock()
		defer ml.mux.Unlock()

		return nil, errors.Join(ml.errs...)
	}
}

func (ml *MultiListener) Close() error {
	errs := make([]error, 0, 4)
	ml.mux.Lock()
	defer ml.mux.Unlock()

	for _, l := range ml.ls {
		errs = append(errs, l.Close())
	}

	return errors.Join(errs...)
}

func (ml *MultiListener) Addr() net.Addr {
  return ml.addr
}
