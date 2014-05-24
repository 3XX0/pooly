package pooly

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"
)

const serverAddress = "localhost:7357"

type server struct{ c chan struct{} }

func echoServer(t *testing.T) *server {
	s := &server{make(chan struct{})}

	go func() {
		l, err := net.Listen("tcp", serverAddress)
		if err != nil {
			t.Fatal(err)
		}
		c, err := l.Accept()
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(c, c)
		c.Close()
		l.Close()
		close(s.c)
	}()

	return s
}

func (s server) wait(t *testing.T) {
	select {
	case <-s.c:
	case <-time.After(1 * time.Second):
		t.Error("server timed out")
	}
}

func ping(t *testing.T, c net.Conn) {
	b := make([]byte, 4)
	m := []byte("ping")

	if _, err := c.Write(m); err != nil {
		t.Error(err)
		return
	}
	if _, err := c.Read(b); err != nil {
		t.Error(err)
		return
	}
	if !bytes.Equal(b, m) {
		t.Error("ping failed")
		return
	}
}

func TestGet(t *testing.T) {
	e := echoServer(t)
	defer e.wait(t)

	p, err := NewPool(&PoolConfig{
		Driver: NewNetDriver("tcp", serverAddress),
	})
	if err != nil {
		t.Error(err)
		return
	}
	c, err := p.Get()
	if err != nil {
		t.Error(err)
		return
	}
	ping(t, c.NetConn())
	p.Put(c, err)
	if err := p.Close(); err != nil {
		t.Error(err)
		return
	}
}
