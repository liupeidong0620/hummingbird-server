package server

import (
	"net"
	"net/url"
)

type Base struct {
	Listen net.Listener

	Url *url.URL

	CertFile, KeyFile string

	Network string
}

func NewBase(network string, addr string) (*Base, error) {
	var err error

	base := new(Base)
	base.Network = network
	base.Url, err = url.Parse(addr)
	if err != nil {
		return nil, err
	}

	base.Listen, err = net.Listen(network, base.Url.Host)
	if err != nil {
		return nil, err
	}

	return base, err
}

func (b *Base) Stop() error {
	if b.Listen != nil {
		return b.Listen.Close()
	}
	return nil
}
