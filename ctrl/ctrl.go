package ctrl

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/fishBone000/xcat/log"
	"github.com/fishBone000/xcat/ray"
	"github.com/fishBone000/xcat/util"
)

const (
	ConnectRetries = 5
	GetPortRetries = 5
)

type ControlLink struct {
	addr string
	usr  []byte
	pwd  []byte

	rconn *ray.RayConn
	mux   sync.Mutex
}

func NewCtrlLink(addr string, usr, pwd []byte) (*ControlLink, error) {
	ctrl := &ControlLink{
		addr: addr,
		usr:  usr,
		pwd:  pwd,
	}

	err := ctrl.connect()
	if err != nil {
		return nil, err
	}
	return ctrl, nil
}

func (c *ControlLink) GetPort() (port uint16, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	log.Debug(fmt.Sprintf("ctrl link %s: getting port. ", c.addr))

	for retry := 0; retry <= GetPortRetries; retry++ {
		if retry != 0 {
			log.Err(fmt.Errorf(
				"ctrl link %s: Query port failed, retrying %d/%d. ",
				c.addr, retry, GetPortRetries,
			))
		}

		err = c.connectNoLock()
		if err != nil {
			log.Err(fmt.Errorf("ctrl link %s: Aborted due to connection failure. ", c.addr))
			return
		}

		_, err = c.rconn.Write([]byte{0x00})
		if err != nil {
			c.rconn = nil
			log.Err(fmt.Errorf("ctrl link %s: Couldn't send port query: %w. ", c.addr, err))
			continue
		}

		buf := make([]byte, 2)
		_, err = io.ReadFull(c.rconn, buf)
		if err != nil {
			c.rconn = nil
			log.Err(fmt.Errorf("ctrl link %s: Couldn't get port: %w. ", c.addr, err))
			continue
		}

		port = binary.BigEndian.Uint16(buf)

		if err == nil {
			log.Debug(fmt.Sprintf("ctrl link %s: Got port %d. ", c.addr, port))
			return
		}
	}

	log.Err(fmt.Sprintf("ctrl link %s: Failed to get port after %d retries. ", c.addr, GetPortRetries))
	return
}

func (c *ControlLink) connect() (err error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	return c.connectNoLock()
}

func (c *ControlLink) connectNoLock() (err error) {
	if c.rconn != nil {
		return nil
	}

	for retry := 0; retry <= ConnectRetries; retry++ {
		if retry != 0 {
			log.Err(fmt.Errorf(
				"ctrl link %s: connect failed, retrying %d/%d: %w. ",
				c.addr, retry, ConnectRetries, err,
			))
		}

		c.rconn, err = ray.Dial("tcp", c.addr, c.usr, c.pwd)
		if err != nil {
			continue
		}
    break
	}

	if err != nil {
		c.rconn = nil
		log.Err(fmt.Errorf(
			"ctrl link %s: Failed to connect after %d retries: %w", 
      c.addr, ConnectRetries, err,
		))
		return err
	}
	log.Info("ctrl link " + c.addr + ": Connect successful: " + util.ConnStr(c.rconn) + ". ")
	return nil
}
