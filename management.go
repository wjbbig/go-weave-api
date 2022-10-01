package go_weave_api

import (
	"fmt"
	"github.com/pkg/errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func (w *Weave) Attach(containerId string, withoutDNS, rewriteHost, noMulticastRoute bool, hosts []string, addr ...string) error {
	cidrArgs := collectValidCIDR(addr)

	containerId, err := getContainerIdByName(w.dockerCli, containerId)
	if err != nil {
		return err
	}

	_, allCIDRs, err := w.ipamCIDRs("allocate", containerId, cidrArgs)
	if err != nil {
		return err
	}

	if rewriteHost {
		args := []string{"rewrite-etc-hosts", containerId, fmt.Sprintf("weaveworks/weaveexec:%s", w.version)}
		args = append(args, allCIDRs...)
		args = append(args, hosts...)
		if _, err = w.runWeaveExec(args...); err != nil {
			return err
		}
	}
	var attachArgs []string
	if noMulticastRoute {
		attachArgs = append(attachArgs, "--no-multicast-route")
	}
	awsvpc, err := detectAWSVPC(fmt.Sprintf("%s:%d", w.address, w.httpPort))
	if err != nil {
		return err
	}
	if awsvpc {
		attachArgs = append(attachArgs, "--keep-tx-on")
	}
	attachArgs = append([]string{"attach-container"}, attachArgs...)
	attachArgs = append(attachArgs, containerId, "weave")
	attachArgs = append(attachArgs, allCIDRs...)
	if _, err = w.runWeaveExec(attachArgs...); err != nil {
		return err
	}

	if !withoutDNS {
		containerFqdnBytes, err := w.runWeaveExec("container-fqdn", containerId)
		if err != nil {
			return err
		}

		containerFqdn := string(containerFqdnBytes[8:])

		containerName := strings.Split(containerFqdn, ".")[0]
		if containerName != containerFqdn || fmt.Sprintf("%s.", containerName) != containerFqdn {
			for _, cidr := range allCIDRs {
				addr, _, found := strings.Cut(cidr, "/")
				if !found {
					// the cidr is invalid, ignore it
					continue
				}
				values := url.Values{}
				values.Add("fqdn", containerFqdn)
				values.Add("check-alive", "true")
				if _, err := callWeave(http.MethodPut, fmt.Sprintf("http://%s:%d/name/%s/%s",
					w.address, w.httpPort, containerId, addr), strings.NewReader(values.Encode())); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (w *Weave) Detach(containerId string, addr ...string) error {
	cidrArgs := collectValidCIDR(addr)

	containerId, err := getContainerIdByName(w.dockerCli, containerId)
	if err != nil {
		return err
	}

	ipamCIDRs, all, err := w.ipamCIDRs("lookup", containerId, cidrArgs)
	if err != nil {
		return err
	}

	execArgs := []string{"detach-container"}
	execArgs = append(execArgs, all...)
	_, err = w.runWeaveExec(execArgs...)
	if err != nil {
		return err
	}

	containerFqdnBytes, err := w.runWeaveExec("container-fqdn", containerId)
	if err != nil {
		return err
	}

	containerFqdn := string(containerFqdnBytes[8:])

	containerName := strings.Split(containerFqdn, ".")[0]
	if containerName != containerFqdn || fmt.Sprintf("%s.", containerName) != containerFqdn {
		for _, cidr := range all {
			addr, _, found := strings.Cut(cidr, "/")
			if !found {
				// the cidr is invalid, ignore it
				continue
			}
			if _, err := callWeave(http.MethodDelete, fmt.Sprintf("http://%s:%d/name/%s/%s?fqdn=%s",
				w.address, w.httpPort, containerId, addr, containerFqdn), nil); err != nil {
				return err
			}
		}
	}

	for _, cidr := range ipamCIDRs {
		addr, _, found := strings.Cut(cidr, "/")
		if !found {
			// the cidr is invalid, ignore it
			continue
		}
		if _, err = callWeave(http.MethodDelete, fmt.Sprintf("http://%s:%d/ip/%s/%s",
			w.address, w.httpPort, containerId, addr), nil); err != nil {
			return err
		}
	}
	return nil
}

func (w *Weave) Expose(fqdn string, withoutMasquerade bool, addr ...string) ([]string, error) {
	cidrArgs := collectValidCIDR(addr)

	_, allCIDRs, err := w.ipamCIDRs("allocate_no_check_alive", "weave:expose", cidrArgs)
	if err != nil {
		return nil, err
	}
	var skipNAT string
	if !withoutMasquerade {
		skipNAT = "?skipNAT=true"
	}

	for _, cidr := range allCIDRs {
		_, err := callWeave(http.MethodPost, fmt.Sprintf("http://%s:%d/expose/%s%s",
			w.address, w.httpPort, cidr, skipNAT), nil)
		if err != nil {
			return nil, err
		}

		if fqdn != "" {
			if err := w.dns.addWeaveDNS("weave:expose", cidr, fqdn, true); err != nil {
				return nil, err
			}
		}
	}

	return allCIDRs, nil
}

func (w *Weave) Hide(addr ...string) ([]string, error) {
	cidrArgs := collectValidCIDR(addr)
	ipamCIDRs, allCIDRs, err := w.ipamCIDRs("lookup", "weave:expose", cidrArgs)
	if err != nil {
		return nil, err
	}

	iptablesCmd := `
ip addr del dev weave %[1]s && \
iptables -w -t nat -C WEAVE -d %[1]s ! -s %[1]s -j MASQUERADE && \
iptables -w -t nat -D WEAVE -d %[1]s ! -s %[1]s -j MASQUERADE && \
iptables -w -t nat -C WEAVE -s %[1]s ! -d %[1]s -j MASQUERADE && \
iptables -w -t nat -D WEAVE -s %[1]s ! -d %[1]s -j MASQUERADE && \
iptables -w -t filter -C WEAVE_EXPOSE -d %[1]s -j ACCEPT && \
iptables -w -t filter -D WEAVE_EXPOSE -d %[1]s -j ACCEPT
`

	for _, cidr := range allCIDRs {
		// forget errors
		_, _ = w.runRemoteCmdWithContainer("sh", "-c", fmt.Sprintf(iptablesCmd, cidr))
	}

	for _, cidr := range ipamCIDRs {
		addr, _, found := strings.Cut(cidr, "/")
		if !found {
			// the cidr is invalid, ignore it
			continue
		}
		_, err := callWeave(http.MethodDelete, fmt.Sprintf("http://%s:%d/ip/weave:expose/%s",
			w.address, w.httpPort, addr), nil)
		if err != nil {
			return nil, err
		}
	}
	return allCIDRs, nil
}

func (w *Weave) ipamCIDRs(funcName string, containerId string, cidrArgs []string) ([]string, []string, error) {
	var method, checkAlive string
	baseURL := fmt.Sprintf("%s:%d", w.address, w.httpPort)
	switch funcName {
	case "lookup":
		method = http.MethodGet
	case "allocate_no_check_alive":
		method = http.MethodPost
	case "allocate":
		method = http.MethodPost
		checkAlive = "?check-alive=true"
		detected, err := detectAWSVPC(baseURL)
		if err != nil {
			return nil, nil, err
		}
		if !detected && restArgLen(containerId, cidrArgs) > 2 {
			return nil, nil, errors.New("Error: no IP addresses or subnets may be specified in AWSVPC mode")
		}
	}
	if len(cidrArgs) == 0 {
		cidrArgs = append(cidrArgs, "net:default")
	}
	var ipamCIDRs, allCIDRs []string
	for _, arg := range cidrArgs {
		var ipamUrl string
		if strings.Contains(arg, "net:") {
			if arg == "net:default" {
				ipamUrl = fmt.Sprintf("/ip/%s", containerId)
			} else {
				_, address, _ := strings.Cut(arg, "net:")
				ipamUrl = fmt.Sprintf("/ip/%s/%s", containerId, address)
			}
			result, err := callWeave(method, fmt.Sprintf("http://%s%s%s", baseURL, ipamUrl, checkAlive), nil)
			if err != nil {
				return nil, nil, err
			}
			ipamCIDRs = append(ipamCIDRs, string(result))
			allCIDRs = append(allCIDRs, string(result))
		} else {
			if method == http.MethodPost {
				if err := w.checkOverlap(arg, "weave"); err != nil {
					return nil, nil, err
				}
				_, err := callWeave(http.MethodPut, fmt.Sprintf("http://%s/ip/%s%s%s", baseURL, containerId, arg, checkAlive), nil)
				if err != nil {
					return nil, nil, err
				}
			}
			allCIDRs = append(allCIDRs, arg)
		}
	}

	return ipamCIDRs, allCIDRs, nil
}

func restArgLen(containerId string, args []string) int {
	num := 0
	if containerId != "" {
		num++
	}
	num += len(args)
	return num
}

func detectAWSVPC(baseURL string) (bool, error) {
	result, err := callWeave(http.MethodGet, fmt.Sprintf("http://%s/ipinfo/tracker", baseURL), nil)
	if err != nil {
		return false, err
	}
	return string(result) != "awsvpc", nil
}

func collectValidCIDR(cidrs []string) []string {
	var cidrArgs []string
	for _, s := range cidrs {
		if s == "net:default" {
			cidrArgs = append(cidrArgs, "net:default")
			continue
		}
		if isCIDR(s) {
			cidrArgs = append(cidrArgs, s)
		}
	}

	return cidrArgs
}

func isCIDR(addr string) bool {
	_, naddr, found := strings.Cut(addr, "net:")
	if !found {
		_, naddr, found = strings.Cut(addr, "ip:")
		if !found {
			naddr = addr
		}
	}
	_, _, err := net.ParseCIDR(naddr)
	return err == nil
}
