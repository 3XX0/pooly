package pooly

import (
	"net"
	"time"
)

type netConn struct {
	net.Conn
	writeTimeout time.Duration
	readTimeout  time.Duration
}

func (c *netConn) SetReadTimeout(d time.Duration) {
	c.readTimeout = d
}

func (c *netConn) SetWriteTimeout(d time.Duration) {
	c.writeTimeout = d
}

func (c *netConn) Read(b []byte) (int, error) {
	if c.readTimeout > 0 {
		c.SetReadDeadline(time.Now().Add(c.readTimeout))
	}
	return c.Conn.Read(b)
}

func (c *netConn) Write(b []byte) (n int, err error) {
	if c.writeTimeout > 0 {
		c.SetWriteDeadline(time.Now().Add(c.writeTimeout))
	}
	return c.Conn.Write(b)
}

// newConn returns a net.Conn that handles per-read & per-write timeout, in addition
// to regular connection establishment timeout
func newNetConn(netw, addr string, connectionTimeout time.Duration) (*netConn, error) {
	var inner net.Conn
	var err error

	if connectionTimeout > 0 {
		inner, err = net.DialTimeout(netw, addr, connectionTimeout)
	} else {
		inner, err = net.Dial(netw, addr)
	}
	conn := &netConn{
		Conn: inner,
	}
	return conn, err

}
