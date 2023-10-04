package ray

import "net"

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
    ray: ray,
  }, nil
}

func Dial(network string, addr string, usr, pwd []byte) (*RayConn, error) {
  conn, err := net.Dial(network, addr)
  if err != nil {
    return nil, err
  }

  ray, err := Negotiate(conn, usr, pwd)
  if err != nil {
    return nil, err
  }

  return &RayConn{
    Conn: conn,
    ray: ray,
  }, nil
}

func (rc *RayConn) Read(p []byte) (n int, err error) {
  return rc.ray.Read(p)
}

func (rc *RayConn) Write(p []byte) (n int, err error) {
  return rc.ray.Write(p)
}
