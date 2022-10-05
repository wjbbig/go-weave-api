package go_weave_api

import (
	"fmt"
	"net/http"
	"strings"
)

type Status struct {
	Overview    *Overview
	DNS         []DNSStatus
	Peers       []PeerStatus
	Connections []ConnectionStatus
	Targets     TargetStatus
	IPAM        IPAMStatus
}

type Overview struct {
	Version string
	Router  struct {
		Protocol       string
		Name           string
		Encryption     string
		PeerDiscovery  string
		Targets        string
		Connections    string
		Peers          string
		TrustedSubnets string
	}
	IPAM struct {
		Status        string
		Range         string
		DefaultSubnet string
	}
	DNS struct {
		Domain   string
		Upstream string
		TTL      string
		Entries  string
	}
	Proxy struct {
		Address string
	}
	Plugin struct {
		DriverName string
	}
}

type IPAMStatus struct {
	IPAM string
}

type ConnectionStatus struct {
	Outbound bool
	Address  string
	State    string
	Info     string
	Attrs    map[string]any
}

type DNSStatus struct {
	Hostname    string
	Address     string
	ContainerId string
	Origin      string
}

type PeerStatus struct {
	NodeId      string
	Connections []Connection
}

type Connection struct {
	Outbound bool
	Address  string
	State    string
	NodeId   string
}

type TargetStatus struct {
	targets []string
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

	switch subStatus {
	case "dns":
		status.DNS = parseDNSStatus(statusBytes)
	case "connections":
		status.Connections = parseConnectionStatus(statusBytes)
	case "peers":
		status.Peers = parsePeerStatus(statusBytes)
	case "targets":
		status.Targets = *parseTargetStatus(statusBytes)
	case "ipam":
		status.IPAM = *parseIPAMStatus(statusBytes)
	default:
		status.Overview = parseOverviewStatus(statusBytes)
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
		dnsArgs = removeSpaceElement(dnsArgs)

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

func parseConnectionStatus(data []byte) []ConnectionStatus {
	connSlice := strings.Split(string(data), "\n")
	if len(connSlice) == 0 {
		return nil
	}

	var connStatus []ConnectionStatus
	for _, s := range connSlice {
		if s == "" {
			continue
		}
		connArgs := strings.Split(s, " ")
		connArgs = removeSpaceElement(connArgs)

		status := ConnectionStatus{
			Outbound: connArgs[0] == "->",
			Address:  connArgs[1],
			State:    connArgs[2],
		}
		i := 3
		for ; i < len(connArgs); i++ {
			if strings.Contains(connArgs[i], "=") {
				break
			}
			status.Info = status.Info + connArgs[i] + " "
		}

		if i < len(connArgs)-1 {
			attrs := make(map[string]any)
			for i := 5; i < len(connArgs); i++ {
				if strings.Contains(connArgs[i], "=") {
					args := strings.Split(connArgs[i], "=")
					attrs[args[0]] = args[1]
				}
			}
			status.Attrs = attrs
		}

		connStatus = append(connStatus, status)

	}
	return connStatus
}

func parseTargetStatus(data []byte) *TargetStatus {
	targetArgs := strings.Split(string(data), "\n")
	targetStatus := &TargetStatus{}
	for _, arg := range targetArgs {
		if arg != "" && arg != " " {
			targetStatus.targets = append(targetStatus.targets, strings.TrimSpace(arg))
		}
	}
	return targetStatus
}

func parseIPAMStatus(data []byte) *IPAMStatus {
	ipamArgs := strings.Split(string(data), "\n")

	return &IPAMStatus{
		IPAM: ipamArgs[0],
	}
}

func parseOverviewStatus(data []byte) *Overview {
	overviewArgs := strings.Split(string(data), "\n")
	overviewArgs = removeSpaceElement(overviewArgs)

	overview := &Overview{}
	args := strings.Split(overviewArgs[0], " ")
	args = removeSpaceElement(args)
	overview.Version = args[1]

	var values []string
	for i := 1; i < len(overviewArgs); i++ {
		if strings.Contains(overviewArgs[i], "service") {
			continue
		}
		args := strings.Split(overviewArgs[i], ":")
		values = append(values, strings.TrimSpace(args[1]))
	}

	overview.Router.Protocol = values[0]
	overview.Router.Name = values[1]
	overview.Router.Encryption = values[2]
	overview.Router.PeerDiscovery = values[3]
	overview.Router.Targets = values[4]
	overview.Router.Connections = values[5]
	overview.Router.Peers = values[6]
	overview.Router.TrustedSubnets = values[7]
	overview.IPAM.Status = values[8]
	overview.IPAM.Range = values[9]
	overview.IPAM.DefaultSubnet = values[10]
	overview.DNS.Domain = values[11]
	overview.DNS.Upstream = values[12]
	overview.DNS.TTL = values[13]
	overview.DNS.Entries = values[14]
	overview.Proxy.Address = values[15]
	overview.Plugin.DriverName = values[16]

	return overview
}

func parsePeerStatus(data []byte) []PeerStatus {
	peerSlice := strings.Split(string(data), "\n")
	var peerStatus []PeerStatus
	i := 0
newStatus:
	var status PeerStatus
	for ; i < len(peerSlice); i++ {
		if peerSlice[i] == "" || peerSlice[i] == " " {
			continue
		}
		args := strings.Split(peerSlice[i], " ")
		if len(args) == 1 {
			if status.NodeId != "" {
				peerStatus = append(peerStatus, status)
				goto newStatus
			}
			status = PeerStatus{NodeId: peerSlice[i]}
		} else {
			args = removeSpaceElement(args)
			conn := Connection{
				Outbound: args[0] == "->",
				Address:  args[1],
				NodeId:   args[2],
				State:    args[3],
			}

			status.Connections = append(status.Connections, conn)
		}
	}
	if status.NodeId != "" {
		peerStatus = append(peerStatus, status)
	}
	return peerStatus
}

func removeSpaceElement(s []string) []string {
	for i := 0; i < len(s); i++ {
	REMOVESPACE:
		if s[i] == "" || s[i] == " " {
			s = removeSliceElement(s, i)
			if i <= len(s)-1 {
				goto REMOVESPACE
			}
		}
	}

	return s
}
