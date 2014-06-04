package pooly

import (
	"net"
	"time"
)

// NetDriver is a predefined driver for handling standard net.Conn objects.
type NetDriver struct {
	network string
	timeout time.Duration
}

// NewNetDriver instantiates a new NetDriver, ready to be used in a PoolConfig.
func NewNetDriver(network string) *NetDriver {
	return &NetDriver{network: network}
}

// SetTimeout sets the dialing timeout on a net.Conn object.
func (n *NetDriver) SetTimeout(timeout time.Duration) {
	n.timeout = timeout
}

// Dial is analogous to net.Dial.
func (n *NetDriver) Dial(address string) (*Conn, error) {
	var c net.Conn
	var err error

	if n.timeout > 0 {
		c, err = net.DialTimeout(n.network, address, n.timeout)
	} else {
		c, err = net.Dial(n.network, address)
	}
	return NewConn(c), err
}

// Close is analogous to net.Close.
func (n *NetDriver) Close(c *Conn) {
	nc := c.NetConn()
	if nc == nil {
		return
	}
	_ = nc.Close()
}

// TestOnBorrow does nothing.
func (n *NetDriver) TestOnBorrow(c *Conn) error {
	return nil
}

// Temporary is analogous to net.Error.Temporary.
func (n *NetDriver) Temporary(err error) bool {
	if e, ok := err.(net.Error); ok {
		return e.Temporary()
	}
	return false
}
