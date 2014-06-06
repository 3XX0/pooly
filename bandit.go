package pooly

type SoftMax struct {
}

type EpsilonGreedy struct {
}

type RoundRobin struct {
}

func NewRoundRobin() *RoundRobin {
	return new(RoundRobin)
}

func (r *RoundRobin) Select(hosts []*host) *host {
	return nil
}
