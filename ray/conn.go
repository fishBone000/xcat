package ray

import (
	"errors"
	"net"
	"time"

	"github.com/fishBone000/xcat/util"
)

type RayConn struct {
	net.Conn
	Ray *Ray
}

func FromConn(conn net.Conn, usr, pwd []byte) (*RayConn, error) {
	ray, err := Negotiate(conn, usr, pwd)
	if err != nil {
		return nil, err
	}
	return &RayConn{
		Conn: conn,
		Ray:  ray,
	}, nil
}

func Dial(network string, addr string, usr, pwd []byte) (*RayConn, error) {
	return DialTimeout(network, addr, usr, pwd, 0)
}

func DialTimeout(network string, addr string, usr, pwd []byte, d time.Duration) (*RayConn, error) {
	ddl := time.Now().Add(d)
	dialer := &net.Dialer{
		Timeout:   d,
		KeepAlive: 10 * time.Second,
	}
	conn, err := dialer.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	if d > 0 {
		err = conn.SetDeadline(ddl)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	ray, err := Negotiate(conn, usr, pwd)
	if err != nil {
		return nil, err
	}

	if d > 0 {
		err = conn.SetDeadline(time.Time{})
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	return &RayConn{
		Conn: conn,
		Ray:  ray,
	}, nil
}

func (rc *RayConn) Read(p []byte) (n int, err error) {
	return rc.Ray.Read(p)
}

func (rc *RayConn) Write(p []byte) (n int, err error) {
	return rc.Ray.Write(p)
}

type RayUDP struct {
	UDP    net.Conn
	TCP    net.Conn
	Ray    *Ray
  laddr net.Addr
  raddr net.Addr
	errTCP util.Fatal
}

func NewRayUDP(u net.Conn, t net.Conn, r *Ray) *RayUDP {
  ru := &RayUDP{
    TCP: t,
    UDP: u,
    Ray: r,
    laddr: util.NewStrAddr("ray udp", t.LocalAddr().String()),
    raddr: util.NewStrAddr("ray udp", t.RemoteAddr().String()),
  }

  go func() {
    for {
      buffer := make([]byte, 1)
      _, err := ru.TCP.Read(buffer)
      if err != nil {
        ru.errTCP.Set(err)
        ru.TCP.Close()
        ru.UDP.Close()
        return
      }
    }
  }()

  return ru
}

func DialUDP(network, addr string, usr, pwd []byte) (*RayUDP, error) {
	return DialTimeoutUDP(network, addr, usr, pwd, 0)
}

func DialTimeoutUDP(network, addr string, usr, pwd []byte, d time.Duration) (*RayUDP, error) {
	var nwTcp string
	switch network {
	case "udp":
		nwTcp = "tcp"
	case "udp4":
		nwTcp = "tcp4"
	case "udp6":
		nwTcp = "tcp6"
	default:
		return nil, net.UnknownNetworkError(network)
	}

  ddl := time.Now().Add(d)
	dialer := net.Dialer{
		Deadline:   ddl,
		KeepAlive: 10 * time.Second,
	}

	tcp, err := dialer.Dial(nwTcp, addr)
	if err != nil {
		return nil, err
	}

  dialer.LocalAddr, _ = net.ResolveUDPAddr(network, tcp.LocalAddr().String())
	udp, err := dialer.Dial(network, tcp.RemoteAddr().String())
	if err != nil {
		tcp.Close()
		return nil, err
	}

  if err := tcp.SetDeadline(ddl); err != nil {
    tcp.Close()
    udp.Close()
    return nil, err
  }
	ray, err := Negotiate(tcp, usr, pwd)
	if err != nil {
		tcp.Close()
    udp.Close()
		return nil, err
	}
  if err := tcp.SetDeadline(time.Time{}); err != nil {
    tcp.Close()
    udp.Close()
    return nil, err
  }

  ru := NewRayUDP(udp, tcp, ray)

	return ru, nil
}

func ListenRayUDPTimeout(network, addr string, usr, pwd []byte, d time.Duration) (*RayUDP, error) {
  var nwTcp string
  switch network {
  case "udp":
    nwTcp = "tcp"
  case "udp4":
    nwTcp = "tcp4"
  case "udp6":
    nwTcp = "tcp6"
  default:
    return nil, net.UnknownNetworkError(network)
  }

  ddl := time.Now().Add(d)
  laddrTcp, err := net.ResolveTCPAddr(nwTcp, addr)
  if err != nil {
    return nil, err
  }
  lc, err := net.ListenTCP(nwTcp, laddrTcp)
  if err != nil {
    return nil, err
  }
  
  if err := lc.SetDeadline(ddl); err != nil {
    return nil, err
  }
  
  tcp, err := lc.Accept()
  if err != nil {
    lc.Close()
    return nil, err
  }
  
  lc.Close()
  laddrUdp, _ := net.ResolveUDPAddr(network, tcp.LocalAddr().String())
  raddrUdp, _ := net.ResolveUDPAddr(network, tcp.RemoteAddr().String())
  udp, err := net.DialUDP(network, laddrUdp, raddrUdp)
  if err != nil {
    tcp.Close()
    return nil, err
  }

  if err := tcp.SetDeadline(ddl); err != nil {
    tcp.Close()
    udp.Close()
    return nil, err
  }
  ray, err := Negotiate(tcp, usr, pwd)
  if err != nil {
    tcp.Close()
    udp.Close()
    return nil, err
  }
  if err := tcp.SetDeadline(time.Time{}); err != nil {
    tcp.Close()
    udp.Close()
    return nil, err
  }

  return &RayUDP{
    TCP: tcp,
    UDP: udp,
    Ray: ray,
  }, nil
}

func (r *RayUDP) Read(b []byte) (n int, err error) {
	n, err = r.UDP.Read(b)
	if n > 0 {
		p, dcErr := r.Ray.DecapPacket(b[:n])
		if dcErr != nil {
			return 0, errors.Join(dcErr, err)
		}
		n = copy(b, p)
	}
	return
}

func (r *RayUDP) Write(b []byte) (n int, err error) {
	var p []byte
	p, err = r.Ray.EncapPacket(b)
	if err != nil {
		return
	}
	n, err = r.UDP.Write(p)
	if err != nil {
	}
	return
}

func (r *RayUDP) LocalAddr() net.Addr { // TODO Improper! In this way the network would be "udp"
	return r.UDP.LocalAddr()
}

func (r *RayUDP) RemoteAddr() net.Addr {
	return r.UDP.RemoteAddr()
}

func (r *RayUDP) Close() error {
	e1 := r.TCP.Close()
	e2 := r.UDP.Close()
	return errors.Join(e1, e2)
}

func (r *RayUDP) SetDeadline(t time.Time) error {
	return r.UDP.SetDeadline(t)
}

func (r *RayUDP) SetReadDeadline(t time.Time) error {
	return r.UDP.SetReadDeadline(t)
}

func (r *RayUDP) SetWriteDeadline(t time.Time) error {
	return r.UDP.SetWriteDeadline(t)
}

func (r *RayUDP) ErrTCP() error {
  return r.errTCP.Get()
}
