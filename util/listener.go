package util

import (
	"errors"
	"net"
	"runtime"
	"sync"
	"time"
)

type MultiListenerTCP struct {
	connChan chan net.Conn
	ls       []*net.TCPListener
	errs     []error
	closed   FlagOnce
	mux      sync.Mutex
	addr     net.Addr
}

func ListenMultipleTCP(network, addr string) (*MultiListenerTCP, error) {
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

	ls := make([]*net.TCPListener, 0, len(ips))

	var err error
	for _, ip := range ips {
		var l *net.TCPListener
		laddr, _ := net.ResolveTCPAddr(network, net.JoinHostPort(ip.String(), port))
		l, err = net.ListenTCP(network, laddr)
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

	ml := &MultiListenerTCP{
		connChan: make(chan net.Conn),
		ls:       ls,
		errs:     make([]error, len(ls)),
		addr:     NewStrAddr(network, net.JoinHostPort(host, port)),
	}
	for i, l := range ls {
		go func(i int, l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					ml.mux.Lock()
					defer ml.mux.Unlock()
					ml.errs[i] = err
					ml.closed.Set()
					return
				}

				ml.connChan <- c
			}
		}(i, l)
	}

	return ml, nil
}

func (ml *MultiListenerTCP) Accept() (net.Conn, error) {
	select {
	case c := <-ml.connChan:
		return c, nil
	case <-ml.closed.Chan():
		ml.mux.Lock()
		defer ml.mux.Unlock()

		return nil, errors.Join(ml.errs...)
	}
}

func (ml *MultiListenerTCP) Close() error {
	errs := make([]error, 0, 4)
	ml.mux.Lock()
	defer ml.mux.Unlock()

	for _, l := range ml.ls {
		errs = append(errs, l.Close())
	}

	return errors.Join(errs...)
}

func (ml *MultiListenerTCP) Addr() net.Addr {
	return ml.addr
}

func (ml *MultiListenerTCP) SetDeadline(t time.Time) (err error) {
	for _, l := range ml.ls {
		err = l.SetDeadline(t)
		if err != nil {
			return
		}
	}
	return
}

type MultiListenerUDP struct {
	mux sync.Mutex

	netConnsByAddr    map[string]*net.UDPConn
	connsTableByLAddr map[string]map[string]*UDPConn // Local Address -> Remote Address -> UDPCOnn
	acceptQueue       chan *UDPConn
	addr              *StrAddr

	fatal Fatal
}

func ListenMultipleUDP(network, addr string) (*MultiListenerUDP, error) {
	switch network {
	case "udp":
	case "udp4":
	case "udp6":
	default:
		return nil, net.UnknownNetworkError(network)
	}

	var host, port string
	var ips []net.IP
	if addr == "" {
		if runtime.GOOS == "dragonfly" || runtime.GOOS == "openbsd" {
			ips = []net.IP{net.IPv4zero, net.IPv6zero}
		} else {
			ips = []net.IP{net.IPv4zero}
		}
    host = "localhost"
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

	d := &MultiListenerUDP{
		netConnsByAddr:    make(map[string]*net.UDPConn),
		connsTableByLAddr: make(map[string]map[string]*UDPConn),
		acceptQueue:       make(chan *UDPConn),
		addr:              NewStrAddr(network, net.JoinHostPort(host, port)),
	}

	var err error
	for _, ip := range ips {
		var laddr *net.UDPAddr
		laddr, err = net.ResolveUDPAddr(network, net.JoinHostPort(ip.String(), port))
		if err != nil {
			break
		}
		var conn *net.UDPConn
		conn, err = net.ListenUDP(network, laddr)
		if err != nil {
			break
		}
		if port == "0" {
			_, port, _ = net.SplitHostPort(conn.LocalAddr().String())
		}
		d.netConnsByAddr[conn.LocalAddr().String()] = conn
	}

	if err != nil {
		for _, conn := range d.netConnsByAddr {
			conn.Close()
		}
		return nil, err
	}

	for addr := range d.netConnsByAddr {
		d.connsTableByLAddr[addr] = make(map[string]*UDPConn)
	}

	go d.run()
	return d, nil
}

func (l *MultiListenerUDP) Accept() (*UDPConn, error) {
	select {
	case conn := <-l.acceptQueue:
		return conn, nil
	case <-l.fatal.ch:
		return nil, l.fatal.Get()
	}
}

func (l *MultiListenerUDP) Addr() net.Addr {
	return l.addr
}

func (l *MultiListenerUDP) pushAcceptQueueNoLock(c *UDPConn) {
	select {
	case l.acceptQueue <- c:
	default:
		discard := <-l.acceptQueue
		delete(l.connsTableByLAddr[discard.laddr.String()], discard.RemoteAddr().String())
		l.acceptQueue <- c
	}
}

func (l *MultiListenerUDP) run() {
	for _, netConn := range l.netConnsByAddr {
		go func(netConn *net.UDPConn) {
			buffer := make([]byte, 65536)
			laddr := netConn.LocalAddr().String()
			for {
				n, addr, err := netConn.ReadFromUDP(buffer)

				l.mux.Lock()
				conn := l.connsTableByLAddr[laddr][addr.String()]
				if conn == nil {
					conn = &UDPConn{
						l:      l,
						inner:  netConn,
						laddr:  netConn.LocalAddr(),
						raddr:  addr,
						buffer: make(chan []byte, 32),
					}
					l.connsTableByLAddr[laddr][addr.String()] = conn
					l.pushAcceptQueueNoLock(conn)
				}
				l.mux.Unlock()

				p := make([]byte, n)
				copy(p, buffer[:n])
				conn.pushBuffer(p)

				if err != nil {
					l.fatal.Set(err)
					return
				}
			}
		}(netConn)
	}
}

type UDPConn struct {
	l      *MultiListenerUDP
	inner  *net.UDPConn
	laddr  net.Addr
	raddr  net.Addr
	buffer chan []byte
  closed FlagOnce
	mux    sync.Mutex
}

func (c *UDPConn) Write(b []byte) (int, error) {
  if c.closed.Get() {
    return 0, net.ErrClosed
  }
	return c.inner.WriteTo(b, c.raddr)
}

func (c *UDPConn) Read() ([]byte, error) {
  if c.closed.Get() {
    return nil, net.ErrClosed
  }
	if c.l.fatal.Get() != nil {
		return nil, c.l.fatal.Get()
	}
	select {
	case b := <-c.buffer:
		return b, nil
	case <-c.l.fatal.Chan():
		return nil, c.l.fatal.Get()
	}
}

func (c *UDPConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *UDPConn) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *UDPConn) Close() error {
	ok := c.closed.Set()
	c.l.mux.Lock()
	defer c.l.mux.Unlock()
  laddr := c.LocalAddr().String()
  raddr := c.RemoteAddr().String()
	if c.l.connsTableByLAddr[laddr][raddr] == c {
		delete(c.l.connsTableByLAddr[laddr], raddr)
	}
	if !ok {
		return net.ErrClosed
	}
	return nil
}

func (c *UDPConn) pushBuffer(p []byte) {
	c.mux.Lock()
	defer c.mux.Unlock()
	if cap(c.buffer) == 0 {
		panic("rcv buffer size for UDP cannot be 0")
	}
	select {
	case c.buffer <- p:
	default:
		<-c.buffer
		c.buffer <- p
	}
}
