package pooly

import (
	"errors"
	"time"
)

var DefaultDriver = NewNetDriver("tcp")

const (
	DefaultConnsNum    = 10
	DefaultAttemptsNum = 3
	DefaultRetryDelay  = 10 * time.Millisecond
)

// Pool global errors.
var (
	ErrPoolInvalidArg = errors.New("pooly: invalid argument")
	ErrPoolClosed     = errors.New("pooly: pool is closed")
	ErrPoolTimeout    = errors.New("pooly: operation timed out")
)

// Driver describes the interface responsible of creating/deleting/testing pool connections.
type Driver interface {
	// Dial is a function that given an address, establishes a connection with a remote host.
	// It returns the connection created or an error on failure.
	Dial(string) (*Conn, error)

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
	// Connection driver (DefaultDriver by default).
	Driver Driver

	// Close connections after remaining idle for this duration.
	// If the value is zero (default), then idle connections are not closed.
	IdleTimeout time.Duration

	// Defines the duration during which Get operations will try to return a connection from the pool.
	// If the value is zero (default), then Get should wait forever.
	WaitTimeout time.Duration

	// Maximum number of connections allowed in the pool (DefaultConnsNum by default).
	MaxConns int32

	// Maximum number of connection attempts (DefaultAttemptsNum by default).
	MaxAttempts int

	// Time interval between connection attempts (DefaultRetryDelay by default).
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

	address    string
	status     state
	inbound    channel
	connsCount counter
	conns      chan *Conn
	gc         chan *Conn
	gcCtl      chan int
}

// Pool status
const (
	active int32 = iota
	closing
	closed
)

// GC control options
const (
	wakeup int = iota
	kill
)

// NewPool creates a new pool of connections.
// If no configuration is specified, defaults values are used.
func NewPool(address string, c *PoolConfig) *Pool {
	if c == nil {
		c = new(PoolConfig)
	}
	if c.Driver == nil {
		c.Driver = DefaultDriver
	}
	if c.MaxConns <= 0 {
		c.MaxConns = DefaultConnsNum
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = DefaultAttemptsNum
	}
	if c.RetryDelay == 0 {
		c.RetryDelay = DefaultRetryDelay
	}

	p := &Pool{
		PoolConfig: c,
		address:    address,
		status:     newState(active),
		connsCount: newCounter(c.MaxConns),
		conns:      make(chan *Conn, c.MaxConns),
		gc:         make(chan *Conn, c.MaxConns),
		gcCtl:      make(chan int, 1),
	}
	p.inbound = newChannel(&p.conns)

	go p.collect()
	return p
}

// Garbage collects connections.
func (p *Pool) collect() {
	var c *Conn

	for {
		if p.status.is(closing) && p.connsCount.zero() {
			// All connections have been garbage collected
			if p.status.set(closed) {
				close(p.conns) // notify Close that we're done
			}
			return
		}

		select {
		case ctl := <-p.gcCtl:
			switch ctl {
			case wakeup:
				continue
			case kill:
				return
			}
		case c = <-p.gc:
		}

		if c != nil && !c.isClosed() {
			// XXX workaround to avoid closing twice a connection
			// Since idle timeouts can occur at any time, we may have duplicates in the queue
			c.setClosed()
			p.Driver.Close(c)
			p.connsCount.decrement()
		} else if c == nil {
			p.connsCount.decrement()
		}
	}
}

func (p *Pool) newConn() {
	if !p.connsCount.increment() {
		return
	}
	for i := 0; i < p.MaxAttempts; i++ {
		c, err := p.Driver.Dial(p.address)
		if c != nil && (err == nil || p.Driver.Temporary(err)) {
			c.setIdle(p)
			p.inbound.channel() <- c
			return
		}
		time.Sleep(p.RetryDelay)
	}
	p.gc <- nil // connection failed
}

// New attempts to create n new connections in background.
// Note that it does nothing when MaxConns is reached.
func (p *Pool) New(n int) error {
	if p.status.is(closing) {
		return ErrPoolClosed
	}

	for i := 0; i < n; i++ {
		go p.newConn()
	}
	return nil
}

// ActiveConns returns the number of connections handled by the pool thus far.
func (p *Pool) ActiveConns() int32 {
	return p.connsCount.fetch()
}

// Get gets a fully tested connection from the pool
func (p *Pool) Get() (*Conn, error) {
	var t <-chan time.Time
	var c *Conn

	if p.status.is(closing) {
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
func (p *Pool) Put(c *Conn, e error) error {
	if p.status.is(closed) {
		return ErrPoolClosed
	}

	if c == nil {
		return ErrPoolInvalidArg
	}
	if e != nil && !p.Driver.Temporary(e) {
		p.gc <- c
		return nil
	}
	c.setIdle(p)
	p.inbound.channel() <- c
	return nil
}

// Close closes the pool, thus destroying all connections.
// It returns when all spawned connections have been successfully garbage collected.
// After a successful call to Close, the pool can not be used again.
func (p *Pool) Close() error {
	if p.status.is(closing) {
		return ErrPoolClosed
	}

	p.inbound.set(&p.gc)
	p.status.set(closing)

	// XXX wakeup the garbage collector if it happens to be asleep
	// This is necessary when a Close is issued and there are no more connections left to collect
	p.gcCtl <- wakeup

	// Garbage collect all the idle connections left
	for c := range p.conns {
		p.gc <- c
	}
	return nil
}

// ForceClose forces the termination of an ongoing Close operation.
// It returns true if Close is interrupted successfully, false otherwise.
// Note that all pending connections unacknowledged by Close will be left unchanged and won't ever be destroyed.
func (p *Pool) ForceClose() bool {
	if p.status.is(closing) && p.status.set(closed) {
		close(p.conns)
		p.gcCtl <- kill
		return true
	}
	return false
}

func (p *Pool) Address() string {
	return p.address
}
