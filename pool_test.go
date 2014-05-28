package pooly

import (
	"errors"
	"sync"
	"testing"
	"time"
)

type customDriver struct {
	*NetDriver
}

func (d *customDriver) TestOnBorrow(c *Conn) error {
	return ping(c.NetConn())
}

var (
	netDriver  = NewNetDriver("tcp", "localhost:7357")
	testDriver = &customDriver{netDriver}
)

func TestBulkGet(t *testing.T) {
	var w sync.WaitGroup

	e := newEchoServer(t)
	defer e.close()

	p, _ := NewPool(&PoolConfig{Driver: netDriver})

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
	}
	w.Wait()
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPut(t *testing.T) {
	e := newEchoServer(t)
	defer e.close()

	p, _ := NewPool(&PoolConfig{Driver: netDriver})

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

func TestClose(t *testing.T) {
	p, _ := NewPool(&PoolConfig{Driver: netDriver})

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Get(); err != ErrPoolClosed {
		t.Fatal("closed pool expected")
	}
}

func TestConnFailed(t *testing.T) {
	p, _ := NewPool(&PoolConfig{
		Driver:      netDriver,
		WaitTimeout: 10 * time.Millisecond,
	})

	_, err := p.Get()
	if err != ErrPoolTimeout {
		t.Fatal("timeout expected")
	}

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestConnIdle(t *testing.T) {
	e := newEchoServer(t)
	defer e.close()

	p, _ := NewPool(&PoolConfig{
		Driver:      netDriver,
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

func TestMaxConns(t *testing.T) {
	e := newEchoServer(t)
	defer e.close()

	p, _ := NewPool(&PoolConfig{
		Driver:      netDriver,
		WaitTimeout: 10 * time.Millisecond,
		MaxConns:    1,
	})

	c, err := p.Get()
	if err != nil {
		t.Fatal(err)
	}

	_, err = p.Get()
	if err != ErrPoolTimeout {
		t.Fatal("timeout expected")
	}

	p.Put(c, nil)
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTestOnBorrow(t *testing.T) {
	e := newEchoServer(t)
	defer e.close()

	p, _ := NewPool(&PoolConfig{Driver: testDriver})

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

func TestBugousPut(t *testing.T) {
	p, _ := NewPool(&PoolConfig{Driver: netDriver})

	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Put(NewConn(nil), nil); err != ErrPoolClosed {
		t.Fatal("closed pool expected")
	}
}
