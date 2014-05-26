package pooly

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
)

const serverAddress = "localhost:7357"

type server struct {
	l net.Listener
	q chan struct{}
	w sync.WaitGroup
}

func echoServer(t *testing.T) *server {
	var err error

	s := &server{q: make(chan struct{})}
	s.l, err = net.Listen("tcp", serverAddress)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
	loop:
		for {
			c, err := s.l.Accept()
			if err != nil {
				select {
				case <-s.q:
					break loop
				default:
					t.Error(err)
					continue
				}
			}
			s.w.Add(1)
			go func() {
				io.Copy(c, c)
				c.Close()
				s.w.Done()
			}()
		}
	}()
	return s
}

func (s server) close(t *testing.T) {
	close(s.q)
	s.l.Close()
	s.w.Wait()
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

func TestCloseAfterNew(t *testing.T) {
	p, err := NewPool(&PoolConfig{
		Driver: NewNetDriver("tcp", serverAddress),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestBulkGet(t *testing.T) {
	var w sync.WaitGroup

	e := echoServer(t)
	defer e.close(t)

	p, err := NewPool(&PoolConfig{
		Driver: NewNetDriver("tcp", serverAddress),
	})
	if err != nil {
		t.Fatal(err)
	}
	w.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			c, err := p.Get()
			w.Done()
			if err != nil {
				t.Error(err)
			} else {
				ping(t, c.NetConn())
			}
			p.Put(c, err)
		}()
	}
	w.Wait()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}
