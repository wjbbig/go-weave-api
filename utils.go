package go_weave_api

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/docker/docker/api/types/container"
	docker "github.com/docker/docker/client"
	"github.com/pkg/errors"
	"math/rand"
	"net"
	"strings"
	"time"
)

func removeSliceElement[E any](s []E, index int) []E {
	if index == 0 {
		return s[1:]
	}
	if index == len(s)-1 {
		return s[:index]
	}
	s = append(s[:index], s[index+1:]...)
	return s
}

func getContainerStateByName(cli *docker.Client, containerName string) (string, error) {
	c, err := cli.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return "", errors.Errorf("unable to inspect container %s: %s", containerName, err)
	}
	return c.State.Status, nil
}

func getContainerWeaveIP(cli *docker.Client, containerName string) (string, error) {
	c, err := cli.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return "", err
	}
	for _, network := range c.NetworkSettings.Networks {
		if network.NetworkID == "weave" {
			return network.IPAddress, nil
		}
	}

	return "", errors.Errorf("can't find weave bridge of container %s", containerName)
}

func getContainerIdByName(cli *docker.Client, containerName string) (string, error) {
	c, err := cli.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func createVolumeContainer(cli *docker.Client, containerName, image, label string, bindMounts ...string) error {
	c, err := cli.ContainerInspect(context.Background(), containerName)
	if err != nil {
		if docker.IsErrNotFound(err) {
			volumes := make(map[string]struct{})
			for _, m := range bindMounts {
				volumes[m] = struct{}{}
			}
			labels := map[string]string{label: ""}

			config := &container.Config{Image: image, Volumes: volumes, Labels: labels, Entrypoint: []string{"data-only"}}
			_, err = cli.ContainerCreate(context.Background(), config, &container.HostConfig{},
				nil, nil, containerName)
			if err != nil {
				return fmt.Errorf("unable to create container: %s", err)
			}
		}
		return err
	}
	// already exist
	if strings.ToLower(c.State.Status) == "created" {
		return nil
	}
	return nil
}

func randString() string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func localhost(host string) bool {
	ip := net.ParseIP(host)
	ip.IsLoopback()
	return host == "127.0.0.1" || host == "localhost"
}
