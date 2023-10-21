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
			log.Errf(
				"Error reading request on control link %s, closing: %w. ",
				util.ConnStr(rconn), err,
			)
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

			log.Infof("Port %d allocated on control link %s, start listening. ", port, util.ConnStr(rconn))
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
	defer util.CloseCloser(rconn)

	select {
	case <-dialed:
	}

	if dialErr != nil {
		log.Errf("Error dial outbound for control link %s: %w", util.ConnStr(rconn), dialErr)
		return
	}

	err = util.Relay(rconn, outbound)
	log.Infof("Relay finished, detail: \n%w", err)
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
	raddr, _ := net.ResolveUDPAddr("udp", tcpIn.RemoteAddr().String())
	udpIn, err := net.DialUDP("udp", laddr, raddr)
	if err != nil {
		log.Errf("Failed to dial UDP outbound for UDP data link %s: %w. ", util.ConnStr(tcpIn), err)
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
	ru := &ray.RayUDP{
		TCP: tcpIn,
		UDP: udpIn,
		Ray: r,
	}

	fatal := util.Fatal{}
	go func() {
		buffer := make([]byte, 65535)
		errCnt := 0
		for {
			n, err := ru.Read(buffer)
			if n > 0 {
				udpOut.Write(buffer[:n])
			}
			if err != nil {
				errCnt++
				if errCnt < 16 {
					continue
				}
				fatal.Set(err)
				return
			} else {
				errCnt = 0
			}
		}
	}()
	go func() {
		buffer := make([]byte, 65535)
		errCnt := 0
		for {
			n, err := udpOut.Read(buffer)
			if n > 0 {
				ru.Write(buffer[:n])
			}
			if err != nil {
				errCnt++
				if errCnt < 16 {
					continue
				}
				fatal.Set(err)
				return
			} else {
				errCnt = 0
			}
		}
	}()

	<-fatal.Chan()
	if err := ru.ErrTCP(); err != nil {
		if errors.Is(err, io.EOF) {
			log.Infof("Relay UDP for inbound %s finished: EOF", util.ConnStr(ru))
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
