package go_weave_api

import "github.com/pkg/errors"

type DNSServer struct {
	Address  string
	Search   string
	Disabled bool
}

func NewDNSServer(address, search string, disabled bool) *DNSServer {
	return &DNSServer{address, search, disabled}
}

func (dns *DNSServer) getDNSArgs() (map[string]string, error) {
	if dns.Disabled {
		return nil, errors.New("dns server disabled")
	}
	return map[string]string{
		"address": dns.Address,
		"search":  dns.Search,
	}, nil
}

func (dns *DNSServer) getDNSArgsRemote() (map[string]string, error) {
	return nil, nil
}
