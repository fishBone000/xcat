package ray

import (
	"errors"
	"net"
)

var ErrAuthFailed = errors.New("authentication failed")

// A RelayError represents errors and address info of TCP traffic relaying
// for CONNECT and BIND requests.
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
