package ray

import (
	"net"
	"time"
)

type RayConn struct {
	net.Conn
	ray *Ray
}

func FromConn(conn net.Conn, usr, pwd []byte) (*RayConn, error) {
	ray, err := Negotiate(conn, usr, pwd)
	if err != nil {
		return nil, err
	}
	return &RayConn{
		Conn: conn,
		ray:  ray,
	}, nil
}

func Dial(network string, addr string, usr, pwd []byte) (*RayConn, error) {
  return DialTimeout(network, addr, usr, pwd, 0)
}

func DialTimeout(network string, addr string, usr, pwd []byte, d time.Duration) (*RayConn, error) {
  ddl := time.Now().Add(d)
	conn, err := net.DialTimeout(network, addr, d)
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
		ray:  ray,
	}, nil
}

func (rc *RayConn) Read(p []byte) (n int, err error) {
	return rc.ray.Read(p)
}

func (rc *RayConn) Write(p []byte) (n int, err error) {
	return rc.ray.Write(p)
}
