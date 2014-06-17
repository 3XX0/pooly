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
	time.Sleep(1 * time.Millisecond) // wait for propagation

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
		// Slow it down, so that we can observe connections reuse
		time.Sleep(1 * time.Millisecond)
	}
	w.Wait()
	t.Log("status:", s.Status())
}

func TestHostScore(t *testing.T) {
	e := newEchoServer(t, echo1)
	defer e.close()

	s := NewService("echo", &ServiceConfig{
		BanditStrategy: NewEpsilonGreedy(0.1),
	})
	defer s.Close()

	s.Add(echo1)

	c, err := s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Release(nil, 0.6); err != nil {
		t.Fatal(err)
	}

	time.Sleep(DefaultDecayDuration / seriesNum)

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Release(nil, 0.3); err != nil {
		t.Fatal(err)
	}

	time.Sleep(DefaultDecayDuration / seriesNum)

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Release(nil, 1); err != nil {
		t.Fatal(err)
	}

	time.Sleep(DefaultMemoizeScoreDuration)

	c, err = s.GetConn()
	if err != nil {
		t.Fatal(err)
	}
	if c.host.Score() != 0.7 {
		t.Fatal("score of 0.7 expected")
	}
	if err := c.Release(nil, 1); err != nil {
		t.Fatal(err)
	}
}
