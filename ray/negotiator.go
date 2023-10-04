package ray

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/sha512"
	"io"
	"reflect"
)

// Can be called simultaneously.
func Negotiate(rw io.ReadWriter, usr []byte, pwd []byte) (*Ray, error) {
	usum := sha512.Sum512_256(usr)
	psum := sha512.Sum512_256(pwd)
	mask := make([]byte, 32)
	copy(mask, usum[:16])
	copy(mask[16:], psum[:16])

	wkey := make([]byte, 32)
	if _, err := rand.Read(wkey); err != nil {
		panic(err)
	}

	msg := make([]byte, 32)
	copy(msg, wkey)
	for i := range msg {
		msg[i] ^= mask[i]
	}

	if _, err := rw.Write(msg); err != nil {
		return nil, err
	}

	if _, err := io.ReadFull(rw, msg); err != nil {
		return nil, err
	}

	rkey := make([]byte, 32)
	for i := range rkey {
		rkey[i] = mask[i] ^ msg[i]
	}

	wblock, err := aes.NewCipher(wkey)
	if err != nil {
		return nil, err
	}
	rblock, err := aes.NewCipher(rkey)
	if err != nil {
		return nil, err
	}

	copy(msg, mask)
	wblock.Encrypt(msg, msg)
	wblock.Encrypt(msg[aes.BlockSize:], msg[aes.BlockSize:])

	if _, err := rw.Write(msg); err != nil {
		return nil, err
	}

	if _, err := io.ReadFull(rw, msg); err != nil {
		return nil, err
	}

	rblock.Decrypt(msg, msg)
	rblock.Decrypt(msg[aes.BlockSize:], msg[aes.BlockSize:])
	if !reflect.DeepEqual(msg, mask) {
		return nil, ErrAuthFailed
	}

	return &Ray{
		rblock: rblock,
		wblock: wblock,
		rw:     rw,
	}, nil
}
