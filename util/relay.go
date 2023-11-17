package util

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/fishBone000/xcat/log"
)

type RelayError struct {
	ClientRemoteAddr net.Addr
	ClientLocalAddr  net.Addr
	HostRemoteAddr   net.Addr
	HostLocalAddr    net.Addr
	Client2HostErr   error
	Host2ClientErr   error
}

func NewRelayErr(clientConn, hostConn net.Conn, chErr, hcErr error) error {
  if errors.Is(chErr, io.EOF) && errors.Is(hcErr, io.EOF) {
    return nil
  }
	return &RelayError{
		ClientRemoteAddr: clientConn.RemoteAddr(),
		ClientLocalAddr:  clientConn.LocalAddr(),
		HostRemoteAddr:   hostConn.RemoteAddr(),
		HostLocalAddr:    hostConn.LocalAddr(),
		Client2HostErr:   chErr,
		Host2ClientErr:   hcErr,
	}
}

func (e *RelayError) Error() string {
	return fmt.Sprintf(
		"%s: \nClient to host: %s\nHost to client: %s",
		RelayAddr2str(e.ClientRemoteAddr, e.ClientLocalAddr, e.HostLocalAddr, e.HostRemoteAddr),
		e.Client2HostErr, e.Host2ClientErr,
	)
}

func (e *RelayError) Unwrap() (errs []error) {
	errs = []error{e.Client2HostErr, e.Host2ClientErr}
	return
}

func RelayAddr2str(craddr, claddr, hladdr, hraddr net.Addr) string {
	return fmt.Sprintf(
		"relay [client %s]<->[%s server %s]<->[%s host]",
		craddr, claddr,
		hladdr, hraddr,
	)
}

func Relay2str(cConn net.Conn, hConn net.Conn) string {
	return RelayAddr2str(cConn.RemoteAddr(), cConn.LocalAddr(), hConn.LocalAddr(), hConn.RemoteAddr())
}

func Relay(clientConn, hostConn net.Conn) error {
	// Copied & pasted from midlayer.go
  errCh := make(chan error, 1)

	doCopy := func(dst, src net.Conn) {
    _, err := io.Copy(dst, src)
    errCh <- err
	}
	go doCopy(clientConn, hostConn)
	go doCopy(hostConn, clientConn)

  err := <- errCh

	CloseCloser(clientConn)
	CloseCloser(hostConn)

	return err
}
