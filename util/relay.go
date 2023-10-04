package util

import (
	"fmt"
	"io"
	"net"
	"sync"
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
		"%s, client to host: %s, host to client: %s",
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

	CloseCloser(clientConn)
	CloseCloser(hostConn)

	err := NewRelayErr(clientConn, hostConn, chErr, hcErr)
	return err
}
