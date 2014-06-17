package pooly

import (
	"sync"
	"time"
)

type Computer interface {
	Compute(float64) float64
}

type Selecter interface {
	Select(map[string]*Host) *Host
}

type ServiceConfig struct {
	PoolConfig

	PrespawnConns        uint
	CloseDeadline        time.Duration
	DecayDuration        time.Duration
	MemoizeScoreDuration time.Duration
	ScoreCalculator      Computer
	BanditStrategy       Selecter
}

type Service struct {
	*ServiceConfig

	sync.RWMutex
	name    string
	hosts   map[string]*Host
	decay   *time.Ticker
	memoize *time.Ticker
	add, rm chan string
	stop    chan struct{}
}

func NewService(name string, c *ServiceConfig) *Service {
	if c == nil {
		c = new(ServiceConfig)
	}
	if c.PrespawnConns == 0 {
		c.PrespawnConns = DefaultPrespawnConns
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

	go s.serve()
	return s
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
	p.New(s.PrespawnConns)
	s.hosts[a] = &Host{
		pool:       p,
		timeSeries: make([]serie, 1, seriesNum),
		score:      -1,
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

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Add(address string) {
	s.add <- address
}

func (s *Service) Remove(address string) {
	s.rm <- address
}

func (s *Service) GetConn() (*Conn, error) {
	s.RLock()
	if len(s.hosts) == 0 {
		return nil, ErrNoHostAvailable
	}
	h := s.BanditStrategy.Select(s.hosts)
	s.RUnlock()

	c, err := h.pool.Get()
	if err != nil {
		// Pool is closed or timed out, demote the host and start over
		h.rate(HostDown)
		return s.GetConn()
	}

	c.setHost(h)
	return c, nil
}

func (s *Service) Status() map[string]int32 {
	s.RLock()
	m := make(map[string]int32, len(s.hosts))
	for a, h := range s.hosts {
		m[a] = h.pool.ActiveConns()
	}
	s.RUnlock()
	return m
}

func (s *Service) Close() {
	close(s.stop)
}
