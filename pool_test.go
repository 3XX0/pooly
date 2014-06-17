package pooly

import (
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestPoolBulkGet(t *testing.T) {
	var w sync.WaitGroup

	e := newEchoServer(t, echo1)
	defer e.close()

	p := NewPool(echo1, nil)

	w.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			c, err := p.Get()
			w.Done()
			if err != nil {
				t.Error(err)
				return
			}
			err = ping(c.NetConn())
			if err != nil {
				t.Error(err)
			}
			p.Put(c, err)
		}()
		// Slow it down, so that we can observe connections reuse
		time.Sleep(1 * time.Millisecond)
	}

	w.Wait()
	t.Log("active connections:", p.ActiveConns())
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolPut(t *testing.T) {
	e := newEchoServer(t, echo1)
	defer e.close()

	p := NewPool(echo1, nil)

	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	p.Put(c, nil)

	d, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	p.Put(d, errors.New("")) // fake an operation failure
	if c != d {
		t.Fatal("connections match expected")
	}

	c, err = p.Get()
	if err != nil {
		t.Fatal(err)
	}
	p.Put(c, nil)
	if c == d {
		t.Fatal("connections mismatch expected")
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolClose(t *testing.T) {
	p := NewPool(echo1, nil)

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Get(); err != ErrPoolClosed {
		t.Fatal("closed pool expected")
	}
}

func TestPoolConnFailed(t *testing.T) {
	p := NewPool(echo1, &PoolConfig{
		WaitTimeout: 10 * time.Millisecond,
	})

	_, err := p.Get()
	if err != ErrOpTimeout {
		t.Fatal("timeout expected")
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolConnIdle(t *testing.T) {
	e := newEchoServer(t, echo1)
	defer e.close()

	p := NewPool(echo1, &PoolConfig{
		IdleTimeout: 10 * time.Millisecond,
	})

	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	p.Put(c, nil)

	time.Sleep(p.IdleTimeout) // wait for idle timeout to expire

	d, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	p.Put(d, nil)
	if c == d {
		t.Fatal("connections mismatch expected")
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolMaxConns(t *testing.T) {
	e := newEchoServer(t, echo1)
	defer e.close()

	p := NewPool(echo1, &PoolConfig{
		WaitTimeout: 10 * time.Millisecond,
		MaxConns:    1,
	})

	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Get()
	if err != ErrOpTimeout {
		t.Fatal("timeout expected")
	}

	p.Put(c, nil)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolTestOnBorrow(t *testing.T) {
	e := newEchoServer(t, echo1)
	defer e.close()

	p := NewPool(echo1, &PoolConfig{
		Driver: testDriver,
	})

	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	c.NetConn().Close() // close the underlying connection
	p.Put(c, nil)

	d, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}
	p.Put(d, nil)
	if c == d {
		t.Fatal("connections mismatch expected")
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPoolBugousPut(t *testing.T) {
	p := NewPool(echo1, nil)

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Put(NewConn(nil), nil); err != ErrPoolClosed {
		t.Fatal("closed pool expected")
	}
}

func TestPoolParallelRandOps(t *testing.T) {
	var w sync.WaitGroup

	e := newEchoServer(t, echo1)
	defer e.close()

	rand.Seed(time.Now().Unix())
	for i := 0; i < 10; i++ {
		p := NewPool(echo1, nil)

		w.Add(1)
		go func() {
			d := time.Duration(rand.Intn(10))
			time.Sleep(d * time.Millisecond)
			c, err := p.Get()
			if err != nil {
				if err != ErrPoolClosed {
					t.Error(err)
				}
				return
			}
			p.Put(c, nil)
		}()
		go func() {
			d := time.Duration(rand.Intn(10))
			time.Sleep(d * time.Millisecond)
			if err := p.Close(); err != nil {
				t.Error(err)
			}
			w.Done()
		}()
		w.Wait()
	}
}

func TestPoolForceClose(t *testing.T) {
	var w sync.WaitGroup
	var b = true

	e := newEchoServer(t, echo1)
	defer e.close()

	p := NewPool(echo1, nil)

	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	f := func() {
		w.Add(1)
		b = p.ForceClose()
		w.Done()
	}
	time.AfterFunc(10*time.Millisecond, f)

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}

	w.Wait()
	if b == false {
		t.Fatal("forced close expected")
	}
	c.NetConn().Close()
}
