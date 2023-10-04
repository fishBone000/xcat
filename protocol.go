package main

import (
	"io"

	. "socksyx"
)

type request []byte

func newRequest(cmd byte, addr string) (request, error) {
	addrPort, err := ParseAddrPort(addr)
  if err != nil {
    return nil, err
  }
  rawAddr, err := addrPort.MarshalBinary()
  if err != nil {
    return nil, err
  }

  req := make(request, 3+len(rawAddr))
  copy(req[3:], rawAddr)
  req[0] = VerSOCKS5
  req[1] = cmd
  req[2] = RSV

  return req, nil
}

type reply struct {
  code byte
  bnd *AddrPort
}

func readReply(r io.Reader) (*reply, error) {
  buf := make([]byte, 3)
  _, err := io.ReadFull(r, buf)
  if err != nil {
    return nil, err
  }

  if buf[0] != VerSOCKS5 {
    return nil, VerIncorrectError(buf[0])
  }
  if buf[2] != RSV {
    return nil, RsvViolationError(buf[2])
  }

  addrPort, err := ReadAddrPort(r)
  if err != nil {
    return nil, err
  }

  rep := new(reply)
  rep.code = buf[1]
  rep.bnd = addrPort
  return rep, nil
}

func postRequest(cmd byte, addr string, rw io.ReadWriter) (*reply, error) {
  req, err := newRequest(cmd, addr)
  if err != nil {
    return nil, err
  }

  if _, err := rw.Write(req); err != nil {
    return nil, err
  }

  rep, err := readReply(rw)
  if err != nil {
    return nil, err
  }
  return rep, nil
}

type methods []byte

func readMethods(r io.Reader) (methods, error) {
  buf := make([]byte, 2)
  if _, err := io.ReadFull(r, buf); err != nil {
    return nil, err
  }

  if buf[0] != VerSOCKS5 {
    return nil, VerIncorrectError(buf[0])
  }

  res := make(methods, buf[1])
  if _, err := io.ReadFull(r, res); err != nil {
    return nil, err
  }

  return res, nil
}
