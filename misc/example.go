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
