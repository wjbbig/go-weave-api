package go_weave_api

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func TestA(t *testing.T) {
	r, err := callWeave(http.MethodGet, "http://192.168.0.101:6784/ipinfo/tracker", nil)
	require.NoError(t, err)
	t.Log(string(r))
}

func TestIsCIDRs(t *testing.T) {
	cidr := isCIDR("net:10.123.11.0/24")
	t.Log(cidr)
}

func TestWeave_Attach(t *testing.T) {
	w, err := NewWeaveNode("127.0.0.1")
	require.NoError(t, err)
	defer w.Close()

	err = w.Attach("box", false, false, false, nil, "net:10.44.0.0/24")
	require.NoError(t, err)

	err = w.Detach("box", "net:default")
	require.NoError(t, err)
}

func TestWeave_Expose(t *testing.T) {
	w, _ := NewWeaveNode("127.0.0.1")
	defer w.Close()
	exposes, err := w.Expose("", false, "net:default", "net:10.44.0.0/24")
	require.NoError(t, err)
	require.Equal(t, 2, len(exposes))
	t.Log(exposes)
}

func TestWeave_Hide(t *testing.T) {
	w, _ := NewWeaveNode("127.0.0.1")
	defer w.Close()
	_, err := w.Hide("net:10.44.0.0/24")
	require.NoError(t, err)
}
