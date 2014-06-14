package pooly

import (
	"sync"
)

// Predefined scores (all or nothing).
const (
	HostDown float64 = 0
	HostUp           = 1
)

const seriesNum = 60

type serie struct {
	score  float64
	trials uint32
}

type host struct {
	sync.RWMutex
	pool       *Pool
	timeSeries []serie
	timeSlot   int
	score      float64
}

// Update the arithmetic mean of the series with a given score [0,1].
func (s *serie) update(score float64) {
	s.trials++
	s.score = s.score + (score-s.score)/float64(s.trials)
}

func (s *serie) reset() {
	s.score = 0
	s.trials = 0
}

func (h *host) computeScore(c Computer) {
	var score float64

	h.Lock()
	n := len(h.timeSeries)
	m := n * (1 + n) / 2 // arithmetic series

	for i := 1; i <= n; i++ {
		t := (h.timeSlot + i) % n

		// Decay [0,1] is factor of time, we start with the oldest entry
		// from which we get the smallest weight
		decay := float64(i) / float64(m)
		if h.timeSeries[t].trials > 0 {
			score += h.timeSeries[t].score * decay
		} else {
			// XXX no trials recorded, neither promote nor demote the host
			score += 0.5 * decay
		}
	}
	if c != nil {
		score = c.Compute(score) // apply the service score calculator
	}
	h.score = score
	h.Unlock()
}

func (h *host) getScore() (score float64) {
	h.RLock()
	score = h.score
	h.RUnlock()
	return
}

func (h *host) decay() {
	h.Lock()
	// Shift the current time slot
	h.timeSlot = (h.timeSlot + 1) % cap(h.timeSeries)
	if len(h.timeSeries) < cap(h.timeSeries) {
		h.timeSeries = append(h.timeSeries, serie{})
	} else {
		h.timeSeries[h.timeSlot].reset()
	}
	h.Unlock()
}

func (h *host) rate(score float64) {
	h.Lock()
	h.timeSeries[h.timeSlot].update(score)
	h.Unlock()
}

func (h *host) releaseConn(c *Conn, e error, score float64) error {
	if err := h.pool.Put(c, e); err != nil {
		return err
	}
	h.rate(score)
	return nil
}
