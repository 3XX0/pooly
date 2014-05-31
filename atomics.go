package pooly

import (
	"sync/atomic"
	"unsafe"
)

type channel struct{ p unsafe.Pointer }

func newChannel(p *chan *Conn) (c channel) {
	c.set(p)
	return
}

func (c *channel) set(p *chan *Conn) {
	atomic.StorePointer(&c.p, unsafe.Pointer(p))
}

func (c *channel) channel() chan *Conn {
	p := atomic.LoadPointer(&c.p)
	return *(*chan *Conn)(p)
}

type counter struct{ c, m int32 }

func newCounter(max int32) counter {
	return counter{m: max}
}

func (c *counter) zero() bool {
	for b := false; !b; {
		n := atomic.LoadInt32(&c.c)
		if n > 0 {
			return false
		}
		b = atomic.CompareAndSwapInt32(&c.c, n, c.m) // zero, set it back to maximum
	}
	return true
}

func (c *counter) fetch() int32 {
	return atomic.LoadInt32(&c.c)
}

func (c *counter) increment() bool {
	for b := false; !b; {
		n := atomic.LoadInt32(&c.c)
		if n == c.m {
			return false // maximum reached
		}
		b = atomic.CompareAndSwapInt32(&c.c, n, n+1)
	}
	return true
}

func (c *counter) decrement() {
	atomic.AddInt32(&c.c, -1)
}

type state int32

func newState(i int32) state {
	return state(i)
}

func (s *state) set(i int32) bool {
	for b := false; !b; {
		n := atomic.LoadInt32((*int32)(s))
		if n >= i {
			return false // lower rank status
		}
		b = atomic.CompareAndSwapInt32((*int32)(s), n, i)
	}
	return true
}

func (s *state) is(i int32) bool {
	return atomic.LoadInt32((*int32)(s)) >= i
}
