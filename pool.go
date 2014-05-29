package pooly

import (
	"errors"
	"sync/atomic"
	"time"
	"unsafe"
)

const (
	defaultConnsNum    = 10
	defaultAttemptsNum = 3
	defaultRetryDelay  = 10 * time.Millisecond
)

// Pool global errors.
var (
	ErrPoolInvalidArg = errors.New("pooly: invalid argument")
	ErrPoolClosed     = errors.New("pooly: pool is closed")
	ErrPoolTimeout    = errors.New("pooly: operation timed out")
)

// Driver describes the interface responsible of creating/deleting/testing pool connections.
type Driver interface {
	// Dial is a function that establishes a connection with a remote host.
	// It returns the connection created or an error on failure.
	Dial() (*Conn, error)

	// Close closes the given connection.
	Close(*Conn)

	// TestOnBorrow is a function that, given a connection, tests it and returns an error on failure.
	TestOnBorrow(*Conn) error

	// Temporary determines whether the error is temporary or fatal for the connection.
	// On fatal error, the connection will be garbage collected.
	Temporary(error) bool
}

// PoolConfig defines the pool configuration options.
type PoolConfig struct {
	// Connection driver.
	Driver Driver

	// Close connections after remaining idle for this duration.
	// If the value is zero, then idle connections are not closed.
	IdleTimeout time.Duration

	// Defines the duration during which Get operations will try to return a connection from the pool.
	// If the value is zero, then Get should wait forever.
	WaitTimeout time.Duration

	// Maximum number of connections allowed in the pool (10 by default).
	MaxConns int32

	// Maximum number of connection attempts (3 by default).
	MaxAttempts int

	// Time interval between connection attempts (10ms by default).
	RetryDelay time.Duration
}

// Pool maintains a pool of connections. The application calls the Get method to get a connection
// from the pool and the Put method to return the connection to the pool. New can be called to allocate
// more connections in the background.
// When one is done with the pool, Close will cleanup all the connections ressources.
// The pool itself will adapt to the demand by spawning and destroying connections as needed. In order to
// tweak its behavior, settings like IdleTimeout and MaxConns may be used.
type Pool struct {
	*PoolConfig

	connsCount int32
	conns      chan *Conn
	gc         chan *Conn
	inbound    unsafe.Pointer
	closing    int32
	wakeupGC   chan struct{}
}

func (p *Pool) setClosing() {
	atomic.StoreInt32(&p.closing, 1)
}

func (p *Pool) isClosing() bool {
	return atomic.LoadInt32(&p.closing) == 1
}

func (p *Pool) inboundChannel() chan *Conn {
	i := atomic.LoadPointer(&p.inbound)
	return *(*chan *Conn)(i)
}

// After that, all inbound connections will be garbage collected.
func (p *Pool) setInboundChannelGC() {
	i := unsafe.Pointer(&p.gc)
	atomic.StorePointer(&p.inbound, i)
}

// Atomically returns the current connections count.
func (p *Pool) fetchConnsCount() int32 {
	for b := false; !b; {
		n := atomic.LoadInt32(&p.connsCount)
		if n > 0 {
			return n
		}
		// Null, set it back to MaxConns in order to prevent newConn from altering it
		b = atomic.CompareAndSwapInt32(&p.connsCount, n, p.MaxConns)
	}
	return 0
}

// Atomically increments the number of connections.
func (p *Pool) incConnsCount() bool {
	for b := false; !b; {
		n := atomic.LoadInt32(&p.connsCount)
		if n == p.MaxConns {
			return false // maximum connections count reached
		}
		b = atomic.CompareAndSwapInt32(&p.connsCount, n, n+1)
	}
	return true
}

// Atomically decrements the number of connections.
func (p *Pool) decConnsCount() {
	atomic.AddInt32(&p.connsCount, -1)
}

// Garbage collects connections.
func (p *Pool) collect() {
	var c *Conn

	for {
		if p.isClosing() {
			if p.fetchConnsCount() == 0 {
				// All connections have been garbage collected
				close(p.gc)
				close(p.conns) // notify Close that we're done
				return
			}
		}

		select {
		case <-p.wakeupGC:
			p.wakeupGC = nil
			continue
		case c = <-p.gc:
		}

		if c != nil && !c.isClosed() {
			// XXX workaround to avoid closing twice a connection
			// Since idle timeouts can occur at any time, we may have duplicates in the queue
			c.setClosed()
			p.Driver.Close(c)
			p.decConnsCount()
		} else if c == nil {
			p.decConnsCount()
		}
	}
}

// NewPool creates a new pool of connections.
func NewPool(c *PoolConfig) (*Pool, error) {
	if c.Driver == nil {
		return nil, ErrPoolInvalidArg
	}
	if c.MaxConns <= 0 {
		c.MaxConns = defaultConnsNum
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = defaultAttemptsNum
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = defaultRetryDelay
	}

	p := &Pool{
		PoolConfig: c,
		conns:      make(chan *Conn, c.MaxConns),
		gc:         make(chan *Conn, c.MaxConns),
		wakeupGC:   make(chan struct{}),
	}
	p.inbound = unsafe.Pointer(&p.conns)
	go p.collect()

	return p, nil
}

func (p *Pool) newConn() {
	if !p.incConnsCount() {
		return
	}
	for i := 0; i < p.MaxAttempts; i++ {
		c, err := p.Driver.Dial()
		if c != nil && (err == nil || p.Driver.Temporary(err)) {
			c.setIdle(p)
			p.inboundChannel() <- c
			return
		}
		time.Sleep(p.RetryDelay)
	}
	p.gc <- nil // connection failed
}

// New attempts to create n new connections in background.
// Note that it does nothing when MaxConns is reached.
func (p *Pool) New(n int) error {
	if p.isClosing() {
		return ErrPoolClosed
	}
	for i := 0; i < n; i++ {
		go p.newConn()
	}
	return nil
}

// ActiveConns returns the number of connections handled by the pool thus far.
func (p *Pool) ActiveConns() int32 {
	return atomic.LoadInt32(&p.connsCount)
}

// Get gets a fully tested connection from the pool
func (p *Pool) Get() (*Conn, error) {
	var t <-chan time.Time
	var c *Conn

	if p.isClosing() {
		return nil, ErrPoolClosed
	}

	// Try to get a connection right away optimistically
	select {
	case c = <-p.conns:
		goto gotone
	default: // connections are running low, spawn a new one
		if err := p.New(1); err != nil {
			return nil, err
		}
	}

	if p.WaitTimeout > 0 {
		t = time.After(p.WaitTimeout)
	}
	select {
	case c = <-p.conns:
		goto gotone
	case <-t:
		return nil, ErrPoolTimeout
	}

gotone:
	if c == nil {
		// Pool has been closed simultaneously
		return nil, ErrPoolClosed
	}
	if !c.setActive() {
		// Connection timed out, start over
		return p.Get()
	}
	// Test the connection
	if err := p.Driver.TestOnBorrow(c); err != nil {
		if !p.Driver.Temporary(err) {
			p.gc <- c // garbage collect the connection and start over
			return p.Get()
		}
	}
	return c, nil
}

// Put puts a given connection back to the pool depending on its error status.
func (p *Pool) Put(c *Conn, e error) (err error) {
	defer func() {
		// XXX should not happen but to be safe, recover from Put on closed pool
		if r := recover(); r != nil {
			err = ErrPoolClosed
		}
	}()

	if c == nil {
		return ErrPoolInvalidArg
	}
	if e != nil && !p.Driver.Temporary(e) {
		p.gc <- c
		return
	}
	c.setIdle(p)
	p.inboundChannel() <- c
	return
}

// Close closes the pool, thus destroying all connections.
// It returns when all spawned connections have been successfully garbage collected.
// After a successful call to Close, the pool can not be used again.
func (p *Pool) Close() error {
	if p.isClosing() {
		return ErrPoolClosed
	}

	p.setInboundChannelGC()
	p.setClosing()
	// XXX wakeup the garbage collector if it happens to be asleep
	// This is necessary when a Close is issued and there are no more connections left to collect
	close(p.wakeupGC)

	// Garbage collect all the idle connections left
	for c := range p.conns {
		p.gc <- c
	}
	return nil
}
