package go_weave_api

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWeave_Status(t *testing.T) {
	w := &Weave{address: "192.168.0.111", httpPort: 6784}
	status, err := w.Status()
	require.NoError(t, err)
	t.Log(status)

	// dns
	status, err = w.Status("dns")
	require.NoError(t, err)
	t.Log(status.DNS)

	// collections
}
