package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/fishBone000/xcat/log"
	"github.com/fishBone000/xcat/ray"
	"github.com/fishBone000/xcat/util"
)

func runServer() {
	log.Info("Server startup! ")

	l, err := util.ListenMultiple("tcp", LAddr)
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
			l, err := net.Listen("tcp", net.JoinHostPort(LHost, "0"))
			if err != nil {
				log.Errf("Listen data link failed: %w. ", err)
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
			go serveDataLink(l)
		}
	}
}

func serveDataLink(l net.Listener) {
	dialed := make(chan struct{})
	var outbound net.Conn
	var dialErr error
	go func() {
		outbound, dialErr = net.Dial("tcp", net.JoinHostPort(Host, strconv.Itoa(Port)))
		close(dialed)
	}()

	c, err := l.Accept()
	if err != nil {
		log.Errf("Failed to accept data link on %s: %w", l.Addr(), err)
    util.CloseCloser(l)
		return
	}
  util.CloseCloser(l)

	rconn, err := ray.FromConn(c, []byte(Usr), []byte(Pwd))
	if err != nil {
		log.Errf("Ray negotiation on data link %s failed: %w", l.Addr(), err)
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
