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
	"net/http"
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
	dockerCli     *docker.Client
	cni           *CNIBuilder
	dns           *DNSServer
	clientTLS     *tlsCerts
	containerID   string
	address       string
	httpPort      int
	statusPort    int
	port          int
	password      string
	local         bool
	version       string
	tlsVerify     bool
	ipRange       string
	dockerHost    string
	nickname      string
	restartPolicy string
	enablePlugin  bool
	enableProxy   bool
	discovery     bool
	disableFastDP bool
	peers         []string
}

type tlsCerts struct {
	cacertPath string
	certPath   string
	keyPath    string
}

func NewWeaveNode(address string, opts ...Option) (*Weave, error) {
	nickname := RandString()
	w := &Weave{
		address:       address,
		port:          weavePort,
		httpPort:      weaveHttpPort,
		statusPort:    weaveStatusPort,
		version:       defaultWeaveVersion,
		ipRange:       "10.32.0.0/12",
		local:         true,
		nickname:      nickname,
		restartPolicy: "always",
		discovery:     true,
	}

	for _, opt := range opts {
		opt(w)
	}
	// create docker client
	var dopts []docker.Opt
	if w.local {
		dopts = append(dopts, docker.FromEnv)
	} else {
		dopts = append(dopts, docker.WithHost(w.dockerHost))
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

	w.cni = NewCNIBuilder(w.dockerCli, w.version)
	return w, nil
}

func (w *Weave) Launch() error {
	// 1. install cni plugin
	if err := w.cni.installCNIPlugin(); err != nil {
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
	timeout := time.Minute
	if err := w.dockerCli.ContainerStop(context.Background(), w.containerID, &timeout); err != nil {
		return err
	}
	return nil
}

func (w *Weave) Connect() error {
	return nil
}

func (w *Weave) startWeaveContainer() error {
	if err := w.dockerCli.ContainerStart(context.Background(), w.containerID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	//statusCh, errCh := w.dockerCli.ContainerWait(context.Background(), w.containerID, container.WaitConditionNotRunning)
	//select {
	//case err := <-errCh:
	//	return errors.Errorf("weave container start failed, err=%s", err.Error())
	//case <-statusCh:
	//}

	return nil
}

func (w *Weave) createWeaveContainer() (string, error) {
	httpAddr := fmt.Sprintf("127.0.0.1:%d", w.httpPort)
	statusAddr := fmt.Sprintf("127.0.0.1:%d", w.statusPort)

	var containerCmds []string
	var containerMounts []mount.Mount

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
		"--resolv-conf", "/var/run/weave/etc/resolv.conf",
		"--docker-bridge", "docker0",
		"-H", "unix:///var/run/weave/weave.sock",
	}

	containerMounts = []mount.Mount{
		{Type: mount.TypeBind, Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"},
		{Type: mount.TypeBind, Source: "/var/run/weave", Target: "/var/run/weave"},
		{Type: mount.TypeBind, Source: "/run/docker/plugins", Target: "/run/docker/plugins"},
		{Type: mount.TypeBind, Source: "/etc", Target: "/host/etc"},
		{Type: mount.TypeBind, Source: "/var/lib/dbus", Target: "/host/var/lib/dbus"},
		{Type: mount.TypeBind, Source: "/etc", Target: "/var/run/weave/etc"},
	}

	if w.enablePlugin {
		containerCmds = append(containerCmds, "--plugin")
	}
	if w.enableProxy {
		containerCmds = append(containerCmds, "--proxy")
	}
	if w.dns.Disabled {
		containerCmds = append(containerCmds, "--without-dns")
	}
	if w.tlsVerify {
		containerCmds = append(containerCmds, "--tlsverify",
			"--tlscacert", "/home/weave/tls/ca.pem",
			"--tlscert", "/home/weave/tls/cert.pem",
			"--tlskey", "/home/weave/tls/key.pem",
		)

		tls, err := w.getDockerTLSArgs()
		if err != nil {
			return "", err
		}
		containerMounts = append(containerMounts,
			mount.Mount{Type: mount.TypeBind, Source: tls.cacertPath, Target: "/home/weave/tls/ca.pem"},
			mount.Mount{Type: mount.TypeBind, Source: tls.certPath, Target: "/home/weave/tls/cert.pem"},
			mount.Mount{Type: mount.TypeBind, Source: tls.keyPath, Target: "/home/weave/tls/key.pem"},
		)
	}

	resp, err := w.dockerCli.ContainerCreate(context.Background(), &container.Config{
		Image: fmt.Sprintf("weaveworks/weave:%s", w.version),
		Env: []string{
			fmt.Sprintf("WEAVE_PASSWORD=%s", w.password),
			fmt.Sprintf("EXEC_IMAGE=weaveworks/weaveexec:%s", w.version),
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

func (w *Weave) AddDNS() error {
	if w.dns.Disabled {
		return errors.New("weaveDNS disabled")
	}

	return nil
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

func (w *Weave) LookupDNS(hostname string) (string, error) {
	return "", nil
}

func (w *Weave) DNSRemove(hostname string) error {
	return nil
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
	resp, err := w.dockerCli.ContainerCreate(context.Background(), &container.Config{
		Entrypoint: []string{"/usr/bin/weaveutil"},
		Image:      fmt.Sprintf("weaveworks/weaveexec:%s", w.version),
		Cmd:        cmd,
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

func (w *Weave) Close() error {
	return w.dockerCli.Close()
}
