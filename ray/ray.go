package ray

// # Design of Ray Encryption Standard (RES)
//
// Standard? It's just a encapsulation of AES!
//
// ## Negotiation
//
// Skipped
//
// ## Encryption
//
// A packet looks like this:
//
// |  BLK #0  |  BLK #1  |          |  BLK #N  |
// +-----------------------  ...  ---------------------+
// | SZ |                  CONTENT             |  SUM  |
// +-----------------------  ...  ---------------------+
//   2                       VAR                  32
//
// Where:
// SZ is the size of CONTENT.
// BLK is an AES block, having size of 16 bytes.
// SUM is the sha512/256 checksum of the plaintext form of SZ and CONTENT.
//
// CONTENT is zero padded at the tail to fit to the AES block size.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// PacketTooLargeError is returned by [Ray.EncapPacket], indicating that
// the packet is too large to suit in Ray's capsulation.
type PacketTooLargeError int

func (e PacketTooLargeError) Error() string {
	return fmt.Sprintf("packet too large (%d Bytes)", int(e))
}

type IncorrectPacketSizeError int

func (e IncorrectPacketSizeError) Error() string {
	return fmt.Sprintf("incorrect packet size (%d Bytes)", int(e))
}

var ErrIntegrityCompromised = errors.New("data integrity compromised")

const MaxPlaintextSize = 0xFFFF

// All methods can be called simultaneously.
type Ray struct {
	rblock    cipher.Block
	decapMux  sync.Mutex
	wblock    cipher.Block
	encapMux  sync.Mutex
	rw        io.ReadWriter
	wmux      sync.Mutex
	rmux      sync.Mutex
	incmplWrt []byte
	// Note that len of this slice represents the number of bytes read,
	// while cap of this slice represents total number of bytes to finish this
	// imcomplete read.
	// If this slice is not nil and cap of this slice is 16, it is considered that
	// the first block is not yet read, therefore needs to be decyphered first to
	// determine the total size of the incomplete packet.
	// After finish
	incmplRPacket []byte
	// Decyphered data is stored here if the buffer provided in the last Read call
	// was too small.
	rbuffer []byte
	rFatal  error
}

func (r *Ray) Read(p []byte) (n int, err error) {
	r.rmux.Lock()
	defer r.rmux.Unlock()

	if r.rFatal != nil {
		return 0, r.rFatal
	}

	if r.rbuffer != nil {
		n = copy(p, r.rbuffer)
		r.rbuffer = r.rbuffer[n:]
		if len(r.rbuffer) == 0 {
			r.rbuffer = nil
		}
		return
	}

	if r.incmplRPacket != nil {
		if cap(r.incmplRPacket) == aes.BlockSize {
			var n2 int
			n2, err = io.ReadFull(r.rw, r.incmplRPacket[len(r.incmplRPacket):cap(r.incmplRPacket)])
			r.incmplRPacket = r.incmplRPacket[:len(r.incmplRPacket)+n2]

			if len(r.incmplRPacket) != cap(r.incmplRPacket) {
				return
			}

			r.decryptBlock(r.incmplRPacket, 0)
			sz := int(binary.BigEndian.Uint16(r.incmplRPacket[:2]))
			nBlock := calcNBlock(sz)

			extend := make([]byte, aes.BlockSize, nBlock*aes.BlockSize+sha512.Size256)
			copy(extend, r.incmplRPacket)
			r.incmplRPacket = extend

			if err != nil {
				return
			}
		}

		var n2 int
		n2, err = io.ReadFull(r.rw, r.incmplRPacket[len(r.incmplRPacket):cap(r.incmplRPacket)])
		r.incmplRPacket = r.incmplRPacket[:len(r.incmplRPacket)+n2]

		if len(r.incmplRPacket) != cap(r.incmplRPacket) {
			return
		}

		r.decryptPacket(r.incmplRPacket, 1)
		if !validateSum(r.incmplRPacket) {
			r.incmplRPacket = nil
			r.rFatal = ErrIntegrityCompromised
			return 0, r.rFatal
		}
		sz := int(binary.BigEndian.Uint16(r.incmplRPacket[:2]))
		msg := r.incmplRPacket[2 : 2+sz]
		r.incmplRPacket = nil
		n = copy(p, msg)
		msg = msg[n:]
		if len(msg) != 0 {
			r.rbuffer = msg
		}
		r.incmplRPacket = nil
		return
	}

	var n2 int
	firstBlock := make([]byte, aes.BlockSize)
	n2, err = io.ReadFull(r.rw, firstBlock)
	if n2 != aes.BlockSize {
		r.incmplRPacket = firstBlock[:n2]
		return
	}
	r.decryptBlock(firstBlock, 0)

	sz := int(binary.BigEndian.Uint16(firstBlock[:2]))
	nBlock := calcNBlock(sz)
	packet := make([]byte, nBlock*aes.BlockSize+sha512.Size256)
	copy(packet, firstBlock)

	n2, err = io.ReadFull(r.rw, packet[aes.BlockSize:])
	if aes.BlockSize+n2 != len(packet) {
		r.incmplRPacket = packet[:aes.BlockSize+n2]
		return
	}

	r.decryptPacket(packet, 1)
	if !validateSum(packet) {
		r.rFatal = ErrIntegrityCompromised
		return n, r.rFatal
	}

	content := packet[2 : 2+sz]
	n = copy(p, content)
	if n != len(content) {
		r.rbuffer = content[n:]
	}
	return
}

func (r *Ray) Write(p []byte) (n int, err error) {
	r.wmux.Lock()
	defer r.wmux.Unlock()
	if r.incmplWrt != nil {
		var n2 int
		n2, err = r.rw.Write(r.incmplWrt)
		r.incmplWrt = r.incmplWrt[n2:]
		if len(r.incmplWrt) == 0 {
			r.incmplWrt = nil
		} else {
			return
		}
	}

	for {
		sz := len(p[n:])
		if sz == 0 {
			return
		}
		if sz > MaxPlaintextSize {
			sz = MaxPlaintextSize
		}

		var msg []byte
		msg, _ = r.EncapPacket(p[n : n+sz])

		var n2 int
		n2, err = r.rw.Write(msg)
		if n2 != len(msg) {
			r.incmplWrt = msg[n2:]
			return
		}
		n += sz
		if err != nil {
			return
		}
	}
}

func (r *Ray) EncapPacket(p []byte) ([]byte, error) {
	sz := len(p)
	if sz > 0xFFFF {
		return nil, PacketTooLargeError(sz)
	}

	nBlock := calcNBlock(sz)

	result := make([]byte, nBlock*aes.BlockSize+sha512.Size256)

	binary.BigEndian.PutUint16(result[:2], uint16(sz))
	copy(result[2:], p)
	sum := sha512.Sum512_256(result[:nBlock*aes.BlockSize])
	copy(result[nBlock*aes.BlockSize:], sum[:])
	r.encryptPacket(result, 0)

	return result, nil
}

func (r *Ray) DecapPacket(p []byte) ([]byte, error) {
	// ceil((2+0xFFFF) / aes.BlockSize) == 4097
	// 4097*aes.BlockSize + sha512.Size256 == 65584
	if len(p) > 65584 {
		return nil, PacketTooLargeError(len(p))
	}
	if len(p)%aes.BlockSize != 0 {
		return nil, IncorrectPacketSizeError(len(p))
	}

	result := make([]byte, len(p))
	copy(result, p)

	r.decryptBlock(result, 0)
	sz := int(binary.BigEndian.Uint16(result[:2]))

	nBlock := calcNBlock(sz)

	if len(p) != nBlock*aes.BlockSize+sha512.Size256 {
		return nil, IncorrectPacketSizeError(len(p))
	}

	r.decryptPacket(result, 1)

	if !validateSum(result) {
		return nil, ErrIntegrityCompromised
	}

	return result[2 : 2+sz], nil
}

func (r *Ray) decryptPacket(p []byte, begin int) {
	r.decapMux.Lock()
	defer r.decapMux.Unlock()

	if len(p)%aes.BlockSize != 0 {
		panic(IncorrectPacketSizeError(len(p)))
	}

	for i := begin; i*aes.BlockSize < len(p); i++ {
		r.rblock.Decrypt(p[i*aes.BlockSize:], p[i*aes.BlockSize:])
	}
}

func (r *Ray) decryptBlock(p []byte, i int) {
	r.decapMux.Lock()
	defer r.decapMux.Unlock()

	r.rblock.Decrypt(p[i*aes.BlockSize:], p[i*aes.BlockSize:])
}

func (r *Ray) encryptPacket(p []byte, begin int) {
	r.encapMux.Lock()
	defer r.encapMux.Unlock()

	if len(p)%aes.BlockSize != 0 {
		panic(IncorrectPacketSizeError(len(p)))
	}

	for i := begin; i*aes.BlockSize < len(p); i++ {
		r.wblock.Encrypt(p[i*aes.BlockSize:], p[i*aes.BlockSize:])
	}
}

func (r *Ray) encryptBlock(p []byte, i int) {
	r.encapMux.Lock()
	defer r.encapMux.Unlock()

	r.wblock.Encrypt(p[i*aes.BlockSize:], p[i*aes.BlockSize:])
}
