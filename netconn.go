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

// Returns a net.Conn that handles per-read/write timeout in addition to the regular
// connection establishment timeout.
func newNetConn(network, address string, timeout time.Duration) (*netConn, error) {
	var c net.Conn
	var err error

	if timeout > 0 {
		c, err = net.DialTimeout(network, address, timeout)
	} else {
		c, err = net.Dial(network, address)
	}
	if err != nil {
		return nil, err
	}

	conn := &netConn{Conn: c}
	return conn, nil
}
