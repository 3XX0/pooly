package pooly

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
)

type echoServer struct {
	l net.Listener
	q chan struct{}
	w sync.WaitGroup
}

func newEchoServer(t *testing.T) *echoServer {
	var err error

	s := &echoServer{q: make(chan struct{})}
	s.l, err = net.Listen("tcp", "localhost:7357")
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

func (s *echoServer) close() {
	close(s.q)
	s.l.Close()
	s.w.Wait()
}

func ping(c net.Conn) error {
	b := make([]byte, 4)
	m := []byte("ping")

	if _, err := c.Write(m); err != nil {
		return err
	}
	if _, err := c.Read(b); err != nil {
		return err
	}
	if !bytes.Equal(b, m) {
		return errors.New("pong bad answer")
	}
	return nil
}
