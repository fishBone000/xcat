package ray

import (
	"io"
	"reflect"
	"sync"
	"testing"
)

func FuzzChanPipe(f *testing.F) {
  f.Add([]byte{0x00, 0x01, 0x05}, []byte{0x00, 0xFF, 0x88})

  f.Fuzz(func(t *testing.T, a []byte, b []byte) {
    rwA, rwB := ChanPipe()
    wg := sync.WaitGroup{}
    wg.Add(2)

    var errAB, errBA error

    go func() {
      rwA.Write(a)
    }()
    go func() {
      rwB.Write(b)
    }()
    go func() {
      defer wg.Done()
      buf := make([]byte, len(a))
      io.ReadFull(rwB, buf)
      if !reflect.DeepEqual(buf, a) {
        errAB = ErrIntegrityCompromised
        t.Logf("\nAB integrity compromised\nexpect: %#v\ngot: %#v\n", a, buf)
      }
    }()
    go func() {
      defer wg.Done()
      buf := make([]byte, len(b))
      io.ReadFull(rwA, buf)
      if !reflect.DeepEqual(buf, b) {
        errBA = ErrIntegrityCompromised
        t.Logf("\nBA integrity compromised\nexpect: %#v\ngot: %#v\n", b, buf)
      }
    }()

    wg.Wait()

    if errAB != nil || errBA != nil {
      t.Fatalf("\nAB: %s\nBA: %s\n", errAB, errBA)
    }
  })
}
