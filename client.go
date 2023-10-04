package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"socksyx/ray"
	"strconv"
	"sync"

	x "socksyx"

	. "github.com/fishBone000/socksy5"
)

type controlLink struct {
	addr string
	neg  x.Subnegotiator

	conn   net.Conn
	capper Capsulator
	mux    sync.Mutex
}

func newCtrlLink(addr string, neg x.Subnegotiator) *controlLink {
	return &controlLink{
		addr: addr,
		neg:  neg,
	}
}

func (c *controlLink) getPort() (port uint16, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	for retry := 0; retry < 5; retry++ {
		if c.capper == nil {
			err = c.connect()
			if err != nil {
				logErr(x.NewOpErr("connect to socksyx", nil, err))
				continue
			}
		}

		port, err = c.queryPort()
		if err != nil {
			logErr(x.NewOpErr("query port failed, retrying", c.conn, err))
			c.cleanup()
		} else {
			return
		}
	}

	logErr(x.NewOpErr("failed to connect ctrl link after 5 tries", nil, nil))
	return
}

func (c *controlLink) cleanup() {
  closeCloser(c.conn)
	c.capper = nil
	c.conn = nil
}

func (c *controlLink) connect() error {
	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return err
	}

	capper, err := c.neg.Negotiate(conn)
	if err != nil {
		return err
	}

	c.conn = conn
	c.capper = capper
	return nil
}

// Only controlLink can call this method!
func (c *controlLink) queryPort() (uint16, error) {
	_, err := c.capper.Write([]byte{0x00})
	if err != nil {
		return 0, err
	}

	buf := make([]byte, 2)
	_, err = io.ReadFull(c.capper, buf)
	if err != nil {
		return 0, err
	}

	port := binary.BigEndian.Uint16(buf)
	return port, nil
}

func runClient() {
  logInfo(fmt.Errorf("client start up"))

	neg := &ray.RayNegotiator{
		Usr: []byte(Usr),
		Pwd: []byte(Pwd),
	}

	ctrl := newCtrlLink(net.JoinHostPort(Host, strconv.Itoa(Port)), neg)

  var l net.Listener
  for {
    var err error
    var inbound net.Conn
    for retry := 1; retry <= 5; retry++ {
      if l == nil {
        l, err = net.Listen("tcp", LAddr)
        if err != nil {
          logErr(fmt.Errorf("listen tcp failed, retrying %d/5: %w", 1, err))
          l = nil
          continue
        }
      }

      inbound, err = l.Accept()
      if err != nil {
        logErr(fmt.Errorf("accept tcp failed, retrying %d/5: %w", 1, err))
        closeCloser(l)
        l = nil
        continue
      }
      break
    }

    if err != nil {
      logErr(fmt.Errorf("failed after 5 tries: %w", err))
      os.Exit(1)
    }

    go serveInbound(inbound, ctrl)
  }
}

func serveInbound(inbound net.Conn, ctrl *controlLink) {
  logInfo(x.NewOpErr("new inbound", inbound, nil))

  mth, err := readMethods(inbound)
  if err != nil {
    closeCloser(inbound)
    logErr(x.NewOpErr("read methods", inbound, err))
    return
  }

  rep := make([]byte, 2)
  rep[0] = VerSOCKS5
  for _, m := range mth {
    if m == MethodNoAuth {
      rep[1] = MethodNoAuth
      break
    }
  }
  if _, err := inbound.Write(rep); err != nil {
    closeCloser(inbound)
    logErr(x.NewOpErr("reply method selection", inbound, err))
    return
  }

  if rep[1] == MethodNoAccepted {
    logWarn(x.NewOpErr("client does not support NO AUTH", inbound, nil))
    closeCloser(inbound)
    return
  }

  logInfo(x.NewOpErr("query available port for inbound", inbound, nil))
  port, err := ctrl.getPort()
  if err != nil {
    closeCloser(inbound)
    logErr(x.NewOpErr("get port for inbound", inbound, err))
    return
  }
  logInfo(x.NewOpErr(fmt.Sprintf("got port %d for inbound", port), inbound, nil))

  neg := &ray.RayNegotiator{
    Usr: []byte(Usr),
    Pwd: []byte(Pwd),
  }
  addr := net.JoinHostPort(Host, strconv.Itoa(int(port)))
  logInfo(x.NewOpErr(fmt.Sprintf("dialing server %s for inbound", addr), inbound, nil))
  cpConn, err := newCapConn(addr, neg)
  if err != nil {
    closeCloser(inbound)
    logErr(x.NewOpErr("dial to server for inbound", inbound, err))
    return
  }

  logInfo(x.NewOpErr("start relay for inbound", inbound, nil))
  err = relay(inbound, cpConn)
  logErr(x.NewOpErr("proxy", nil, err))
}
