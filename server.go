package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/fishBone000/xcat/log"
	"github.com/fishBone000/xcat/ray"
	"github.com/fishBone000/xcat/util"
)

func runServer() {
	log.Info("Server startup! ")
	log.Infof("Version: %s", version)

	l, err := util.ListenMultipleTCP("tcp", LAddr)
	if err != nil {
		log.Errf("Failed to listen control link, exitting: %w", err)
		os.Exit(1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Errf("Failed to accept control link, exitting: %w", err)
			os.Exit(1)
		}

		go serveControlLink(conn)
	}
}

func serveControlLink(conn net.Conn) {
	log.Info("New control link " + util.ConnStr(conn))

	rconn, err := ray.FromConn(conn, []byte(Usr), []byte(Pwd))
	if err != nil {
		log.Warnf("Ray negotiation on control link %s failed: %w", util.ConnStr(conn), err)
		util.CloseCloser(conn)
		return
	}

	buf := make([]byte, 16)
	for {
		n, err := rconn.Read(buf)
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Debugf("Finished serving control link %s: EOF", util.ConnStr(rconn))
			} else {
				log.Errf(
					"Error reading request on control link %s, closing: %w. ",
					util.ConnStr(rconn), err,
				)
			}
			util.CloseCloser(rconn)
			return
		}

		for i := 0; i < n; i++ {
			log.Debugf("New port allocating request from %s type 0x%02X", conn.RemoteAddr(), buf[i])
			l, err := util.ListenMultipleTCP("tcp", net.JoinHostPort(LHost, "0"))
			if err != nil {
				log.Errf("Allocate port for new data link failed: %w. ", err)
				util.CloseCloser(rconn)
				return
			}

			port, err := util.ParsePortFromAddr(l.Addr())
			if err != nil {
				panic(fmt.Errorf("IMPOSSIBLE! Failed to parse port from listener address %s: %w", l.Addr(), err))
			}

			portBuf := make([]byte, 2)
			binary.BigEndian.PutUint16(portBuf, uint16(port))
			if _, err := rconn.Write(portBuf); err != nil {
				log.Err("Failed to reply allocated port on control link " + util.ConnStr(rconn) + ". ")
				util.CloseCloser(rconn)
				return
			}

			log.Debugf("Port %d allocated on control link %s, start serving. ", port, util.ConnStr(rconn))
			if buf[i] == 0x00 {
				go serveDataLinkTCP(l)
			} else {
				go serveDataLinkUDP(l)
			}
		}
	}
}

func serveDataLinkTCP(l *util.MultiListenerTCP) {
	dialed := make(chan struct{})
	var outbound net.Conn
	var dialErr error
	go func() {
		outbound, dialErr = net.Dial("tcp", net.JoinHostPort(Host, strconv.Itoa(Port)))
		close(dialed)
	}()

	if DataLinkListenTimeout > 0 {
		err := l.SetDeadline(time.Now().Add(time.Second * time.Duration(DataLinkListenTimeout)))
		if err != nil {
			log.Warnf("Failed to set deadline for listener %s: %w. ", l.Addr(), err)
		}
	}
	c, err := l.Accept()
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			log.Warnf("Timed out listening for TCP data link at %s. ", l.Addr())
		} else {
			log.Errf("Failed to accept TCP data link on %s: %w", l.Addr(), err)
		}
		util.CloseCloser(l)
		return
	}
	util.CloseCloser(l)

	rconn, err := ray.FromConn(c, []byte(Usr), []byte(Pwd))
	if err != nil {
		log.Errf("Ray negotiation on TCP data link %s failed: %w", l.Addr(), err)
		return
	}
	log.Debugf("TCP data link established: %s. ", util.ConnStr(rconn))
	defer util.CloseCloser(rconn)

	select {
	case <-dialed:
	}

	if dialErr != nil {
		log.Errf("Error dial outbound for TCP data link %s.\n%w", util.ConnStr(rconn), dialErr)
		return
	}

	log.Debugf("Relaying for TCP data link %s started. ", util.ConnStr(rconn))
	if err := util.Relay(rconn, outbound); err != nil {
		log.Warnf("Error relaying TCP for inbound %s: \n%w", util.ConnStr(rconn), err)
	} else {
		log.Debugf("Relay TCP finished for inbound %s. ", util.ConnStr(rconn))
	}
}

func serveDataLinkUDP(l *util.MultiListenerTCP) {
	if DataLinkListenTimeout > 0 {
		err := l.SetDeadline(time.Now().Add(time.Second * time.Duration(DataLinkListenTimeout)))
		if err != nil {
			log.Warnf("Failed to set deadline for listener %s: %w. ", l.Addr(), err)
		}
	}

	tcpIn, err := l.Accept()
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			log.Warnf("Timed out listening for data link at %s. ", l.Addr())
		} else {
			log.Errf("Failed to accept data link on %s: %w", l.Addr(), err)
		}
		util.CloseCloser(l)
		return
	}
	util.CloseCloser(l)

	udpOut, err := net.Dial("udp", net.JoinHostPort(Host, strconv.Itoa(int(Port))))
	if err != nil {
		log.Errf("Failed to dial UDP outbound for UDP data link %s: %w. ", util.ConnStr(tcpIn), err)
		util.CloseCloser(tcpIn)
		return
	}

	laddr, _ := net.ResolveUDPAddr("udp", tcpIn.LocalAddr().String())
	udpIn, err := net.ListenUDP("udp", laddr)
	if err != nil {
		log.Errf("Failed to dial UDP inbound for UDP data link %s: %w. ", util.ConnStr(tcpIn), err)
		util.CloseCloser(tcpIn)
		util.CloseCloser(udpOut)
		return
	}

	r, err := ray.Negotiate(tcpIn, []byte(Usr), []byte(Pwd))
	if err != nil {
		log.Errf("Ray negotiation failed for UDP data link %s: %w. ", util.ConnStr(tcpIn), err)
		util.CloseCloser(tcpIn)
		util.CloseCloser(udpOut)
		util.CloseCloser(udpIn)
		return
	}

	ru := ray.NewRayUDP(udpIn, false, tcpIn, r)
	log.Debugf("UDP data link %s established. ", util.ConnStr(ru))

	fatal := util.Fatal{}
	go func() {
		buffer := make([]byte, 65535)
		wRetry := util.Retry{Max: udpIoRetries}
		rRetry := util.Retry{Max: udpIoRetries}
		for {
			n, err := ru.Read(buffer)
			if n > 0 {
				if _, werr := udpOut.Write(buffer[:n]); wRetry.Test(werr) {
					fatal.Set(werr)
					return
				}
			}
			if rRetry.Test(err) {
				fatal.Set(err)
				return
			}
		}
	}()
	go func() {
		buffer := make([]byte, 65535)
		wRetry := util.Retry{Max: udpIoRetries}
		rRetry := util.Retry{Max: udpIoRetries}
		for {
			n, err := udpOut.Read(buffer)
			if n > 0 {
				if _, werr := ru.Write(buffer[:n]); wRetry.Test(werr) {
					fatal.Set(werr)
					return
				}
			}
			if rRetry.Test(err) {
				fatal.Set(err)
				return
			}
		}
	}()

	<-fatal.Chan()
	if err := ru.ErrTCP(); err != nil {
		if errors.Is(err, io.EOF) {
			log.Debugf("Relay UDP for inbound %s finished: EOF", util.ConnStr(ru))
		} else {
			log.Errf("Error relaying UDP for inbound %s: %w", util.ConnStr(ru), err)
		}
	} else {
		log.Errf("Error relaying UDP for inbound %s: %w", util.ConnStr(ru), fatal.Get())
	}
	util.CloseCloser(tcpIn)
	util.CloseCloser(udpIn)
	util.CloseCloser(udpOut)
	return
}
