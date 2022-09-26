package go_weave_api

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type DNSServer struct {
	weave    *Weave
	Address  string
	Search   string
	Disabled bool
}

func NewDNSServer(address, search string, disabled bool) *DNSServer {
	return &DNSServer{nil, address, search, disabled}
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

func (dns *DNSServer) addWeaveDNS(containerId, cip, fqdn string, external bool) error {
	if !strings.Contains(fqdn, dns.Search) {
		fqdn = fmt.Sprintf("%s.%s", fqdn, dns.Search)
	}

	checkAlive := true
	if external {
		containerId = "weave:extern"
		// if this dns is an external one, ignore checking alive
		checkAlive = false
	}

	values := url.Values{}
	values.Add("fqdn", fqdn)
	values.Add("check-alive", strconv.FormatBool(checkAlive))

	address := fmt.Sprintf("%s:%d", dns.weave.address, dns.weave.httpPort)
	dnsUrl := fmt.Sprintf("http://%s/name/%s/%s", address, containerId, cip)
	_, err := callWeave(http.MethodPut, dnsUrl, bytes.NewReader([]byte(values.Encode())))
	if err != nil {
		return err
	}

	return nil
}

func (dns *DNSServer) removeWeaveDNS(containerId, ip, fqdn string, external bool) error {
	if fqdn != "" && !strings.Contains(fqdn, dns.Search) {
		fqdn = fmt.Sprintf("?fqdn=%s.%s", fqdn, dns.Search)
	}
	if external {
		containerId = "weave:extern"
		if fqdn == "" {
			return errors.New("fqdn is required when removing external dns")
		}
	}
	address := fmt.Sprintf("%s:%d", dns.weave.address, dns.weave.httpPort)
	dnsUrl := fmt.Sprintf("http://%s/name/%s/%s%s", address, containerId, ip, fqdn)

	_, err := callWeave(http.MethodDelete, dnsUrl, nil)
	if err != nil {
		return err
	}
	return nil
}

// Deprecated
func (dns *DNSServer) lookupWeaveDNS() {}
