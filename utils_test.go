package go_weave_api

import (
	docker "github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
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

func TestRandString(t *testing.T) {
	s := RandString()
	t.Log(s)
}
