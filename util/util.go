package util

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

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
		log.Debug("close connection ", ConnStr(c))
		err := c.Close()
		if err != nil {
			log.Warnf("close connection %s: %w", ConnStr(c), err)
		}
	case net.Listener:
		log.Debug("close listener ", c.Addr())
		err := c.Close()
		if err != nil {
			log.Warnf("close connection %s: %w", c.Addr(), err)
		}
  case *MultiListener:
    log.Debug("close listener ", c.Addr())
		err := c.Close()
		if err != nil {
			log.Warnf("close listener %s: \n%w", c.Addr(), err)
		}
	default:
		log.Info(fmt.Sprintf("close %T", c))
		err := c.Close()
		if err != nil {
			log.Warnf("close %T: %w", c, err)
		}
	}
}

func ConnStr(c net.Conn) string {
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
