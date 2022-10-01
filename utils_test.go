package go_weave_api

import (
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestRemoveSliceElement(t *testing.T) {
	s := []string{"a", "b", "c", "d"}
	s = removeSliceElement(s, 0)
	require.Equal(t, 3, len(s))
	t.Log(s)

	s = removeSliceElement(s, 1)
	require.Equal(t, 2, len(s))

	s = removeSliceElement(s, 1)
	require.Equal(t, 1, len(s))
	require.Equal(t, "b", s[0])
	t.Log(s)
}

func TestGetContainerStateByName(t *testing.T) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	defer cli.Close()
	containerName := "weavedb"
	state, err := getContainerStateByName(cli, containerName)
	require.NoError(t, err)
	require.Equal(t, "created", state)
}

func TestGetContainerIdByName(t *testing.T) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv)
	require.NoError(t, err)
	defer cli.Close()

	containerName := "w1"
	containerId := "34cd9ec936b2"

	id, err := getContainerIdByName(cli, containerName)
	t.Log(id)
	require.NoError(t, err)
	require.NotEqual(t, containerName, id)

	id, err = getContainerIdByName(cli, containerName)
	t.Log(id)
	require.NoError(t, err)
	b := strings.HasPrefix(id, containerId)
	require.Equal(t, true, b)
}

func TestRandString(t *testing.T) {
	s := randString()
	t.Log(s)
}
