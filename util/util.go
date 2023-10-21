package util

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"

	"github.com/fishBone000/xcat/log"
)

type StrAddr struct {
	network string
	str     string
}

func NewStrAddr(n, a string) *StrAddr {
	return &StrAddr{n, a}
}

func (a *StrAddr) Network() string {
	return a.network
}

func (a *StrAddr) String() string {
	return a.str
}

type AddrConn struct {
	net.Conn
	laddr net.Addr
	raddr net.Addr
}

func (c *AddrConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *AddrConn) RemoteAddr() net.Addr {
	return c.raddr
}

func CloseCloser(c io.Closer) {
	switch c := c.(type) {
	case net.Conn:
		log.Debugf("close %s connection %s", c.LocalAddr().Network(), ConnStr(c))
		err := c.Close()
		if err != nil {
			log.Debugf("close %s connection %s: %w", c.LocalAddr().Network(), ConnStr(c), err)
		}
	case *UDPConn:
		log.Debugf("close %s connection %s", c.LocalAddr().Network(), ConnStr(c))
		err := c.Close()
		if err != nil {
			log.Debugf("close %s connection %s: %w", c.LocalAddr().Network(), ConnStr(c), err)
		}
	case net.Listener:
		log.Debugf("close %s listener %s", c.Addr().Network(), c.Addr())
		err := c.Close()
		if err != nil {
			log.Debugf("close %s listener %s: %w", c.Addr().Network(), c.Addr(), err)
		}
	default:
		log.Info(fmt.Sprintf("close %T", c))
		err := c.Close()
		if err != nil {
			log.Debugf("close %T: %w", c, err)
		}
	}
}

type connAddressHolder interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

func ConnStr(c connAddressHolder) string {
	return fmt.Sprintf("%s<L-R>%s", c.LocalAddr(), c.RemoteAddr())
}

func ParsePortFromAddr(addr net.Addr) (uint16, error) {
	if addr == nil {
		return 0, errors.New("parse port: nil address")
	}
	_, portStr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("parse port %s: %w", portStr, err)
	}
	if port < 0x00 && port > 0xFFFF {
		return 0, fmt.Errorf("invalid port range %d", port)
	}

	return uint16(port), nil
}

type FlagOnce struct {
	ch  chan struct{}
	set bool
	mux sync.Mutex
}

func (f *FlagOnce) Set() bool {
	f.mux.Lock()
	defer f.mux.Unlock()
	if f.set {
		return false
	}
	if f.ch != nil {
		close(f.ch)
	}
	f.set = true
	return true
}

func (f *FlagOnce) Get() bool {
	f.mux.Lock()
	defer f.mux.Unlock()
	return f.set
}

func (f *FlagOnce) Chan() <-chan struct{} {
	f.mux.Lock()
	defer f.mux.Unlock()
	if f.ch == nil {
		f.ch = make(chan struct{})
		if f.set {
			close(f.ch)
		}
	}
	return f.ch
}

type Fatal struct {
	inner error
	set   bool
	mux   sync.Mutex
	ch    chan struct{}
}

func (f *Fatal) Set(err error) bool {
	f.mux.Lock()
	defer f.mux.Unlock()

  if f.set {
    return false
  }
  f.inner = err
  f.set = true
  if f.ch != nil {
    close(f.ch)
  }
  return true
}

func (f *Fatal) Get() error {
	f.mux.Lock()
	defer f.mux.Unlock()

	return f.inner
}

func (f *Fatal) Chan() <-chan struct{} {
	f.mux.Lock()
	defer f.mux.Unlock()
	if f.ch == nil {
		f.ch = make(chan struct{})
		if f.set {
			close(f.ch)
		}
	}
	return f.ch
}

type Retry struct {
  cnt uint
  Max uint
  err error
  mux sync.Mutex
}

func (r *Retry) Test(err error) bool {
  r.mux.Lock()
  defer r.mux.Unlock()
  if err == nil {
    r.cnt = 0
    return false
  }
  if r.err != nil {
    return true
  }
  r.cnt++
  if r.cnt > r.Max {
    r.err = err
    return true
  }
  return false
}

func (r *Retry) Err() error {
  r.mux.Lock()
  defer r.mux.Unlock()
  return r.err
}
