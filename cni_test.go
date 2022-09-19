 package go_weave_api

import (
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGenerateCNIConf(t *testing.T) {
	if err := writeCNIConf(); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratePluginWithDocker(t *testing.T) {
	cli, err := docker.NewClientWithOpts(docker.WithHost("tcp://192.168.0.112:2375"))
	require.NoError(t, err)
	defer cli.Close()

	cni := &CNIBuilder{cli: cli, version: "2.8.1"}
	err = cni.generatePluginWithDocker()
	require.NoError(t, err)
}

func TestBuildCNIPluginSymlink(t *testing.T) {
	err := buildCNIPluginSymlink("2.8.1")
	require.NoError(t, err)
}

func TestWriteCNIConf(t *testing.T) {
	err := writeCNIConf()
	require.NoError(t, err)
}
