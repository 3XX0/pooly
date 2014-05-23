package pooly

import (
	"time"
)

// Conn abstracts user connections that are part of a Pool.
type Conn struct {
	iface  interface{}
	timer  *time.Timer
	closed bool
}

// Create a new connection container, wrapping up a user defined connection object.
func NewConn(i interface{}) *Conn {
	return &Conn{iface: i}
}

// Interface returns an interface referring to the underlying user object.
func (c *Conn) Interface() interface{} {
	return c.iface
}

func (c *Conn) isClosed() bool {
	return c.closed
}

func (c *Conn) setClosed() {
	c.closed = true
}

func (c *Conn) setIdle(p *Pool) {
	if p.IdleTimeout > 0 {
		c.timer = time.AfterFunc(p.IdleTimeout, func() {
			// The connection has been idle for too long,
			// send it to the garbage collector
			p.gc <- c
		})
	}
}

func (c *Conn) setActive() bool {
	return c.timer.Stop()
}
