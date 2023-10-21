package ctrl

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

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
  timeout time.Duration

	rconn *ray.RayConn
	mux   sync.Mutex
}

func NewCtrlLink(addr string, usr, pwd []byte, timeout time.Duration) (*ControlLink, error) {
	ctrl := &ControlLink{
		addr: addr,
		usr:  usr,
		pwd:  pwd,
    timeout: timeout,
	}

	err := ctrl.connect()
	if err != nil {
		return nil, err
	}
	return ctrl, nil
}

func (c *ControlLink) GetPortTCP() (port uint16, err error) {
  return c.getPort(0x00)
}

func (c *ControlLink) GetPortUDP() (port uint16, err error) {
  return c.getPort(0x01)
}

func (c *ControlLink) getPort(msg byte) (port uint16, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	log.Debug("ctrl link: Getting port. ")

	for retry := 0; retry <= GetPortRetries; retry++ {
		if retry != 0 {
			log.Err(fmt.Errorf(
				"ctrl link: Query port failed, retrying %d/%d. ",
				retry, GetPortRetries,
			))
		}

		err = c.connectNoLock()
		if err != nil {
			log.Err("ctrl link: Aborted due to connection failure. ")
			return
		}

    err = c.rconn.SetDeadline(time.Now().Add(c.timeout))
    if err != nil {
      log.Warnf("ctrl link: Failed to set deadline: %w. ", err)
    }

		_, err = c.rconn.Write([]byte{msg})
		if err != nil {
			c.rconn = nil
			log.Err(fmt.Errorf("ctrl link: Couldn't send port query: %w. ", err))
			continue
		}

		buf := make([]byte, 2)
		_, err = io.ReadFull(c.rconn, buf)
		if err != nil {
			c.rconn = nil
			log.Err(fmt.Errorf("ctrl link: Couldn't get port: %w. ", err))
			continue
		}

		port = binary.BigEndian.Uint16(buf)

		if err == nil {
			log.Debug(fmt.Sprintf("ctrl link: Got port %d. ", port))
			return
		}
	}

	log.Err(fmt.Sprintf("ctrl link: Failed to get port after %d retries. ", GetPortRetries))
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
				"ctrl link: connect failed, retrying %d/%d: %w. ",
				retry, ConnectRetries, err,
			))
		}

		c.rconn, err = ray.DialTimeout("tcp", c.addr, c.usr, c.pwd, c.timeout)
		if err != nil {
			continue
		}
    break
	}

	if err != nil {
		c.rconn = nil
		log.Err(fmt.Errorf(
			"ctrl link: Failed to connect after %d retries: %w", 
      ConnectRetries, err,
		))
		return err
	}
	log.Info("ctrl link " + c.addr + ": Connect successful: " + util.ConnStr(c.rconn) + ". ")
	return nil
}
