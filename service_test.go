package pooly

import (
	"sync"
	"testing"
	"time"
)

func TestServiceStatus(t *testing.T) {
	e := newEchoServer(t, echo1)
	defer e.close()

	s := NewService("echo", nil)
	defer s.Close()

	s.Add(echo1)
	time.Sleep(1 * time.Millisecond)
	m := s.Status()
	if len(m) == 0 {
		t.Fatal("status report expected")
	}
	if n, ok := m[echo1]; !ok || n != 1 {
		t.Fatal("bad status:", m)
	}
}

func TestServiceAddRemove(t *testing.T) {
	var w sync.WaitGroup

	e := newEchoServer(t, echo1)
	defer e.close()

	s := NewService("echo", nil)
	defer s.Close()

	w.Add(2)
	go func() {
		s.Add(echo1)
		s.Remove(echo1)
		w.Done()
	}()
	go func() {
		s.Add(echo1)
		s.Remove(echo1)
		w.Done()
	}()

	w.Wait()
	if _, err := s.GetConn(); err != ErrNoHostAvailable {
		t.Fatal("no host available expected")
	}
}

func TestServiceBulkGetConn(t *testing.T) {
	var w sync.WaitGroup

	e := newEchoServer(t, echo1)
	defer e.close()

	s := NewService("echo", &ServiceConfig{
		PoolConfig: PoolConfig{Driver: testDriver},
	})
	defer s.Close()

	s.Add(echo1)
	w.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			c, err := s.GetConn()
			w.Done()
			if err != nil {
				t.Error(err)
				return
			}
			if err := c.Release(nil, HostUp); err != nil {
				t.Error(err)
			}
		}()
		time.Sleep(1 * time.Millisecond)
	}
	w.Wait()
	t.Log("status:", s.Status())
}

func TestServiceRoundRobin(t *testing.T) {
	e1 := newEchoServer(t, echo1)
	defer e1.close()
	e2 := newEchoServer(t, echo2)
	defer e2.close()
	e3 := newEchoServer(t, echo3)
	defer e3.close()

	s := NewService("echo", nil)
	defer s.Close()

	s.Add(echo1)
	s.Add(echo2)
	s.Add(echo3)

	c, err := s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo1 {
		t.Fatal(echo1, "expected")
	}

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a, _ = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo2 {
		t.Fatal(echo2, "expected")
	}

	t.Log("status:", s.Status())
	s.Remove(echo3)
	time.Sleep(1 * time.Millisecond)

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a, _ = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo1 {
		t.Fatal(echo1, "expected")
	}

	t.Log("status:", s.Status())
	s.Add(echo3)
	time.Sleep(1 * time.Millisecond)

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a, _ = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo2 {
		t.Fatal(echo2, "expected")
	}

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a, _ = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo3 {
		t.Fatal(echo3, "expected")
	}

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a, _ = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo1 {
		t.Fatal(echo1, "expected")
	}

	t.Log("status:", s.Status())
}
