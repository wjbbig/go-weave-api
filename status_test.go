package go_weave_api

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWeave_Status(t *testing.T) {
	w := &Weave{address: "127.0.0.1", httpPort: 6784}

	// dns
	status, err := w.Status("dns")
	require.NoError(t, err)
	t.Log(status.DNS)

	//overview
	status, err = w.Status()
	require.NoError(t, err)
	t.Log(status.Overview)

	status, err = w.Status("connections")
	require.NoError(t, err)
	t.Log(status.Connections)

	status, err = w.Status("targets")
	require.NoError(t, err)
	t.Log(status.Targets)

	status, err = w.Status("peers")
	require.NoError(t, err)
	t.Log(status.Peers)
}
