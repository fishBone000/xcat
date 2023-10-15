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

	l, err := util.ListenMultiple("tcp", LAddr)
	if err != nil {
		log.Err("Listen failed: " + err.Error())
		os.Exit(1)
	}

	for {
		inbound, err := l.Accept()
		if err != nil {
			log.Err(fmt.Errorf("Accept inbound failed, exitting. \nReason: %w", err))
			os.Exit(1)
		}
		go serveInbound(inbound, ctrl)
	}
}

func serveInbound(inbound net.Conn, ctrl *ctrl.ControlLink) {
	log.Infof("New inbound %s. ", util.ConnStr(inbound))
	port, err := ctrl.GetPort()
	if err != nil {
		log.Err("Failed to get available port, closing inbound " + util.ConnStr(inbound) + ". ")
		util.CloseCloser(inbound)
		return
	}
	log.Debug(fmt.Sprintf("Got port %d for inbound %s. ", port, util.ConnStr(inbound)))

	rconn, err := ray.Dial("tcp", net.JoinHostPort(Host, strconv.Itoa(int(port))), []byte(Usr), []byte(Pwd))
	if err != nil {
    log.Errf("Establish data link to server %s failed, closing inbound %s: %w", Addr, util.ConnStr(inbound), err)
		util.CloseCloser(inbound)
		return
	}
	log.Infof("Established data link %s for inbound %s, relay starting. ", util.ConnStr(rconn), util.ConnStr(inbound))

	err = util.Relay(inbound, rconn)
	log.Info(fmt.Errorf("Relay finished for inbound %s, result: \n%w", util.ConnStr(inbound), err))
}
