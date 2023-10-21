package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/fishBone000/xcat/ctrl"
	"github.com/fishBone000/xcat/log"
	"github.com/fishBone000/xcat/ray"
	"github.com/fishBone000/xcat/util"
)

func runClient() {
	log.Info(fmt.Errorf("Client start up!"))

	ctrl, err := ctrl.NewCtrlLink(
		net.JoinHostPort(Host, strconv.Itoa(Port)), []byte(Usr), []byte(Pwd),
		time.Second*time.Duration(CtrlLinkTimeout),
	)
	if err != nil {
		log.Err("Establish control link failed! Exitting. ")
		os.Exit(1)
	}

	lt, err := util.ListenMultipleTCP("tcp", LAddr)
	if err != nil {
		log.Err("Listen TCP failed: " + err.Error())
		os.Exit(1)
	}
  lu, err := util.ListenMultipleUDP("udp", LAddr)
	if err != nil {
		log.Err("Listen UDP failed: " + err.Error())
		os.Exit(1)
	}

  fatal := util.Fatal{}

  go func() {
    for {
      inbound, err := lt.Accept()
      if err != nil {
        fatal.Set(err)
        return
      }
      go serveInboundTCP(inbound, ctrl)
    }
  }()

  go func() {
    for {
      inbound, err := lu.Accept()
      if err != nil {
        fatal.Set(err)
        return
      }
      go serveInboundUDP(inbound, ctrl)
    }
  }()

  <-fatal.Chan()
  fmt.Printf("Accept inbound failed, exitting.\nReason: %s.\n", fatal.Get().Error())
}

func serveInboundTCP(inbound net.Conn, ctrl *ctrl.ControlLink) {
	log.Infof("New TCP inbound %s. ", inbound.RemoteAddr().String())
	port, err := ctrl.GetPortTCP()
	if err != nil {
		log.Errf("Failed to get available port, closing inbound %s. ", inbound.RemoteAddr())
		util.CloseCloser(inbound)
		return
	}
	log.Debug(fmt.Sprintf("Got port %d for inbound %s. ", port, util.ConnStr(inbound)))

	rconn, err := ray.Dial("tcp", net.JoinHostPort(Host, strconv.Itoa(int(port))), []byte(Usr), []byte(Pwd))
	if err != nil {
    log.Errf("Establish TCP data link to server %s failed, closing inbound %s: %w", Addr, util.ConnStr(inbound), err)
		util.CloseCloser(inbound)
		return
	}
	log.Infof("Established TCP data link %s for inbound %s, relay starting. ", util.ConnStr(rconn), util.ConnStr(inbound))

	err = util.Relay(inbound, rconn)
	log.Info(fmt.Errorf("Relay TCP finished for inbound %s, result: \n%w", util.ConnStr(inbound), err))
}

func serveInboundUDP(inbound *util.UDPConn, ctrl *ctrl.ControlLink) {
  log.Infof("New UDP inbound %s. ", inbound.RemoteAddr().String())
  port, err := ctrl.GetPortUDP()
  if err != nil {
		log.Errf("Failed to get available port, closing inbound %s. ", inbound.RemoteAddr())
		util.CloseCloser(inbound)
		return
  }

  addr := net.JoinHostPort(Host, strconv.Itoa(int(port)))
  ru, err := ray.DialTimeoutUDP("udp", addr, []byte(Usr), []byte(Pwd), time.Second * time.Duration(DataLinkListenTimeout))
  if err != nil {
    log.Errf("Failed to dial UDP data link to %s. Reason: \n%w", addr, err)
    return
  }

  fatal := util.Fatal{}
  activity := make(chan struct{})
  go func() {
    for {
      p, err := inbound.Read()
      if err != nil {
        fatal.Set(err)
        return
      }
      activity <- struct{}{}
      ru.Write(p)
    }
  }()
  go func() {
    buffer := make([]byte, 65535)
    for {
      n, err := ru.Read(buffer)
      if n > 0 {
        activity <- struct{}{}
        inbound.Write(buffer[:n])
      }
      if err != nil {
        fatal.Set(err)
        return
      }
    }
  }()
  go func() {
    d := time.Second * time.Duration(UDPTimeout)
    var ticker *time.Ticker
    if UDPTimeout > 0 {
      ticker = time.NewTicker(d)
    } else {
      // Make a dummy ticker
      ticker = new(time.Ticker)
      ticker.C = make(<-chan time.Time)
    }
    defer ticker.Stop()
    for {
      select {
      case <-activity:
        ticker.Reset(d)
      case <-ticker.C:
        fatal.Set(nil)
        util.CloseCloser(inbound)
        util.CloseCloser(ru)
        return
      }
    }
  }()

  <-fatal.Chan()
  if err := fatal.Get(); err != nil {
    log.Errf("Error relaying UDP for %s. Reason:\n%w", inbound.RemoteAddr(), fatal.Get())
  }
  log.Infof("Relay UDP for %s finished (no activity for %d secs). ", inbound.RemoteAddr(), UDPTimeout)
}
