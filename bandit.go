package pooly

import (
	"math"
	"math/rand"
	"sync"
)

// SoftMax strategy varies host selection probabilities as a graded function of their estimated scores.
// The temperature parameter is used to tweak the algorithm behavior:
// high temperature (+inf) means that all hosts will have nearly the same probability of being selected (equiprobable)
// low temperature (+0) favors a greedy selection and will tend to select hosts having the highest scores
type SoftMax struct {
	temperature float32
}

// NewSoftMax creates a new SoftMax bandit strategy.
func NewSoftMax(temperature float32) *SoftMax {
	return &SoftMax{temperature}
}

// Select implements the Selecter interface.
func (s *SoftMax) Select(hosts map[string]*Host) *Host {
	var sum, prob float64
	exp := make(map[*Host]float64, len(hosts))

	for _, h := range hosts {
		score := h.Score()
		if score < 0 { // no score recorded
			exp[h] = 0
			continue
		}
		exp[h] = math.Exp(score / float64(s.temperature))
		sum += exp[h]
	}

	p := rand.Float64()
	for _, h := range hosts {
		if sum == 0 {
			return h
		}
		prob += exp[h] / sum // cumulative probability
		if prob > p {
			return h
		}
	}
	return nil
}

// EpsilonGreedy strategy selects generally the host having the highest score (greedy) but every once in a while
// it will randomly explore for other alternatives.
// The epsilon parameter (0-1) defines the proportion that the exploration phase occupies (e.g 1 for 100%).
type EpsilonGreedy struct {
	epsilon float32
}

// NewEpsilonGreedy creates a new EpsilonGreedy bandit strategy.
func NewEpsilonGreedy(epsilon float32) *EpsilonGreedy {
	return &EpsilonGreedy{epsilon}
}

// Select implements the Selecter interface.
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

// RoundRobin strategy selects hosts in circular manner with every request returning the next host in line.
type RoundRobin struct {
	sync.Mutex
	nextSchedule  int64
	nextAvailSlot int64
}

// NewRoundRobin creates a new RoundRobin bandit strategy.
func NewRoundRobin() *RoundRobin {
	return new(RoundRobin)
}

// Select implements the Selecter interface.
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
