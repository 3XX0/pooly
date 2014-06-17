package pooly

import (
	"sync"
)

type SoftMax struct {
}

type EpsilonGreedy struct {
}

type RoundRobin struct {
	sync.Mutex
	nextSchedule  int64
	nextAvailSlot int64
}

func NewRoundRobin() *RoundRobin {
	return new(RoundRobin)
}

func (r *RoundRobin) Select(hosts map[string]*Host) (host *Host) {
	var offset int64
	var found bool

	// XXX score is not used, use it to attribute round robin scheduling instead
	// we don't need proper synchronization since score memoization isn't running here
	r.Lock()
	for _, h := range hosts {
		if h.score < 0 {
			h.score = float64(r.nextAvailSlot)
			r.nextAvailSlot++
		}
		if int64(h.score) == r.nextSchedule {
			offset = 1
			host = h
			found = true
		}
		if !found {
			// Find the next best schedule
			o := int64(h.score) - r.nextSchedule
			if o < 0 {
				o = r.nextAvailSlot + o
			}
			if offset == 0 || o < offset {
				offset = o + 1
				host = h
			}
		}
	}
	r.nextSchedule = (r.nextSchedule + offset) % r.nextAvailSlot
	r.Unlock()
	return
}
