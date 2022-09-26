package go_weave_api

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWeave_AddDNS(t *testing.T) {
	w := &Weave{address: "192.168.0.111", httpPort: weaveHttpPort}
	dns := NewDNSServer("", "weave.local.", true)
	dns.weave = w
	err := dns.addWeaveDNS("", "180.101.49.11", "baidu3", true)
	require.NoError(t, err)
	err = dns.addWeaveDNS("", "180.101.49.11", "baidu2", true)
	require.NoError(t, err)
	err = dns.addWeaveDNS("", "180.101.49.11", "baidu", true)
	require.NoError(t, err)

	err = dns.addWeaveDNS("90440c9f28af", "10.32.0.1", "box4", false)
	require.NoError(t, err)
	err = dns.addWeaveDNS("90440c9f28af", "10.32.0.1", "box5", false)
	require.NoError(t, err)
	err = dns.addWeaveDNS("90440c9f28af", "10.32.0.1", "box6", false)
	require.NoError(t, err)
}

func TestWeave_RemoveDNS(t *testing.T) {
	// run TestWeave_AddDNS first
	w := &Weave{address: "192.168.0.111", httpPort: weaveHttpPort}
	dns := NewDNSServer("", "weave.local.", true)
	dns.weave = w

	status, err := w.Status("dns")
	require.NoError(t, err)
	require.Equal(t, 6, len(status.DNS))

	err = dns.removeWeaveDNS("90440c9f28af", "10.32.0.1", "box4", false)
	require.NoError(t, err)
	status, err = w.Status("dns")
	require.NoError(t, err)
	require.Equal(t, 5, len(status.DNS))

	err = dns.removeWeaveDNS("90440c9f28af", "10.32.0.1", "", false)
	require.NoError(t, err)
	status, err = w.Status("dns")
	require.NoError(t, err)
	require.Equal(t, 3, len(status.DNS))

	err = dns.removeWeaveDNS("", "180.101.49.11", "baidu", true)
	require.NoError(t, err)
	status, err = w.Status("dns")
	require.NoError(t, err)
	require.Equal(t, 2, len(status.DNS))
}
