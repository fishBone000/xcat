package ray

import (
	"errors"
	"net"
	"sync"
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
	udp          *net.UDPConn
	preconnected bool
	tcp          net.Conn
	ray          *Ray
	laddr        net.Addr
	raddr        net.Addr
	mux          sync.Mutex
	errTCP       util.Fatal
}

func NewRayUDP(u *net.UDPConn, preconnected bool, t net.Conn, r *Ray) *RayUDP {
	ru := &RayUDP{
		tcp:          t,
		udp:          u,
		ray:          r,
		preconnected: preconnected,
		laddr:        util.NewStrAddr("ray udp", t.LocalAddr().String()),
	}
	if raddr := u.RemoteAddr(); raddr != nil {
		ru.raddr = raddr
	}

	go func() {
		for {
			buffer := make([]byte, 1)
			_, err := ru.tcp.Read(buffer)
			if err != nil {
				ru.errTCP.Set(err)
				ru.tcp.Close()
				ru.udp.Close()
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
		Deadline:  ddl,
		KeepAlive: 10 * time.Second,
	}

	tcp, err := dialer.Dial(nwTcp, addr)
	if err != nil {
		return nil, err
	}

	raddr, _ := net.ResolveUDPAddr("udp", tcp.RemoteAddr().String())
	udp, err := net.DialUDP(network, nil, raddr)
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

	ru := NewRayUDP(udp, true, tcp, ray)

	return ru, nil
}

// Deprecated. Check code before use.
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
		tcp: tcp,
		udp: udp,
		ray: ray,
	}, nil
}

func (r *RayUDP) Read(b []byte) (n int, err error) {
	var addr net.Addr
	for {
		n, addr, err = r.udp.ReadFrom(b)
		if addr == nil {
			if n == 0 && err != nil {
				return
			}
			panic("IMPOSSIBLE! net.UDPConn.ReadFrom returned unexpected values. ")
		}
		r.mux.Lock()
		if r.raddr == nil {
			r.raddr = addr
			r.mux.Unlock()
			break
		} else {
			r.mux.Unlock()
			if r.raddr.String() == addr.String() {
				break
			}
		}
	}

	if n > 0 {
		p, dcErr := r.ray.DecapPacket(b[:n])
		if dcErr != nil {
			return 0, errors.Join(dcErr, err)
		}
		n = copy(b, p)
	}
	return
}

func (r *RayUDP) Write(b []byte) (n int, err error) {
	var p []byte
	p, err = r.ray.EncapPacket(b)
	if err != nil {
		return
	}
	if r.preconnected {
		return r.udp.Write(p)
	}
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.udp.WriteTo(p, r.raddr)
}

func (r *RayUDP) LocalAddr() net.Addr { // FIXME Improper! In this way the network would be "udp"
	return r.udp.LocalAddr()
}

func (r *RayUDP) RemoteAddr() net.Addr {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.raddr
}

func (r *RayUDP) Close() error {
	e1 := r.tcp.Close()
	e2 := r.udp.Close()
	return errors.Join(e1, e2)
}

func (r *RayUDP) SetDeadline(t time.Time) error {
	return r.udp.SetDeadline(t)
}

func (r *RayUDP) SetReadDeadline(t time.Time) error {
	return r.udp.SetReadDeadline(t)
}

func (r *RayUDP) SetWriteDeadline(t time.Time) error {
	return r.udp.SetWriteDeadline(t)
}

func (r *RayUDP) ErrTCP() error {
	return r.errTCP.Get()
}
