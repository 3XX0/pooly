package pooly

import (
	"net"
	"time"
)

type NetDriver struct {
	network string
	address string
	timeout time.Duration
}

func NewNetDriver(network, address string) *NetDriver {
	return &NetDriver{
		network: network,
		address: address,
	}
}

func (n *NetDriver) SetTimeout(timeout time.Duration) {
	n.timeout = timeout
}

func (n *NetDriver) Dial() (*Conn, error) {
	var c net.Conn
	var err error

	if n.timeout > 0 {
		c, err = net.DialTimeout(n.network, n.address, n.timeout)
	} else {
		c, err = net.Dial(n.network, n.address)
	}
	return NewConn(c), err
}

func (n *NetDriver) Close(c *Conn) {
	_ = c.NetConn().Close()
}

func (n *NetDriver) TestOnBorrow(c *Conn) error {
	return nil
}

func (n *NetDriver) Temporary(err error) bool {
	if e, ok := err.(net.Error); ok {
		return e.Temporary()
	}
	return false
}
