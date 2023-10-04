package util

import (
	"fmt"
	"io"
	"net"
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

func Relay(clientConn, hostConn net.Conn) error {
	// Copied & pasted from midlayer.go
	var chErr error
	var hcErr error
	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		_, hcErr = io.Copy(clientConn, hostConn)
		wg.Done()
	}()
	go func() {
		_, chErr = io.Copy(hostConn, clientConn)
		wg.Done()
	}()
	wg.Wait()

	if chErr == nil {
		chErr = io.EOF
	}
	if hcErr == nil {
		hcErr = io.EOF
	}

	closeCloser(clientConn)
	closeCloser(hostConn)

	err := NewRelayErr(clientConn, hostConn, chErr, hcErr)
	return err
}

func closeCloser(c io.Closer) {
	switch c := c.(type) {
	case net.Conn:
		log.Info("close connection ", ConnStr(c))
		err := c.Close()
		if err != nil {
      log.Err(fmt.Errorf("close connection %s: %w", ConnStr(c), err))
		}
	case net.Listener:
		log.Info("close listener ", c.Addr())
		err := c.Close()
		if err != nil {
      log.Err(fmt.Errorf("close connection %s: %w", c.Addr(), err))
		}
	default:
		log.Info(fmt.Sprintf("close %T", c))
		err := c.Close()
		if err != nil {
      log.Err(fmt.Errorf("close %T: %w", c, err))
		}
	}
}

func ConnStr(c net.Conn) string {
	return fmt.Sprintf("%s<L-R>%s", c.LocalAddr(), c.RemoteAddr())
}
