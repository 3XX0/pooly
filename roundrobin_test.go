package pooly

import (
	"testing"
	"time"
)

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

	time.Sleep(1 * time.Millisecond) // wait for propagation

	c, err := s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a := c.Address()
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
	a = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo2 {
		t.Fatal(echo2, "expected")
	}

	t.Log("status:", s.Status())
	s.Remove(echo3)
	time.Sleep(1 * time.Millisecond) // wait for propagation

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo1 {
		t.Fatal(echo1, "expected")
	}

	t.Log("status:", s.Status())
	s.Add(echo3)
	time.Sleep(1 * time.Millisecond) // wait for propagation

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	a = c.Address()
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
	a = c.Address()
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
	a = c.Address()
	if err := c.Release(nil, HostUp); err != nil {
		t.Fatal(err)
	}
	if a != echo1 {
		t.Fatal(echo1, "expected")
	}

	t.Log("status:", s.Status())
}
