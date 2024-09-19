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
	"github.com/fishBone000/xcat/stat"
	"github.com/fishBone000/xcat/util"
)

var (
	cnt stat.Counter
	sf stat.StatFile
)

func runClient() {
	log.Info("Client start up!")
	log.Infof("Version: %s", version)
	sf.Init()
	log.Infof("Stastic file: %s", sf.Name())

	ctrl := ctrl.NewCtrlLink(
		net.JoinHostPort(Host, strconv.Itoa(Port)), []byte(Usr), []byte(Pwd),
		time.Second*time.Duration(CtrlLinkTimeout),
	)

	ctrl.Sf = &sf

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

 // New Inbound: n
 // Got port: p(P)
 // Ray: r(R)
 // Relay: l(L)
func serveInboundTCP(inbound net.Conn, ctrl *ctrl.ControlLink) {
	id := cnt.Tick()
	sf.Write("t", id, "n")
	
	log.Debugf("New TCP inbound %s. ", inbound.RemoteAddr().String())
	port, err := ctrl.GetPortTCP()
	if err != nil {
		sf.Write("t", id, "P")
		log.Debugf("Failed to get available port, closing inbound %s. ", inbound.RemoteAddr())
		util.CloseCloser(inbound)
		return
	}
	sf.Write("t", id, "p")
	log.Debug(fmt.Sprintf("Got port %d for inbound %s. ", port, util.ConnStr(inbound)))

	rconn, err := ray.Dial("tcp", net.JoinHostPort(Host, strconv.Itoa(int(port))), []byte(Usr), []byte(Pwd))
	if err != nil {
		sf.Write("t", id, "R")
		log.Errf("Establish TCP data link to server %s failed, closing inbound %s: %w", Addr, util.ConnStr(inbound), err)
		util.CloseCloser(inbound)
		return
	}
	sf.Write("t", id, "r")
	log.Debugf("Established TCP data link %s for inbound %s, relay starting. ", util.ConnStr(rconn), util.ConnStr(inbound))

	if err := util.Relay(inbound, rconn); err != nil {
		sf.Write("t", id, "L")
		log.Warnf("Error relaying TCP for inbound %s: \n%w", util.ConnStr(inbound), err)
	} else {
		sf.Write("t", id, "l")
		log.Debugf("Relay TCP finished for inbound %s. ", util.ConnStr(inbound))
	}
}

 // New Inbound: n
 // Got port: p(P)
 // Ray: r(R)
 // Relay: l(L)
func serveInboundUDP(inbound *util.UDPConn, ctrl *ctrl.ControlLink) {
	defer util.CloseCloser(inbound)

	id := cnt.Tick()
	sf.Write("u", id, "n")

	log.Debugf("New UDP inbound %s. ", inbound.RemoteAddr().String())
	port, err := ctrl.GetPortUDP()
	if err != nil {
		sf.Write("u", id, "P")
		log.Errf("Failed to get available port, closing inbound %s. ", inbound.RemoteAddr())
		return
	}
	sf.Write("u", id, "p")

	addr := net.JoinHostPort(Host, strconv.Itoa(int(port)))
	ru, err := ray.DialTimeoutUDP("udp", addr, []byte(Usr), []byte(Pwd), time.Second*time.Duration(DataLinkListenTimeout))
	if err != nil {
		sf.Write("u", id, "R")
		log.Errf("Failed to dial UDP data link to %s. Reason: \n%w", addr, err)
		return
	}
	sf.Write("u", id, "r")
	defer util.CloseCloser(ru)

	fatal := util.Fatal{}
	activity := make(chan struct{}, 4)
	go func() {
		for {
			p, err := inbound.Read()
			if err != nil {
				fatal.Set(err)
				return
			}
			activity <- struct{}{}
			_, err = ru.Write(p)
			if err != nil {
				fatal.Set(err)
				return
			}
		}
	}()
	go func() {
		buffer := make([]byte, 65535)
		for {
			n, err := ru.Read(buffer)
			var werr error
			if n > 0 {
				activity <- struct{}{}
				_, werr = inbound.Write(buffer[:n])
			}
			switch {
			case err != nil:
				fatal.Set(err)
				return
			case werr != nil:
				fatal.Set(werr)
				return
			}
		}
	}()
	go func() {
		d := time.Second * time.Duration(UDPTimeout)
		var ticker *time.Ticker
		if UDPTimeout > 0 {
			ticker = time.NewTicker(d)
			defer ticker.Stop()
		} else {
			// Make a dummy ticker
			ticker = new(time.Ticker)
			ticker.C = make(<-chan time.Time)
		}
		for {
			select {
			case <-activity:
				ticker.Reset(d)
			case <-ticker.C:
				fatal.Set(nil)
				return
			}
		}
	}()

	<-fatal.Chan()
	if err := ru.ErrTCP(); err != nil {
		sf.Write("u", id, "L")
		log.Errf("Error relaying UDP for %s. Reason:\n%w", inbound.RemoteAddr(), err)
		return
	}
	if err := fatal.Get(); err != nil {
		sf.Write("u", id, "L")
		log.Errf("Error relaying UDP for %s. Reason:\n%w", inbound.RemoteAddr(), fatal.Get())
		return
	}
	sf.Write("u", id, "l")
	log.Debugf("Relay UDP for %s finished (no activity for %d secs). ", inbound.RemoteAddr(), UDPTimeout)
}
