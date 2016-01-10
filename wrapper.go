package pooly

import (
	"net"
	"sync/atomic"
	"time"
)

type timeoutWrapper struct {
	net.Conn

	writeTimeout time.Duration
	readTimeout  time.Duration
}

func (w *timeoutWrapper) Read(b []byte) (int, error) {
	if w.readTimeout > 0 {
		w.SetReadDeadline(time.Now().Add(w.readTimeout))
	}
	return w.Conn.Read(b)
}

func (w *timeoutWrapper) Write(b []byte) (int, error) {
	if w.writeTimeout > 0 {
		w.SetWriteDeadline(time.Now().Add(w.writeTimeout))
	}
	return w.Conn.Write(b)
}

type releaseWrapper struct {
	net.Conn

	conn    *Conn
	lasterr atomic.Value
}

func (w *releaseWrapper) Read(b []byte) (n int, err error) {
	n, err = w.Conn.Read(b)
	if err != nil {
		w.lasterr.Store(&err)
	}
	return
}

func (w *releaseWrapper) Write(b []byte) (n int, err error) {
	n, err = w.Conn.Write(b)
	if err != nil {
		w.lasterr.Store(&err)
	}
	return
}

func (w *releaseWrapper) Close() error {
	return w.conn.Release(w.lasterr.Load(), HostUp)
}

type closeWrapper struct {
	net.Conn

	conn    *Conn
	lasterr atomic.Value
}

func (w *closeWrapper) Read(b []byte) (n int, err error) {
	n, err = w.Conn.Read(b)
	if err != nil {
		w.lasterr.Store(&err)
	}
	return
}

func (w *closeWrapper) Write(b []byte) (n int, err error) {
	n, err = w.Conn.Write(b)
	if err != nil {
		w.lasterr.Store(&err)
	}
	return
}

func (w *closeWrapper) Close() error {
	return w.conn.Close()
}
