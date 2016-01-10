package pooly

import (
	"fmt"
	"github.com/cactus/go-statsd-client/statsd"
	"runtime"
	"sync"
	"time"
)

// Computer describes the interface responsible of computing the resulting score of a host.
// It takes the initial score and returns one computed using a predefined function (e.g exp, log ...)
type Computer interface {
	Compute(float64) float64
}

// Selecter describes the interface responsible of selecting a host among the ones registered in the service.
type Selecter interface {
	Select(map[string]*Host) *Host
}

// ServiceConfig defines the service configuration options.
type ServiceConfig struct {
	PoolConfig

	// Number of connections to prespawn on hosts additions (DefaultPrespawnConns by default).
	PrespawnConns uint

	// Number of attempts to get a connection from the service before giving up (DefaultGetAttempts by default).
	GetAttempts uint

	// Deadline after which pools are forced closed (see Pool.ForceClose) (DefaultCloseDeadline by default).
	CloseDeadline time.Duration

	// Defines the time interval taken into account when scores are computed (DefaultDecayDuration by default).
	// Scores are calculated using a weighted average over the course of this duration (recent feedbacks get higher weight).
	DecayDuration time.Duration

	// Time interval between two successive hosts scores computations.
	// Each score is calculated and cached for this duration (DefaultMemoizeScoreDuration by default).
	MemoizeScoreDuration time.Duration

	// Optional score calculator (none by default).
	ScoreCalculator Computer

	// Multi-armed bandit strategy used for host selection (RoundRobin by default).
	// The tradeoff faced by the service at each GetConn is between "exploitation" (choose hosts having the highest score)
	// and "exploration" (find about the expected score of other hosts).
	// The key here is to find the right balance between "exploration" and "exploitation" of hosts given their respective score.
	// Some strategies will favor fairness while others will prefer to pick hosts based on how well they perform.
	BanditStrategy Selecter

	// Address and port of a statsd server to collect and aggregate pooly service metrics (none by default).
	StatsdAddr string
}

// Service manages several hosts, every one of them having a connection pool (see Pool).
// It computes periodically hosts scores and learns about the best alternatives according to the BanditStrategy option.
// Hosts are added or removed from the service via Add and Remove respectively.
// The application calls the GetConn method to get a connection and releases it through the Conn.Release interface.
// When one is done with the pool, Close will cleanup all the service resources.
type Service struct {
	*ServiceConfig

	sync.RWMutex
	name    string
	hosts   map[string]*Host
	decay   *time.Ticker
	memoize *time.Ticker
	add, rm chan string
	stop    chan struct{}
	stats   statsd.Statter
}

// NewService creates a new service given a unique name.
// If no configuration is specified (nil), defaults values are used.
func NewService(name string, c *ServiceConfig) (*Service, error) {
	var err error

	if c == nil {
		c = new(ServiceConfig)
	}
	if c.PrespawnConns == 0 {
		c.PrespawnConns = DefaultPrespawnConns
	}
	if c.GetAttempts == 0 {
		c.GetAttempts = DefaultGetAttempts
	}
	if c.CloseDeadline == 0 {
		c.CloseDeadline = DefaultCloseDeadline
	}
	if c.DecayDuration == 0 {
		c.DecayDuration = DefaultDecayDuration
	}
	if c.MemoizeScoreDuration == 0 {
		c.MemoizeScoreDuration = DefaultMemoizeScoreDuration
	}
	if c.BanditStrategy == nil {
		c.BanditStrategy = NewRoundRobin()
	}

	s := &Service{
		ServiceConfig: c,
		name:          name,
		hosts:         make(map[string]*Host),
		add:           make(chan string),
		rm:            make(chan string),
		stop:          make(chan struct{}),
	}
	if _, ok := s.BanditStrategy.(*RoundRobin); !ok {
		s.decay = time.NewTicker(c.DecayDuration / seriesNum)
		s.memoize = time.NewTicker(c.MemoizeScoreDuration)
	}
	if c.StatsdAddr != "" {
		s.stats, err = statsd.New(c.StatsdAddr, "service."+name)
		if err != nil {
			return nil, err
		}
	}
	if s.stats == nil {
		s.stats, _ = statsd.NewNoop()
	} else {
		runtime.SetFinalizer(s.stats, func(s statsd.Statter) { s.Close() })
		s.stats.Gauge("conns.count", 0, sampleRate)
		s.stats.Gauge("hosts.count", 0, sampleRate)
		go s.monitor()
	}

	go s.serve()
	return s, nil
}

func (s *Service) monitor() {
	t := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-t.C:
			status := s.Status()
			n := int64(len(status))
			s.stats.Gauge("hosts.count", n, sampleRate)
			n = 0
			for _, c := range status {
				n += int64(c)
			}
			s.stats.Gauge("conns.count", n, sampleRate)

		case <-s.stop:
			t.Stop()
			return
		}
	}
}

func (s *Service) serve() {
	var decay, memoize <-chan time.Time

	if s.decay != nil {
		decay = s.decay.C
	}
	if s.memoize != nil {
		memoize = s.memoize.C
	}
	for {
		select {
		case a := <-s.add:
			s.newHost(a)
		case a := <-s.rm:
			s.deleteHost(a)
		case <-decay:
			for _, h := range s.hosts {
				h.decay()
			}
		case <-memoize:
			// XXX lock to prevent selecting hosts during scores computation
			//s.Lock()
			for _, h := range s.hosts {
				h.computeScore(s.ScoreCalculator)
			}
			//c.Unlock()
		case <-s.stop:
			for a := range s.hosts {
				s.deleteHost(a)
			}
			if s.decay != nil {
				s.decay.Stop()
			}
			if s.memoize != nil {
				s.memoize.Stop()
			}
			return
		}
	}
}

func (s *Service) newHost(a string) {
	s.Lock()
	if h := s.hosts[a]; h != nil {
		s.Unlock()
		return
	}

	p := NewPool(a, &s.PoolConfig)
	p.setStats(s.stats)

	p.New(s.PrespawnConns)
	s.hosts[a] = &Host{
		pool:       p,
		timeSeries: make([]serie, 1, seriesNum),
		score:      -1,
		stats:      s.stats,
	}
	s.Unlock()
}

func (s *Service) deleteHost(a string) {
	s.Lock()
	h := s.hosts[a]
	delete(s.hosts, a)
	s.Unlock()

	if h == nil {
		return
	}
	go func() {
		time.AfterFunc(s.CloseDeadline, func() {
			h.pool.ForceClose()
		})
		h.pool.Close()
	}()
}

// Name returns the name of the service.
func (s *Service) Name() string {
	return s.name
}

// Add adds a given host to the service.
// The effect of such operation may not be reflected immediately.
func (s *Service) Add(address string) {
	s.add <- address
}

// Remove removes a given host from the service.
// The effect of such operation may not be reflected immediately.
func (s *Service) Remove(address string) {
	s.rm <- address
}

// GetConn returns a connection from the service.
// The host serving the connection is chosen according to the BanditStrategy policy in place.
func (s *Service) GetConn() (*Conn, error) {
	var attempts uint

	start := time.Now()
again:
	s.RLock()
	if len(s.hosts) == 0 {
		s.RUnlock()
		if attempts < s.GetAttempts {
			attempts++
			goto again
		}
		return nil, ErrNoHostAvailable
	}
	h := s.BanditStrategy.Select(s.hosts)
	s.RUnlock()

	c, err := h.pool.Get()
	if err != nil {
		// Pool is closed or timed out, demote the host and start over
		s.stats.Inc("conns.get.fails", 1, sampleRate)
		h.rate(HostDown)
		if attempts < s.GetAttempts {
			attempts++
			goto again
		}
		return nil, fmt.Errorf("%s: %v", s.name, err)
	}

	// Send statsd metrics
	end := time.Now()
	dt := int64(end.Sub(start).Seconds() * 1000)
	s.stats.Timing("conns.get.delay", dt, sampleRate)
	s.stats.Inc("conns.get.count", 1, sampleRate)
	if _, ok := s.BanditStrategy.(*RoundRobin); !ok {
		p := int64(h.Score() * 100)
		s.stats.Timing("hosts.score", p, sampleRate)
	}

	c.setTime(end)
	c.setHost(h)
	return c, nil
}

// Status returns every host addresses managed by the service along with
// the number of connections handled by their respective pool thus far.
func (s *Service) Status() map[string]int32 {
	s.RLock()
	m := make(map[string]int32, len(s.hosts))
	for a, h := range s.hosts {
		m[a] = h.pool.ActiveConns()
	}
	s.RUnlock()
	return m
}

// Close closes the service, thus destroying all hosts and their respective pool.
// After a call to Close, the service can not be used again.
func (s *Service) Close() {
	close(s.stop)
}
