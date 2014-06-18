package pooly

import (
	"math"
	"math/rand"
	"sync"
)

type SoftMax struct {
	temperature float32
}

func NewSoftMax(temperature float32) *SoftMax {
	return &SoftMax{temperature}
}

func (s *SoftMax) Select(hosts map[string]*Host) *Host {
	var sum, prob float64
	exp := make([]float64, len(hosts))

	i := 0
	for _, h := range hosts {
		score := h.Score()
		if score < 0 { // no score recorded
			exp[i] = 0
			continue
		}
		exp[i] = math.Exp(score / float64(s.temperature))
		sum += exp[i]
		i++
	}

	i = 0
	p := rand.Float64()
	for _, h := range hosts {
		if sum == 0 {
			return h
		}
		prob += exp[i] / sum // cumulative probability
		if prob > p {
			return h
		}
		i++
	}
	return nil
}

type EpsilonGreedy struct {
	epsilon float32
}

func NewEpsilonGreedy(epsilon float32) *EpsilonGreedy {
	return &EpsilonGreedy{epsilon}
}

func (e *EpsilonGreedy) Select(hosts map[string]*Host) (host *Host) {
	if rand.Float32() > e.epsilon { // exploit
		var max float64 = -1
		for _, h := range hosts {
			score := h.Score()
			if max = math.Max(max, score); max == score {
				host = h
			}
		}
	} else { // explore
		i, n := 0, rand.Intn(len(hosts))
		for _, h := range hosts {
			if i == n {
				host = h
				break
			}
			i++
		}
	}
	return
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
		if h.score < 0 { // no score recorded
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
