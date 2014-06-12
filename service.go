package pooly

import (
	"sync"
	"time"
)

type Computer interface {
	Compute(float64) float64
}

type Selecter interface {
	Select([]*host) *host
}

type ServiceConfig struct {
	*PoolConfig

	SeriesNum       uint
	PrespawnConnNum uint
	CloseDeadline   time.Duration
	DecayDuration   time.Duration
	ScoreCalculator Computer
	BanditStrategy  Selecter
}

type Service struct {
	*ServiceConfig

	sync.RWMutex
	name  string
	hosts map[string]*host
	decay *time.Ticker
	add   chan string
	rm    chan string
	stop  chan struct{}
}

func NewService(name string, c *ServiceConfig) *Service {
	if c == nil {
		c = new(ServiceConfig)
	}
	if c.SeriesNum == 0 {
		c.SeriesNum = DefaultSeriesNum
	}
	if c.PrespawnConnNum == 0 {
		c.PrespawnConnNum = DefaultPrespawnConnNum
	}
	if c.CloseDeadline == 0 {
		c.CloseDeadline = DefaultCloseDeadline
	}
	if c.DecayDuration == 0 {
		c.DecayDuration = DefaultDecayDuration
	}
	if c.BanditStrategy == nil {
		c.BanditStrategy = DefaultBandit
	}

	s := &Service{
		ServiceConfig: c,
		name:          name,
		hosts:         make(map[string]*host),
		decay:         time.NewTicker(c.DecayDuration),
		add:           make(chan string),
		rm:            make(chan string),
		stop:          make(chan struct{}),
	}

	go s.timeShift()
	go s.serve()
	return s
}

func (s *Service) timeShift() {
	for {
		select {
		case <-s.stop:
			return
		case <-s.decay.C:
		}
		s.Lock()
		for _, h := range s.hosts {
			h.shift()
		}
		s.Unlock()
	}
}

func (s *Service) serve() {
	for {
		select {
		case a := <-s.add:
			s.newHost(a)
		case a := <-s.rm:
			s.deleteHost(a)
		case <-s.stop:
			return
		}
	}
}

func (s *Service) Add(address string) {
	s.add <- address
}

func (s *Service) Remove(address string) {
	s.rm <- address
}

func (s *Service) newHost(a string) {
	s.Lock()
	if _, ok := s.hosts[a]; !ok {
		p := NewPool(a, s.PoolConfig)
		p.New(s.PrespawnConnNum)
		s.hosts[a] = &host{
			pool:       p,
			timeSeries: make([]serie, 1, s.SeriesNum),
			score:      -1,
		}
	}
	s.Unlock()
}

func (s *Service) deleteHost(a string) {
	s.Lock()
	h, ok := s.hosts[a]
	if !ok {
		s.Unlock()
		return
	}
	delete(s.hosts, a)
	s.Unlock()

	go s.closePool(h)
}

func (s *Service) closePool(h *host) {
	if err := h.pool.Close(); err != nil {
		f := func() { _ = h.pool.ForceClose() }
		time.AfterFunc(s.CloseDeadline, f)
	}
}

func (s *Service) GetConn() (*Conn, error) {
	s.RLock()
	// TODO cache scores and compute them periodically
	hosts := make([]*host, 0, len(s.hosts))

	for _, h := range s.hosts {
		if _, ok := s.BanditStrategy.(*RoundRobin); !ok {
			h.computeScore(s.ScoreCalculator)
		}
		hosts = append(hosts, h)
	}
	s.RUnlock()

	if len(hosts) == 0 {
		return nil, ErrNoHostAvailable
	}
	h := s.BanditStrategy.Select(hosts)

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

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Close() {
	close(s.stop)

	s.Lock()
	for a, h := range s.hosts {
		go s.closePool(h)
		delete(s.hosts, a)
	}
	s.Unlock()
}
