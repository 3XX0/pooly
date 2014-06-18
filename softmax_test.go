// +build montecarlo_simulation

package pooly

import (
	"code.google.com/p/plotinum/plot"
	"code.google.com/p/plotinum/plotter"
	"image/color"
	"strconv"
	"testing"
	"time"
)

func TestServiceSoftMax(t *testing.T) {
	var (
		horizon      = 250
		temperatures = []float32{0.1, 0.5, 0.9}
		hosts        = map[string]bernouilliExperiment{
			echo1: 0.1,
			echo2: 0.1,
			echo3: 0.9,
		}
	)

	e1 := newEchoServer(t, echo1)
	defer e1.close()
	e2 := newEchoServer(t, echo2)
	defer e2.close()
	e3 := newEchoServer(t, echo3)
	defer e3.close()

	p, err := plot.New()
	if err != nil {
		t.Fatal(err)
	}
	p.Add(plotter.NewGrid())
	p.X.Label.Text = "trials"
	p.Y.Label.Text = "average score"
	p.X.Max = float64(horizon)
	p.Y.Min = 0
	p.Y.Max = 1

	for _, tp := range temperatures {
		means := make(plotter.XYs, horizon)

		s := NewService("echo", &ServiceConfig{
			BanditStrategy:       NewSoftMax(tp),
			MemoizeScoreDuration: 1 * time.Millisecond,
		})
		s.Add(echo1)
		s.Add(echo2)
		s.Add(echo3)
		time.Sleep(1 * time.Millisecond) // wait for propagation

		for i := 0; i < horizon; i++ {
			c, err := s.GetConn()
			if err != nil {
				t.Error(err)
				continue
			}

			if i == 0 {
				means[i].X = 0
				means[i].Y = c.host.Score()
			} else {
				n := means[i-1].X + 1
				m := means[i-1].Y
				means[i].X = float64(i)
				means[i].Y = m + (c.host.Score()-m)/(n+1)
			}

			a, _ := c.Address()
			if err := c.Release(nil, hosts[a].trial()); err != nil {
				t.Error(err)
				continue
			}
			time.Sleep(1 * time.Millisecond) // wait for memoization
		}
		s.Close()

		l, err := plotter.NewLine(means)
		if err != nil {
			t.Fatal(err)
		}
		l.LineStyle.Color = color.RGBA{G: uint8(255 * tp), A: 255}
		p.Legend.Add("temperature "+strconv.FormatFloat(float64(tp), 'f', 1, 32), l)
		p.Add(l)
	}

	if err := p.Save(7, 7, "softmax_test.svg"); err != nil {
		t.Fatal(err)
	}
}
