package go_weave_api

import (
	"fmt"
	"net/http"
	"strings"
)

type Status struct {
	DNS []DNSStatus
}

type Overview struct {
	Version string
}

type DNSStatus struct {
	Hostname    string
	Address     string
	ContainerId string
	Origin      string
}

func (w *Weave) Status(subArgs ...string) (*Status, error) {
	var subStatus string
	statusUrl := fmt.Sprintf("http://%s:%d/status", w.address, w.httpPort)
	if len(subArgs) > 0 {
		subStatus = subArgs[0]
		statusUrl = fmt.Sprintf("%s/%s", statusUrl, subStatus)
	}

	statusBytes, err := callWeave(http.MethodGet, statusUrl, nil)
	if err != nil {
		return nil, err
	}

	status := &Status{}

	// todo
	switch subStatus {
	case "dns":
		status.DNS = parseDNSStatus(statusBytes)
	case "connections":
		parseConnectionsStatus(statusBytes)
	case "peers":

	case "targets":

	case "ipam":

	default:

	}

	return status, nil
}

func parseDNSStatus(data []byte) []DNSStatus {
	dnsSlice := strings.Split(string(data), "\n")
	if len(dnsSlice) == 0 {
		return nil
	}
	var dnsStatus []DNSStatus
	for _, s := range dnsSlice {
		if s == "" {
			continue
		}
		dnsArgs := strings.Split(s, " ")

		for i := 0; i < len(dnsArgs); i++ {
		REMOVESPACE:
			if dnsArgs[i] == "" || dnsArgs[i] == " " {
				dnsArgs = removeSliceElement(dnsArgs, i)
				goto REMOVESPACE
			}
		}

		status := DNSStatus{
			Hostname:    strings.TrimSpace(dnsArgs[0]),
			Address:     strings.TrimSpace(dnsArgs[1]),
			ContainerId: strings.TrimSpace(dnsArgs[2]),
			Origin:      strings.TrimSpace(dnsArgs[3]),
		}
		dnsStatus = append(dnsStatus, status)

	}
	return dnsStatus
}

func parseConnectionsStatus(data []byte) string {

	return ""
}
