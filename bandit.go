package pooly

type SoftMax struct {
}

type EpsilonGreedy struct {
}

type RoundRobin struct {
	nextSchedule, nextAvailSlot int64
}

func NewRoundRobin() *RoundRobin {
	return new(RoundRobin)
}

func (r *RoundRobin) Select(hosts []*host) (host *host) {
	var offset int64

	for _, h := range hosts {
		// XXX score is not used, use it to attribute round robin scheduling instead
		if h.score < 0 {
			h.score = float64(r.nextAvailSlot)
			r.nextAvailSlot++
		}
	}

	for _, h := range hosts {
		if int64(h.score) == r.nextSchedule {
			offset = 1
			host = h
			break
		}
		// Find the next best schedule
		o := int64(h.score) - r.nextSchedule
		if o < 0 {
			o = r.nextAvailSlot - o
		}
		if offset == 0 || o < offset {
			offset = o + 1
			host = h
		}
	}
	r.nextSchedule = (r.nextSchedule + offset) % r.nextAvailSlot
	return
}
