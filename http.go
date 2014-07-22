package pooly

import (
	"net"
	"net/http"
	"time"
)

// HTTPDriver is a predefined driver for handling standard http.Client objects.
type HTTPDriver struct {
	connTimeout  time.Duration
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewHTTPDriver instantiates a new HTTPDriver, ready to be used in a PoolConfig.
func NewHTTPDriver() *HTTPDriver {
	return new(HTTPDriver)
}

// SetConnTimeout sets the dialing timeout on a http.Client (underlying http.Transport) object.
func (h *HTTPDriver) SetConnTimeout(timeout time.Duration) {
	h.connTimeout = timeout
}

// SetReadTimeout sets the read timeout on a http.Client (underlying http.Transport) object.
func (h *HTTPDriver) SetReadTimeout(timeout time.Duration) {
	h.readTimeout = timeout
}

// SetWriteTimeout sets the write timeout on a http.Client (underlying http.Transport) object.
func (h *HTTPDriver) SetWriteTimeout(timeout time.Duration) {
	h.writeTimeout = timeout
}

// Dial is analogous to net.Dial.
func (h *HTTPDriver) Dial(address string) (*Conn, error) {
	c, err := newNetConn("tcp", address, h.connTimeout)
	if err != nil {
		return nil, err
	}
	c.SetReadTimeout(h.readTimeout)
	c.SetWriteTimeout(h.writeTimeout)

	tr := &http.Transport{
		MaxIdleConnsPerHost: 1,
		Dial: func(network, addr string) (net.Conn, error) {
			return c, nil
		},
	}
	client := &http.Client{Transport: tr}
	return NewConn(client), err
}

// Close is analogous to net.Close.
func (h *HTTPDriver) Close(c *Conn) {
	hc := c.HTTPConn()
	if hc == nil {
		return
	}
	hc.Transport.(*http.Transport).CloseIdleConnections()
}

// TestOnBorrow does nothing.
func (h *HTTPDriver) TestOnBorrow(c *Conn) error {
	return nil
}

// Temporary is analogous to net.Error.Temporary.
func (h *HTTPDriver) Temporary(err error) bool {
	if e, ok := err.(net.Error); ok {
		return e.Temporary()
	}
	return false
}
