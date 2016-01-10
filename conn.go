package pooly

import (
	"net"
	"time"
)

// Conn abstracts user connections that are part of a Pool.
type Conn struct {
	iface     interface{}
	timer     *time.Timer
	timerStop chan bool
	closed    bool
	host      *Host
	gottenAt  time.Time
}

// NewConn creates a new connection container, wrapping up a user defined connection object.
func NewConn(i interface{}) *Conn {
	return &Conn{
		iface:     i,
		timerStop: make(chan bool),
	}
}

// Interface returns an interface referring to the underlying user object.
func (c *Conn) Interface() interface{} {
	return c.iface
}

// NetConn is a helper for underlying user objects that satisfy the standard library net.Conn interface.
func (c *Conn) NetConn() net.Conn {
	if c.iface == nil {
		return nil
	}
	return c.iface.(net.Conn)
}

func (c *Conn) isClosed() bool {
	return c.closed
}

func (c *Conn) setClosed() {
	if c.timer != nil {
		if c.timer.Stop() {
			c.timerStop <- true
		}
	}
	c.closed = true
}

func (c *Conn) setIdle(p *Pool) {
	if p.ConnIdleTimeout > 0 {
		c.timer = time.NewTimer(p.ConnIdleTimeout)
		go func() {
			select {
			case <-c.timerStop:
				return
			case <-c.timer.C:
				// The connection has been idle for too long,
				// send it to the garbage collector
				p.gc <- c
			}
		}()
	}
}

func (c *Conn) setActive() bool {
	if c.timer != nil {
		if !c.timer.Stop() {
			return false
		}
		c.timerStop <- true
	}
	return true
}

func (c *Conn) setHost(h *Host) {
	c.host = h
}

func (c *Conn) setTime(t time.Time) {
	c.gottenAt = t
}

func (c *Conn) diffTime() time.Duration {
	return time.Now().Sub(c.gottenAt)
}

// Release releases the connection back to its linked service.
// It takes an error state which defines whether or not the connection failed during operation and
// a score between 0 and 1 which describes how well the connection performed (e.g inverse response time, up/down ...).
// If the error state indicates a fatal error (determined by Driver.Temporary), the score is forced to the value 0 (HostDown).
func (c *Conn) Release(err interface{}, score float64) error {
	var e error

	if c.host == nil {
		return ErrNoHostAvailable
	}
	if score < 0 || score > 1 {
		return ErrInvalidArg
	}

	switch v := err.(type) {
	case error:
		e = v
	case *error:
		e = *v
	case nil:
		e = nil
	default:
		return ErrInvalidArg
	}

	h := c.host
	c.host = nil
	return h.releaseConn(c, e, score)
}

// Close the connection
func (c *Conn) Close() error {
	h := c.host
	c.host = nil
	return h.pool.CloseConn(c)
}

// Address returns the address of the host bound to the connection.
func (c *Conn) Address() string {
	if c.host == nil {
		return ""
	}
	return c.host.pool.Address()
}
