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
