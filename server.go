package main

import (
	"fmt"
	"net"
	"strconv"
	"time"

	. "socksyx"
	"socksyx/ray"
)

func runServer() {
  logInfo(fmt.Errorf("server startup"))

	ml := &MidLayer{
		Usr: []byte(Usr),
		Pwd: []byte(Pwd),
	}

	go dispatchLogS(ml)
	go handleInboundsS(ml)
	go handleRequestsS(ml)

	l := &Listener{
		Usr:   []byte(Usr),
		Pwd:   []byte(Pwd),
		Neg:   &ray.RayNegotiator{Usr: []byte(Usr), Pwd: []byte(Pwd)},
		Layer: ml,
    Timeout: time.Minute,
	}

  err := l.Serve(net.JoinHostPort(Host, strconv.Itoa(Port)))
  logErr(NewOpErr("listener is down", nil, err))
}

func dispatchLogS(ml *MidLayer) {
	ch := ml.LogChan()
	for {
		log := <-ch
    fmt.Println("midlayer: "+log.String())
	}
}

func handleRequestsS(ml *MidLayer) {
	ch := ml.RequestChan()
	binder := &Binder{
		Hostname: Host,
	}
	assoc := &Associator{
		Hostname: Host,
	}
	for {
		req := <-ch
		switch req := req.(type) {
		case *ConnectRequest:
			err := Connect(req)
			if err != nil {
				logErr(NewOpErr("handle CONNECT", nil, err))
			}
		case *BindRequest:
			go func() {
				err := binder.Handle(req, "", time.Minute)
				if err != nil {
					logErr(NewOpErr("handle BIND", nil, err))
				}
			}()
		case *AssocRequest:
			go func() {
				err := assoc.Handle(req, "")
				if err != nil {
					logErr(NewOpErr("handle ASSOC", nil, err))
				}
			}()
		}
	}
}

func handleInboundsS(ml *MidLayer) {
	ch := ml.HandshakeChan()
	for {
		in := <-ch
		in.Accept(&ray.RayNegotiator{
			Usr: []byte(Usr),
			Pwd: []byte(Pwd),
		})
	}
}
