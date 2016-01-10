package pooly

import (
	"net"
	"net/http"
)

func NewHTTPTransport(service *Service) *http.Transport {
	dial := func(network, addr string) (net.Conn, error) {
		c, err := service.GetConn()
		if err != nil {
			return nil, err
		}

		w := &closeWrapper{
			Conn: c.NetConn(),
			conn: c,
		}
		return w, nil
	}

	return &http.Transport{
		Dial:                dial,
		DisableKeepAlives:   true,
		MaxIdleConnsPerHost: 1,
	}
}
