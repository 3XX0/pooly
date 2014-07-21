package pooly


import (
	"net/http"
	"net"
	"time"
)

// HTTPDriver is a predefined driver for handling standard http.Client objects.
type HTTPDriver struct {
	timeout time.Duration
}

// NewHTTPDriver instantiates a new HTTPDriver, ready to be used in a PoolConfig.
func NewHTTPDriver() *HTTPDriver {
	return new(HTTPDriver)
}

// SetTimeout sets the dialing timeout on a http.Client (underlying http.Transport) object.
func (h *HTTPDriver) SetTimeout(timeout time.Duration) {
	h.timeout = timeout
}

// Dial is analogous to net.Dial.
func (h *HTTPDriver) Dial(address string) (*Conn, error) {
	var err error
	var c net.Conn

	if h.timeout > 0 {
		c, err = net.DialTimeout("tcp", address, h.timeout)
	} else {
		c, err = net.Dial("tcp", address)
	}
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{
		MaxIdleConnsPerHost:    1,
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
