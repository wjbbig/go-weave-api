package go_weave_api

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	weavePort           = 6783
	weaveHttpPort       = 6784
	weaveStatusPort     = 6782
	defaultWeaveVersion = "2.8.1"
)

type Weave struct {
	dockerCli            *docker.Client
	cni                  *CNIBuilder
	dns                  *DNSServer
	clientTLS            *tlsCerts
	containerID          string
	address              string
	host                 string
	resume               bool
	noDNS                bool
	withoutDNS           bool
	httpPort             int
	statusPort           int
	port                 int
	password             string
	local                bool
	version              string
	tlsVerify            bool
	ipAllocInit          string
	ipRange              string
	ipAllocDefaultSubnet string
	noDefaultIpAlloc     bool
	noRewriteHost        bool
	dockerPort           int
	nickname             string
	name                 string
	restartPolicy        string
	enablePlugin         bool
	enableProxy          bool
	discovery            bool
	disableFastDP        bool
	noMultiRouter        bool
	hostnameFromLabel    string
	hostnameMatch        string
	rewriteInspect       bool
	hostnameReplacement  string
	mtu                  int
	trustedSubnets       string
	logLevel             string
	token                string
	peers                []string
}

type tlsCerts struct {
	cacertPath string
	certPath   string
	keyPath    string
}

func NewWeaveNode(address string, opts ...Option) (*Weave, error) {
	nickname := randString()
	w := &Weave{
		dns:           &DNSServer{Search: "weave.local"},
		address:       address,
		port:          weavePort,
		httpPort:      weaveHttpPort,
		statusPort:    weaveStatusPort,
		version:       defaultWeaveVersion,
		ipRange:       "10.32.0.0/12",
		local:         localhost(address),
		nickname:      nickname,
		restartPolicy: "always",
		discovery:     true,
		logLevel:      "info",
	}

	for _, opt := range opts {
		opt(w)
	}
	// create docker client
	var dopts []docker.Opt
	if w.local && localhost(address) {
		dopts = append(dopts, docker.FromEnv)
	} else {
		if w.dockerPort == 0 {
			return nil, errors.New("the docker port is required when the address is not local")
		}
		dopts = append(dopts, docker.WithHost(fmt.Sprintf("tcp://%s:%d", address, w.dockerPort)))
		if w.tlsVerify {
			dopts = append(dopts, docker.WithTLSClientConfig(w.clientTLS.cacertPath,
				w.clientTLS.certPath, w.clientTLS.keyPath))
		}
	}
	cli, err := docker.NewClientWithOpts(dopts...)
	if err != nil {
		return nil, err
	}
	w.dockerCli = cli

	if err := w.checkOverlap(w.ipRange, "weave"); err != nil {
		return nil, err
	}
	if !w.dns.Disabled && w.dns.Address == "" {
		//result, err := w.runWeaveExec("bridge-ip", "docker0")
		//if err != nil {
		//	return nil, err
		//}
		resp, err := w.dockerCli.NetworkInspect(context.Background(), "bridge", types.NetworkInspectOptions{})
		if err != nil {
			return nil, err
		}

		w.dns.Address = fmt.Sprintf("%s:53", resp.IPAM.Config[0].Gateway)
	}
	w.cni = NewCNIBuilder(w.dockerCli, w.version)
	return w, nil
}

// ==================== network helper =====================

func (w *Weave) Launch() error {
	// 1. install cni plugin
	if err := w.cni.installCNIPlugin(); err != nil {
		return err
	}
	// validate brige type
	if err := w.validateBridgeType(); err != nil {
		return err
	}

	// 2. create weavedb volume
	if err := w.createWeaveVolumeFrom(); err != nil {
		return err
	}
	// 3. create weave container
	containerId, err := w.createWeaveContainer()
	if err != nil {
		return err
	}
	w.containerID = containerId
	// 4. start the container
	return w.startWeaveContainer()
}

func (w *Weave) Stop() error {
	result, err := w.runWeaveExec("remove-plugin-network", "weave")
	if err != nil {
		return err
	}
	if string(result) != "" {
		return errors.New(string(result))
	}

	timeout := time.Minute
	if err := w.dockerCli.ContainerStop(context.Background(), w.containerID, &timeout); err != nil {
		return err
	}
	_, err = w.runRemoteCmdWithContainer("conntrack", "-D", "-p", "udp", "--dport", strconv.Itoa(w.port))
	return err
}

func (w *Weave) Connect(replace bool, peer ...string) error {
	values := url.Values{"peer": peer}
	values.Add("replace", strconv.FormatBool(replace))
	result, err := callWeave(http.MethodPost, fmt.Sprintf("http://%s:%d/connect", w.address, w.httpPort),
		strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}
	fmt.Println(string(result))
	return nil
}

func (w *Weave) SetupCNI() error {
	return w.Setup()
}

func (w *Weave) Setup() error {
	return w.cni.installCNIPlugin()
}

func (w *Weave) Forget(peer ...string) error {
	values := url.Values{"peer": peer}
	_, err := callWeave(http.MethodPost, fmt.Sprintf("http://%s:%d/forget", w.address, w.httpPort),
		strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}

	return nil
}

func (w *Weave) startWeaveContainer() error {
	if err := w.dockerCli.ContainerStart(context.Background(), w.containerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (w *Weave) createWeaveContainer() (string, error) {
	httpAddr := fmt.Sprintf("0.0.0.0:%d", w.httpPort)

	containerCmds, containerMounts, err := w.collectCmdsAndMounts()
	containerCmds = append(containerCmds, w.peers...)
	resp, err := w.dockerCli.ContainerCreate(context.Background(), &container.Config{
		Image: fmt.Sprintf("weaveworks/weave:%s", w.version),
		Env: []string{
			fmt.Sprintf("WEAVE_PASSWORD=%s", w.password),
			fmt.Sprintf("EXEC_IMAGE=weaveworks/weaveexec:%s", w.version),
			fmt.Sprintf("WEAVE_HTTP_ADDR=%s", httpAddr),
		},
		Cmd: containerCmds,
	}, &container.HostConfig{
		NetworkMode: "host",
		//PortBindings:  nat.PortMap{},  // weave uses iptable to expose port
		RestartPolicy: container.RestartPolicy{Name: w.restartPolicy},
		VolumesFrom:   []string{"weavedb", fmt.Sprintf("weavevolumes-%s", w.version)},
		PidMode:       "host",
		Privileged:    true,
		Mounts:        containerMounts,
	}, nil, nil, "weave")
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func (w *Weave) getDockerTLSArgs() (tls *tlsCerts, err error) {
	result, err := w.runWeaveExec("docker-tls-args")
	if err != nil {
		return
	}
	if result == nil {
		return nil, errors.New("can not get docker tls args")
	}

	args := bytes.Split(result, []byte(" "))
	for _, arg := range args {
		if strings.Contains(string(arg), "tlsverify") {
			continue
		}
		if strings.Contains(string(arg), "tlscacert") {
			cacertArgs := strings.Split(string(arg), "=")
			tls.cacertPath = cacertArgs[1]
		}
		if strings.Contains(string(arg), "tlscert") {
			certArgs := strings.Split(string(arg), "=")
			tls.certPath = certArgs[1]
		}
		if strings.Contains(string(arg), "tlskey") {
			keyArgs := strings.Split(string(arg), "=")
			tls.keyPath = keyArgs[1]
		}
	}

	return
}

func (w *Weave) createWeaveVolumeFrom() error {
	if err := createVolumeContainer(w.dockerCli, "weavedb", "weaveworks/weavedb:latest",
		"weavevolumes", "/weavedb"); err != nil {
		return err
	}

	if err := createVolumeContainer(w.dockerCli, fmt.Sprintf("weavevolumes-%s", w.version),
		fmt.Sprintf("weaveworks/weaveexec:%s", w.version),
		"weavevolumes", "/w", "/w-noop", "/w-nomcast"); err != nil {
		return err
	}
	return nil
}

// ====================DNS Helpers=====================

func (w *Weave) AddContainerDNS(containerId, fqdn string) error {
	if w.dns.Disabled {
		return errors.New("weaveDNS disabled")
	}
	ip, err := getContainerWeaveIP(w.dockerCli, containerId)
	if err != nil {
		return err
	}
	id, err := getContainerIdByName(w.dockerCli, containerId)
	if err != nil {
		return err
	}
	return w.dns.addWeaveDNS(id, ip, fqdn, false)
}

func (w *Weave) AddExternalDNS(ip, fqdn string) error {
	return w.dns.addWeaveDNS("", ip, fqdn, true)
}

func (w *Weave) LookupDNS(hostname string) ([]string, error) {
	// find dns from weave router,
	// no dig command here
	var ips []string
	if !w.dns.Disabled {
		status, err := w.Status("dns")
		if err != nil {
			return nil, err
		}
		for _, dn := range status.DNS {
			if hostname == dn.Hostname {
				ips = append(ips, hostname)
			} else {
				before, _, found := strings.Cut(hostname, w.dns.Search)
				if found {
					if before == dn.Hostname {
						ips = append(ips, hostname)
					}
				}
			}
		}
	}
	result, err := net.LookupIP(hostname)
	// return empty slice, not the error
	if err != nil {
		return ips, nil
	}
	for _, ip := range result {
		ips = append(ips, ip.String())
	}
	return ips, nil
}

func (w *Weave) RemoveContainerDNS(containerId string, fqdn ...string) error {
	var f string
	if len(fqdn) != 0 {
		f = fqdn[0]
	}
	ip, err := getContainerWeaveIP(w.dockerCli, containerId)
	if err != nil {
		return err
	}
	id, err := getContainerIdByName(w.dockerCli, containerId)
	if err != nil {
		return err
	}
	return w.dns.removeWeaveDNS(ip, id, f, false)
}

func (w *Weave) RemoveExternalDNS(ip, fqdn string) error {
	return w.dns.removeWeaveDNS("", ip, fqdn, true)
}

func (w *Weave) RemovePeer(peers ...string) error {
	if len(peers) == 0 {
		return errors.New("should provide at least 1 peer")
	}
	for _, peer := range peers {
		resp, err := callWeave(http.MethodDelete, fmt.Sprintf("/peer/%s", peer), nil)
		if err != nil {
			return err
		}
		fmt.Println(string(resp))
	}

	return nil
}

func (w *Weave) Prime() error {
	_, err := callWeave(http.MethodGet, "/ring", nil)
	return err
}

func (w *Weave) checkOverlap(ipRange, bridge string) error {
	result, err := w.runWeaveExec("netcheck", ipRange, bridge)
	if err != nil {
		return err
	}
	if result != nil && len(result) != 0 {
		return errors.Errorf("ipalloc-range %s overlaps with existing route on host", ipRange)
	}
	return nil
}

func (w *Weave) detectBridgeType() (string, error) {
	result, err := w.runWeaveExec("detect-bridge-type", "weave", "datapath")
	if err != nil {
		return "", err
	}
	return string(result), nil
}

func (w *Weave) validateBridgeType() error {
	bridgeType, err := w.detectBridgeType()
	if err != nil {
		return err
	}
	if bridgeType != "bridge" && bridgeType != "bridged_fastdp" && bridgeType != "fastdp" {
		return nil
	}

	if bridgeType == "bridge" && !w.disableFastDP {
		return errors.New("WEAVE_NO_FASTDP is not set, but there is already a bridge present of the wrong type for fast datapath.  Please do 'weave reset' to remove the bridge first.")
	}

	if bridgeType != "bridge" && w.disableFastDP {
		return errors.New("WEAVE_NO_FASTDP is set, but there is already a weave fast datapath bridge present.  Please do 'weave reset' to remove the bridge first.")
	}

	return nil
}

func (w *Weave) runWeaveExec(cmd ...string) ([]byte, error) {
	execCmd := []string{"/usr/bin/weaveutil"}
	execCmd = append(execCmd, cmd...)
	return w.createWeaveExecContainer(execCmd...)
}

// runRemoteCmdWithContainer uses to run iptables, conntrack ...
func (w *Weave) runRemoteCmdWithContainer(cmd ...string) ([]byte, error) {
	return w.createWeaveExecContainer(cmd...)
}

func (w *Weave) createWeaveExecContainer(cmd ...string) ([]byte, error) {
	resp, err := w.dockerCli.ContainerCreate(context.Background(), &container.Config{
		Entrypoint: cmd,
		Image:      fmt.Sprintf("weaveworks/weaveexec:%s", w.version),
	}, &container.HostConfig{
		Privileged:  true,
		NetworkMode: "host",
		PidMode:     "host",
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/var/run/docker.sock",
				Target: "/var/run/docker.sock",
			},
			{
				Type:   mount.TypeBind,
				Source: "/",
				Target: "/host/",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return nil, err
	}

	if err := w.dockerCli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}); err != nil {
		return nil, err
	}

	statusCh, errCh := w.dockerCli.ContainerWait(context.Background(), resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return nil, errors.Errorf("container start failed, err=%s", err.Error())
	case <-statusCh:
	}

	// get the result of command
	out, err := w.dockerCli.ContainerLogs(context.Background(), resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return nil, errors.Errorf("get container log failed, err=%s", err.Error())
	}
	data, err := io.ReadAll(out)
	if err != nil {
		return nil, err
	}

	// remove the container, ignore the error
	_ = w.dockerCli.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	return data, nil
}

func (w *Weave) collectCmdsAndMounts() ([]string, []mount.Mount, error) {
	httpAddr := fmt.Sprintf("0.0.0.0:%d", w.httpPort)
	statusAddr := fmt.Sprintf("0.0.0.0:%d", w.statusPort)

	var containerCmds []string
	var containerMounts []mount.Mount

	resolvConfPath, err := w.getRemoteResolvConfPath()
	if err != nil {
		return nil, nil, err
	}

	resolvConfDir, resolvConfName := filepath.Split(resolvConfPath)

	containerCmds = []string{
		"--port", strconv.Itoa(w.port),
		"--nickname", w.nickname,
		"--host-root=/host",
		"--weave-bridge", "weave",
		"--datapath", "datapath",
		"--ipalloc-range", w.ipRange,
		"--dns-listen-address", w.dns.Address,
		"--http-addr", httpAddr,
		"--status-addr", statusAddr,
		"--resolv-conf", fmt.Sprintf("/var/run/weave/etc/%s", resolvConfName),
		"--docker-bridge", "docker0",
		"-H", "unix:///var/run/weave/weave.sock",
		fmt.Sprintf("--log-level=%s", w.logLevel),
	}

	if w.enablePlugin {
		containerCmds = append(containerCmds, "--plugin")
	}
	if w.trustedSubnets != "" {
		containerCmds = append(containerCmds, "--trusted-subnets", w.trustedSubnets)
	}
	if w.name != "" {
		containerCmds = append(containerCmds, "--name", w.name)
	}
	if w.host != "" {
		containerCmds = append(containerCmds, "--host", w.host)
	}
	if w.noRewriteHost {
		containerCmds = append(containerCmds, "--no-rewrite-hosts")
	}
	if w.rewriteInspect {
		containerCmds = append(containerCmds, "--rewrite-inspect")
	}
	if w.noDefaultIpAlloc {
		containerCmds = append(containerCmds, "--no-default-ipalloc")
	}
	if w.enableProxy {
		containerCmds = append(containerCmds, "--proxy")
	}
	if w.dns.Disabled {
		containerCmds = append(containerCmds, "--no-dns")
	}
	if w.withoutDNS {
		containerCmds = append(containerCmds, "--without-dns")
	}
	if !w.discovery {
		containerCmds = append(containerCmds, "--no-discovery")
	}
	if w.ipAllocDefaultSubnet != "" {
		containerCmds = append(containerCmds, "--ipalloc-default-subnet", w.ipAllocDefaultSubnet)
	}
	if w.ipAllocInit != "" {
		containerCmds = append(containerCmds, "--ipalloc-init", w.ipAllocInit)
	}
	if w.noMultiRouter {
		containerCmds = append(containerCmds, "--no-multicast-route")
	}
	if w.hostnameMatch != "" {
		containerCmds = append(containerCmds, "--hostname-match", w.hostnameMatch)
	}
	if w.hostnameReplacement != "" {
		containerCmds = append(containerCmds, "--hostname-replacement", w.hostnameReplacement)
	}
	if w.hostnameFromLabel != "" {
		containerCmds = append(containerCmds, "--hostname-from-label", w.hostnameFromLabel)
	}
	if w.token != "" {
		containerCmds = append(containerCmds, "--token", w.token)
	}
	if w.disableFastDP {
		containerCmds = append(containerCmds, "--no-fastdp")
	}
	if w.mtu > 0 {
		containerCmds = append(containerCmds, "--mtu", strconv.Itoa(w.mtu))
	}

	if w.tlsVerify {
		containerCmds = append(containerCmds, "--tlsverify",
			"--tlscacert", "/home/weave/tls/ca.pem",
			"--tlscert", "/home/weave/tls/cert.pem",
			"--tlskey", "/home/weave/tls/key.pem",
		)

		tls, err := w.getDockerTLSArgs()
		if err != nil {
			return nil, nil, err
		}
		containerMounts = append(containerMounts,
			mount.Mount{Type: mount.TypeBind, Source: tls.cacertPath, Target: "/home/weave/tls/ca.pem"},
			mount.Mount{Type: mount.TypeBind, Source: tls.certPath, Target: "/home/weave/tls/cert.pem"},
			mount.Mount{Type: mount.TypeBind, Source: tls.keyPath, Target: "/home/weave/tls/key.pem"},
		)
	}

	containerMounts = []mount.Mount{
		{Type: mount.TypeBind, Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"},
		{Type: mount.TypeBind, Source: "/var/run/weave", Target: "/var/run/weave"},
		{Type: mount.TypeBind, Source: "/run/docker/plugins", Target: "/run/docker/plugins"},
		{Type: mount.TypeBind, Source: "/etc", Target: "/host/etc"},
		{Type: mount.TypeBind, Source: "/var/lib/dbus", Target: "/host/var/lib/dbus"},
		{Type: mount.TypeBind, Source: resolvConfDir[5:], Target: "/var/run/weave/etc"},
		//{Type: mount.TypeBind, Source: "/", Target: "/host"},
	}

	return containerCmds, containerMounts, nil
}

func (w *Weave) getRemoteResolvConfPath() (string, error) {
	result, err := w.runRemoteCmdWithContainer("readlink", "-f", "/host/etc/resolv.conf")
	if err != nil {
		return "", err
	}
	i := 0
	for ; i < len(result); i++ {
		if string(result[i]) == "/" {
			break
		}
	}
	return string(result[i:]), nil
}

func (w *Weave) Close() error {
	return w.dockerCli.Close()
}
