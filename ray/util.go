package ray

import (
	"crypto/aes"
	"crypto/sha512"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"
)

// Does not check if the contentSz is valid!
func calcNBlock(contentSz int) int {
	nBlock := (2 + contentSz) / aes.BlockSize
	if (2+contentSz)%aes.BlockSize > 0 {
		nBlock++
	}

	return nBlock
}

func validateSum(p []byte) (ok bool) {
	sum := sha512.Sum512_256(p[:len(p)-sha512.Size256])
	return reflect.DeepEqual(sum[:], p[len(p)-sha512.Size256:])
}

type chanPipe struct {
	r <-chan byte
	w chan<- byte
}

func (pipe chanPipe) Write(p []byte) (int, error) {
	for _, b := range p {
		pipe.w <- b
	}
	return len(p), nil
}

func (pipe chanPipe) Read(p []byte) (int, error) {
	n := 0
Loop:
	for i := range p {
		select {
		case p[i] = <-pipe.r:
			n++
		default:
			break Loop
		}
	}
	return n, nil
}

func ChanPipe() (io.ReadWriter, io.ReadWriter) {
	ch1 := make(chan byte, 6*1024)
	ch2 := make(chan byte, 6*1024)
	return chanPipe{ch1, ch2}, chanPipe{ch2, ch1}
}
