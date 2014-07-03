package main

import "github.com/3XX0/pooly"

func main() {
	s := pooly.NewService("netcat", nil)
	s.Add("127.0.0.1:1234")

	c, err := s.GetConn()
	if err != nil {
		println("could not get connection")
		return
	}

	_, err = c.NetConn().Write([]byte("ping"))

	if err := c.Release(err, 1); err != nil {
		println("could not release connection")
		return
	}

	s.Close()
}
