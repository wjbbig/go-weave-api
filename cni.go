package go_weave_api

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	confListDirPath = "/etc/cni/net.d"
	confListName    = "10-weave.conflist"
	pluginPath      = "/opt/cni/bin"
	ipamLinkName    = "weave-ipam"
	netLinkName     = "weave-net"
	pluginName      = "weave-plugin"
	confList        = `{
    "cniVersion": "0.3.0",
    "name": "weave",
    "plugins": [
        {
            "name": "weave",
            "type": "weave-net",
            "hairpinMode": true
        },
        {
            "type": "portmap",
            "capabilities": {"portMappings": true},
            "snat": true
        }
    ]
}`
)

const installCNI = `
mkdir -p %s && \
mkdir -p %s && \
rm -rf %s/%s && \
rm -rf %s/%s && \
rm -rf %s/%s && \
cp /usr/bin/weaveutil %s/%s && \
cp /usr/bin/weaveutil %s/%s && \
cat > %s/%s <<EOF 
%s
EOF
`

type CNIBuilder struct {
	cli     *docker.Client
	version string
}

func NewCNIBuilder(cli *docker.Client, version string) *CNIBuilder {
	if version == "" {
		version = "latest"
	}
	return &CNIBuilder{
		cli:     cli,
		version: version,
	}
}

func (cni *CNIBuilder) installCNIPlugin() error {
	// cp weave-plugin from weaveexec image
	return cni.generatePluginWithDocker()
	// make symlink of weave-plugin
	//if err := buildCNIPluginSymlink(cni.version); err != nil {
	//	return err
	//}
	//
	//return writeCNIConf()
}

func (cni *CNIBuilder) generatePluginWithDocker() error {
	resp, err := cni.cli.ContainerCreate(context.Background(), &container.Config{
		Entrypoint: []string{
			"sh",
			"-c",
			fmt.Sprintf(installCNI, pluginPath, confListDirPath, pluginPath, ipamLinkName, pluginPath, netLinkName,
				confListDirPath, confListName, pluginPath, ipamLinkName, pluginPath, netLinkName, confListDirPath,
				confListName, confList),
		},
		Image: fmt.Sprintf("weaveworks/weaveexec:%s", cni.version),
	}, &container.HostConfig{
		Privileged:  true,
		NetworkMode: "host",
		AutoRemove:  true,
		PidMode:     "host",
		Mounts: []mount.Mount{
			{Type: mount.TypeBind, Source: "/var/run/docker.sock", Target: "/var/run/docker.sock"},
			{Type: mount.TypeBind, Source: "/opt", Target: "/opt"},
			{Type: mount.TypeBind, Source: "/etc", Target: "/etc"},
		},
	}, nil, nil, "weaveexec")
	if err != nil {
		return err
	}
	return cni.cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{})
}

func buildCNIPluginSymlink(version string) error {
	ipamLinkPath := filepath.Join(pluginPath, ipamLinkName)
	netLinkPath := filepath.Join(pluginPath, netLinkName)
	plugin := fmt.Sprintf("%s-%s", pluginName, version)
	if err := os.RemoveAll(ipamLinkPath); err != nil {
		return err
	}
	if err := os.Symlink(plugin, ipamLinkPath); err != nil {
		return err
	}
	if err := os.RemoveAll(netLinkPath); err != nil {
		return err
	}
	return os.Symlink(plugin, netLinkPath)
}

func writeCNIConf() error {
	if err := os.MkdirAll(confListDirPath, 0755); err != nil {
		return err
	}
	confListPath := filepath.Join(confListDirPath, confListName)
	if err := os.RemoveAll(confListPath); err != nil {
		return err
	}
	return ioutil.WriteFile(confListPath, []byte(confList), 0755)
}
