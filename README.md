Pooly [![Build Status](https://travis-ci.org/3XX0/pooly.svg)](https://travis-ci.org/3XX0/pooly) [![GoDoc](https://godoc.org/github.com/3XX0/pooly?status.png)](http://godoc.org/github.com/3XX0/pooly)
=====

A package inspired by [go-hostpool](https://github.com/bitly/go-hostpool), to manage efficiently pools of connection across multiple hosts. Pooly uses multi-armed bandit algorithms for host selection in order to maximize service connectivity by avoiding faulty hosts.

**Currently supported bandit strategies:**
* Softmax
* Epsilon-greedy
* Round-robin


Basic Usage
-----------

Start two dummy netcat servers.
```sh
nc -l -p 1234 &
nc -l -p 12345 &
```

Get two connections from the "netcat" service (_Round-robin_ is the strategy used by default).
```go
package main

import "github.com/3XX0/pooly"

func main() {
	s := pooly.NewService("netcat", nil)
	defer s.Close()

	s.Add("127.0.0.1:1234")
	s.Add("127.0.0.1:12345")

	c, err := s.GetConn()
	if err != nil {
		println("could not get connection")
		return
	}

	_, err = c.NetConn().Write([]byte("ping\n"))

	if err := c.Release(err, pooly.HostUp); err != nil {
		println("could not release connection")
		return
	}

	c, err = s.GetConn()
	if err != nil {
		println("could not get connection")
		return
	}

	_, err = c.NetConn().Write([]byte("ping\n"))

	if err := c.Release(err, pooly.HostUp); err != nil {
		println("could not release connection")
		return
	}
}
```

Simulations
-----------

In order to test the behavior of different strategies, Monte Carlo simulations may be run through the Go testing framework.

```sh
go test -tags 'montecarlo_simulation' -v pooly
```

**Softmax strategy sample output**

![](https://github.com/3XX0/pooly/raw/master/misc/softmax_test.png "Softmax strategy")

**Epsilon-greedy strategy sample output**

![](https://github.com/3XX0/pooly/raw/master/misc/egreedy_test.png "Epsilon-greedy strategy")
