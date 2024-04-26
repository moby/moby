package remote

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/pkg/plugins"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func decodeToMap(r *http.Request) (res map[string]interface{}, err error) {
	err = json.NewDecoder(r.Body).Decode(&res)
	return
}

func handle(t *testing.T, mux *http.ServeMux, method string, h func(map[string]interface{}) interface{}) {
	mux.HandleFunc(fmt.Sprintf("/%s.%s", ipamapi.PluginEndpointType, method), func(w http.ResponseWriter, r *http.Request) {
		ask, err := decodeToMap(r)
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
		answer := h(ask)
		err = json.NewEncoder(w).Encode(&answer)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func setupPlugin(t *testing.T, name string, mux *http.ServeMux) func() {
	specPath := "/etc/docker/plugins"
	if runtime.GOOS == "windows" {
		specPath = filepath.Join(os.Getenv("programdata"), "docker", "plugins")
	}

	if err := os.MkdirAll(specPath, 0o755); err != nil {
		t.Fatal(err)
	}

	defer func() {
		if t.Failed() {
			os.RemoveAll(specPath)
		}
	}()

	server := httptest.NewServer(mux)
	if server == nil {
		t.Fatal("Failed to start an HTTP Server")
	}

	if err := os.WriteFile(filepath.Join(specPath, name+".spec"), []byte(server.URL), 0o644); err != nil {
		t.Fatal(err)
	}

	mux.HandleFunc("/Plugin.Activate", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", plugins.VersionMimetype)
		fmt.Fprintf(w, `{"Implements": ["%s"]}`, ipamapi.PluginEndpointType)
	})

	return func() {
		if err := os.RemoveAll(specPath); err != nil {
			t.Fatal(err)
		}
		server.Close()
	}
}

func TestGetCapabilities(t *testing.T) {
	plugin := "test-ipam-driver-capabilities"

	mux := http.NewServeMux()
	defer setupPlugin(t, plugin, mux)()

	handle(t, mux, "GetCapabilities", func(msg map[string]interface{}) interface{} {
		return map[string]interface{}{
			"RequiresMACAddress": true,
		}
	})

	p, err := plugins.Get(plugin, ipamapi.PluginEndpointType)
	if err != nil {
		t.Fatal(err)
	}

	client, err := getPluginClient(p)
	if err != nil {
		t.Fatal(err)
	}
	d := newAllocator(plugin, client)

	caps, err := d.(*allocator).getCapabilities()
	if err != nil {
		t.Fatal(err)
	}

	if !caps.RequiresMACAddress || caps.RequiresRequestReplay {
		t.Fatalf("Unexpected capability: %v", caps)
	}
}

func TestGetCapabilitiesFromLegacyDriver(t *testing.T) {
	plugin := "test-ipam-legacy-driver"

	mux := http.NewServeMux()
	defer setupPlugin(t, plugin, mux)()

	p, err := plugins.Get(plugin, ipamapi.PluginEndpointType)
	if err != nil {
		t.Fatal(err)
	}

	client, err := getPluginClient(p)
	if err != nil {
		t.Fatal(err)
	}

	d := newAllocator(plugin, client)

	if _, err := d.(*allocator).getCapabilities(); err == nil {
		t.Fatalf("Expected error, but got Success %v", err)
	}
}

func TestGetDefaultAddressSpaces(t *testing.T) {
	plugin := "test-ipam-driver-addr-spaces"

	mux := http.NewServeMux()
	defer setupPlugin(t, plugin, mux)()

	handle(t, mux, "GetDefaultAddressSpaces", func(msg map[string]interface{}) interface{} {
		return map[string]interface{}{
			"LocalDefaultAddressSpace":  "white",
			"GlobalDefaultAddressSpace": "blue",
		}
	})

	p, err := plugins.Get(plugin, ipamapi.PluginEndpointType)
	if err != nil {
		t.Fatal(err)
	}

	client, err := getPluginClient(p)
	if err != nil {
		t.Fatal(err)
	}
	d := newAllocator(plugin, client)

	l, g, err := d.(*allocator).GetDefaultAddressSpaces()
	if err != nil {
		t.Fatal(err)
	}

	if l != "white" || g != "blue" {
		t.Fatalf("Unexpected default local and global address spaces: %s, %s", l, g)
	}
}

func TestRemoteDriver(t *testing.T) {
	plugin := "test-ipam-driver"

	mux := http.NewServeMux()
	defer setupPlugin(t, plugin, mux)()

	handle(t, mux, "GetDefaultAddressSpaces", func(msg map[string]interface{}) interface{} {
		return map[string]interface{}{
			"LocalDefaultAddressSpace":  "white",
			"GlobalDefaultAddressSpace": "blue",
		}
	})

	handle(t, mux, "RequestPool", func(msg map[string]interface{}) interface{} {
		as := "white"
		if v, ok := msg["AddressSpace"]; ok && v.(string) != "" {
			as = v.(string)
		}

		pl := "172.18.0.0/16"
		sp := ""
		if v, ok := msg["Pool"]; ok && v.(string) != "" {
			pl = v.(string)
		}
		if v, ok := msg["SubPool"]; ok && v.(string) != "" {
			sp = v.(string)
		}
		pid := fmt.Sprintf("%s/%s", as, pl)
		if sp != "" {
			pid = fmt.Sprintf("%s/%s", pid, sp)
		}
		return map[string]interface{}{
			"PoolID": pid,
			"Pool":   pl,
			"Data":   map[string]string{"DNS": "8.8.8.8"},
		}
	})

	handle(t, mux, "ReleasePool", func(msg map[string]interface{}) interface{} {
		if _, ok := msg["PoolID"]; !ok {
			t.Fatal("Missing PoolID in Release request")
		}
		return map[string]interface{}{}
	})

	handle(t, mux, "RequestAddress", func(msg map[string]interface{}) interface{} {
		if _, ok := msg["PoolID"]; !ok {
			t.Fatal("Missing PoolID in address request")
		}
		prefAddr := ""
		if v, ok := msg["Address"]; ok {
			prefAddr = v.(string)
		}
		ip := prefAddr
		if ip == "" {
			ip = "172.20.0.34"
		}
		ip = fmt.Sprintf("%s/16", ip)
		return map[string]interface{}{
			"Address": ip,
		}
	})

	handle(t, mux, "ReleaseAddress", func(msg map[string]interface{}) interface{} {
		if _, ok := msg["PoolID"]; !ok {
			t.Fatal("Missing PoolID in address request")
		}
		if _, ok := msg["Address"]; !ok {
			t.Fatal("Missing Address in release address request")
		}
		return map[string]interface{}{}
	})

	p, err := plugins.Get(plugin, ipamapi.PluginEndpointType)
	if err != nil {
		t.Fatal(err)
	}

	client, err := getPluginClient(p)
	if err != nil {
		t.Fatal(err)
	}
	d := newAllocator(plugin, client)

	l, g, err := d.(*allocator).GetDefaultAddressSpaces()
	assert.NilError(t, err)
	assert.Check(t, is.Equal(l, "white"))
	assert.Check(t, is.Equal(g, "blue"))

	// Request any pool
	alloc, err := d.RequestPool(ipamapi.PoolRequest{
		AddressSpace: "white",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, "white/172.18.0.0/16"))
	assert.Check(t, is.Equal(alloc.Pool.String(), "172.18.0.0/16"))

	// Request specific pool
	alloc, err = d.RequestPool(ipamapi.PoolRequest{
		AddressSpace: "white",
		Pool:         "172.20.0.0/16",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, "white/172.20.0.0/16"))
	assert.Check(t, is.Equal(alloc.Pool.String(), "172.20.0.0/16"))
	assert.Check(t, is.Equal(alloc.Meta["DNS"], "8.8.8.8"))

	// Request specific pool and subpool
	alloc, err = d.RequestPool(ipamapi.PoolRequest{
		AddressSpace: "white",
		Pool:         "172.20.0.0/16",
		SubPool:      "172.20.3.0/24",
		Options:      map[string]string{"culo": "yes"},
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(alloc.PoolID, "white/172.20.0.0/16/172.20.3.0/24"))
	assert.Check(t, is.Equal(alloc.Pool.String(), "172.20.0.0/16"))

	// Request any address
	addr, _, err := d.RequestAddress("white/172.20.0.0/16", nil, nil)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(addr.String(), "172.20.0.34/16"))

	// Request specific address
	addr2, _, err := d.RequestAddress("white/172.20.0.0/16", net.ParseIP("172.20.1.45"), nil)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(addr2.String(), "172.20.1.45/16"))

	// Release address
	err = d.ReleaseAddress("white/172.20.0.0/16", net.ParseIP("172.18.1.45"))
	if err != nil {
		t.Fatal(err)
	}
}
