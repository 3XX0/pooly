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

Get two connections from the "netcat" service using the _NetDriver_ and the _RoundRobin_ strategy (defaults).
```go
package main

import "fmt"
import "github.com/3XX0/pooly"

func ping(s *pooly.Service) error {
	c, err := s.GetConn()
	if err != nil {
		return err
	}
	defer c.Release(&err, pooly.HostUp)

	_, err = c.NetConn().Write([]byte("ping\n"))
	return err
}

func main() {
	s := pooly.NewService("netcat", nil)
	defer s.Close()

	s.Add("127.0.0.1:1234")
	s.Add("127.0.0.1:12345")

	if err := ping(s); err != nil {
		fmt.Println(err)
		return
	}
	if err := ping(s); err != nil {
		fmt.Println(err)
		return
	}
}
```

Metrics
-------

Pooly exports useful service metrics provided that a [statsd](https://github.com/bitly/statsdaemon) server is running and the option _ServiceConfig.StatsdAddr_ is set accordingly.

The following metrics are available:

metric              | description
--------------------|------------------------------------
hosts.score         | average score of the service hosts (in percentage)
hosts.count         | number of hosts registered to the service
conns.count         | number of connections spawned by the service
conns.fails         | number of connection failures (temporary included)
conns.put.count     | number of _Release_ performed
conns.get.count     | number of _GetConn_ performed
conns.get.delay     | average delay before receiving a connection from the service (in millisecond)
conns.get.fails     | number of _GetConn_ failures
conns.active.period | average time during which a connection was active (in millisecond)

**Example of a grafana dashboard using [vizu](https://github.com/3XX0/vizu)**

![](https://github.com/3XX0/pooly/raw/master/misc/grafana.png)

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
