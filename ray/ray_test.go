package ray

import (
	"crypto/aes"
	"errors"
	"io"
	"reflect"
	"sync"
	"testing"
)

func FuzzRayCap(f *testing.F) {
  f.Add([]byte{0x00, 0xFF}, []byte{0x00, 0xFF})

  f.Fuzz(func(t *testing.T, key []byte, b []byte) {
    if len(key) < 32 {
      return
    }
    if len(b) > 65584 {
      return
    }

    block, err := aes.NewCipher(key[:32])
    if err != nil {
      panic(err)
    }

    r := &Ray{
      rblock: block,
      wblock: block,
    }

    packet, err := r.EncapPacket(b)
    if err != nil {
      panic(err)
    }
    data, err := r.DecapPacket(packet)
    if err != nil {
      panic(err)
    }
    if !reflect.DeepEqual(b, data) {
      t.Log("data integrity compromised")
      t.Logf("\nwant: % 2X", b)
      t.Logf("\ngot: % 2X", data)
      t.FailNow()
    }
  })
}

func FuzzRayRW(f *testing.F) {
  f.Add([]byte{0x00, 0x88}, []byte{0x88, 0x00}, []byte{0x00, 0x00, 0x00})

  f.Fuzz(func(t *testing.T, usr []byte, pwd []byte, data []byte) {
    t.Log("begin subnegotiation")

    rwA, rwB := ChanPipe()

    var capperA, capperB *Ray
    var errA, errB error
    wg := sync.WaitGroup{}

    wg.Add(2)
    go func() {
      capperA, errA = Negotiate(rwA, usr, pwd)
      wg.Done()
    }()
    go func() {
      capperB, errB = Negotiate(rwB, usr, pwd)
      wg.Done()
    }()
    wg.Wait()
    t.Log("subnegotiation finished")

    if errA != nil || errB != nil {
      t.Fatalf("error negotiating\nerror A: %s\nerror B: %s\n", errA, errB)
    }

    t.Log("begin data transmission")
    wg.Add(4)
    var errAR, errAW, errBR, errBW error
    errA = nil
    errB = nil
    go func() {
      _, errAW = capperA.Write(data)
      wg.Done()
      t.Log("done writing A")
    }()
    go func() {
      _, errBW = capperB.Write(data)
      wg.Done()
      t.Log("done writing B")
    }()
    go func() {
      defer wg.Done()
      defer t.Log("done reading A")
      buf := make([]byte, len(data))
      _, errAR = io.ReadFull(capperA, buf)
      if errAR != nil {
        return
      }
      if !reflect.DeepEqual(buf, data) {
        errA = ErrIntegrityCompromised
      }
    }()
    go func() {
      defer wg.Done()
      defer t.Log("done reading A")
      buf := make([]byte, len(data))
      _, errBR = io.ReadFull(capperB, buf)
      if errBR != nil {
        return
      }
      if !reflect.DeepEqual(buf, data) {
        errB = ErrIntegrityCompromised
      }
    }()
    wg.Wait()
    t.Log("data transmission finished")

    err := errors.Join(errA, errB, errAR, errBR, errAW, errBW)
    if err != nil {
      t.Logf("A read: %s\n", errAR)
      t.Logf("B read: %s\n", errBR)
      t.Logf("A write: %s\n", errAW)
      t.Logf("B write: %s\n", errBW)
      t.Logf("A error: %s\n", errA)
      t.Logf("B error: %s\n", errB)
      t.FailNow()
    }
  })
}
