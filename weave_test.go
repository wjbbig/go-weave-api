package go_weave_api

import (
	"bytes"
	"context"
	"fmt"
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)

func TestRunWeaveExec(t *testing.T) {
	cli, err := docker.NewClientWithOpts(docker.WithHost("tcp://192.168.0.112:2375"))
	require.NoError(t, err)
	defer cli.Close()

	w := &Weave{dockerCli: cli, version: defaultWeaveVersion}
	result, err := w.runWeaveExec("check-datapath", "datapath")
	require.NoError(t, err)
	t.Log(result)
	//require.NoError(t, err)
	//t.Log(string(result))
	//split := bytes.Split(result, []byte(" "))
	//t.Log(len(split))
	//t.Log(string(split[0]))
	//err = w.checkOverlap("10.32.0.0/12", "weave")
	//require.NoError(t, err)
}

func TestRemoteDockerHostLoadLocalImage(t *testing.T) {
	cli, err := docker.NewClientWithOpts(docker.WithHost("tcp://192.168.0.106:2375"))
	require.NoError(t, err)
	defer cli.Close()
	data, err := ioutil.ReadFile("./redis.tar")
	require.NoError(t, err)

	resp, err := cli.ImageLoad(context.Background(), bytes.NewReader(data), false)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err = ioutil.ReadAll(resp.Body)
	require.NoError(t, err)

	t.Log(string(data))
}

func TestCreateNewWeave(t *testing.T) {
	hostname, err := os.Hostname()
	require.NoError(t, err)

	node, err := NewWeaveNode("192.168.0.112", WithDNSAddress("172.17.0.1:53"),
		WithProxy(), WithPlugin(), WithNickname(hostname), WithDockerPort(2375))
	require.NoError(t, err)

	err = node.Launch()
	require.NoError(t, err)
}

func TestCreateVolumeFrom(t *testing.T) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	defer cli.Close()
	err = createVolumeContainer(cli, "weavedb",
		fmt.Sprintf("weaveworks/weavedb:%s", "latest"), "weavevolumes",
		"/weavedb")
	require.NoError(t, err)
}

func TestWeave_ConnectAndForget(t *testing.T) {
	w := &Weave{address: "192.168.0.111", httpPort: 6784}

	err := w.Connect(false, "192.168.0.112")
	require.NoError(t, err)
	err = w.Forget("192.168.0.112")
	require.NoError(t, err)
	err = w.Connect(true, "192.168.0.113")
	require.NoError(t, err)

}

func TestWeave_LookupDNS(t *testing.T) {
	w, err := NewWeaveNode("127.0.0.1", WithDNSAddress("10.17.0.1:53"))
	require.NoError(t, err)
	defer w.Close()

	result, err := w.LookupDNS("whoami")
	require.NoError(t, err)

	t.Log(result)
}

func TestDNSResolvConfPath(t *testing.T) {
	w, err := NewWeaveNode("127.0.0.1")
	require.NoError(t, err)
	defer w.Close()

	result, err := w.runRemoteCmdWithContainer("readlink", "-f", "/host/etc/resolv.conf")
	require.NoError(t, err)
	i := 0
	for ; i < len(result); i++ {
		if string(result[i]) == "/" {
			break
		}
	}
	t.Log(string(result[i:]))
}
