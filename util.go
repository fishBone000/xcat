package main

import (
	"fmt"
	"io"
	"net"
	"sync"

	x "socksyx"
)

func logErr(err error) {
	log := x.NewLogEntry(x.SeverityError, 0, err)
	fmt.Println(log.String())
}

func logWarn(err error) {
	log := x.NewLogEntry(x.SeverityWarning, 0, err)
	fmt.Println(log.String())
}

func logInfo(err error) {
	log := x.NewLogEntry(x.SeverityInfo, 0, err)
	fmt.Println(log.String())
}

func logDebug(err error) {
	log := x.NewLogEntry(x.SeverityDebug, 0, err)
	fmt.Println(log.String())
}

type strAddr struct {
	network string
	str     string
}

func newStrAddr(n, a string) *strAddr {
	return &strAddr{n, a}
}

func (a *strAddr) Network() string {
	return a.network
}

func (a *strAddr) String() string {
	return a.str
}

type capConn struct {
	net.Conn
	capper x.Capsulator
}

func newCapConn(addr string, neg x.Subnegotiator) (*capConn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	capper, err := neg.Negotiate(conn)
	if err != nil {
		return nil, err
	}

	return &capConn{Conn: conn, capper: capper}, nil
}

func (c *capConn) Read(p []byte) (int, error) {
	return c.capper.Read(p)
}

func (c *capConn) Write(p []byte) (int, error) {
	return c.capper.Write(p)
}

type addrConn struct {
	net.Conn
	laddr net.Addr
	raddr net.Addr
}

func (c *addrConn) LocalAddr() net.Addr {
	return c.laddr
}

func (c *addrConn) RemoteAddr() net.Addr {
	return c.raddr
}

func relay(clientConn, hostConn net.Conn) error {
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

	err := x.NewRelayErr(clientConn, hostConn, chErr, hcErr)
  return err
}

func closeCloser(c io.Closer) {
  switch c := c.(type) {
  case net.Conn:
    logInfo(x.NewOpErr("connection close", c, nil))
    err := c.Close()
    if err != nil {
      logErr(x.NewOpErr("close connection", c, err))
    }
  case net.Listener:
    logInfo(x.NewOpErr("listener close", c, nil))
    err := c.Close()
    if err != nil {
      logErr(x.NewOpErr("close listener", c, err))
    }
  default:
    logInfo(x.NewOpErr(fmt.Sprintf("%T close", c), c, nil))
    err := c.Close()
    if err != nil {
      logErr(x.NewOpErr(fmt.Sprintf("%T close", c), c, err))
    }
  }
}
