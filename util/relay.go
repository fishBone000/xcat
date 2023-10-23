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

func NewRelayErr(clientConn, hostConn net.Conn, chErr, hcErr error) *RelayError {
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
	var chErr error
	var hcErr error
	wg := sync.WaitGroup{}
	wg.Add(2)

	doCopy := func(dst, src net.Conn) {
		_, hcErr = io.Copy(dst, src)
		if c, ok := dst.(interface{ CloseWrite() error }); ok {
			if err := c.CloseWrite(); err != nil && err != net.ErrClosed {
				log.Warnf("Error closing write end of %s: %w. ", ConnStr(dst), err)
			} else {
        log.Infof("Close write end %s. ", ConnStr(dst))
      }
		}
		wg.Done()
	}
	go doCopy(clientConn, hostConn)
	go doCopy(hostConn, clientConn)
	wg.Wait()

	if chErr == nil {
		chErr = io.EOF
	}
	if hcErr == nil {
		hcErr = io.EOF
	}

	CloseCloser(clientConn)
	CloseCloser(hostConn)

	err := NewRelayErr(clientConn, hostConn, chErr, hcErr)
	return err
}
