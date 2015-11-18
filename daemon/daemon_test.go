package daemon

import (
	"os"
	"path"
	"testing"

	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/runconfig"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
	"github.com/docker/docker/volume/store"
)

//
// https://github.com/docker/docker/issues/8069
//

func TestGet(t *testing.T) {
	c1 := &Container{
		CommonContainer: CommonContainer{
			ID:   "5a4ff6a163ad4533d22d69a2b8960bf7fafdcba06e72d2febdba229008b0bf57",
			Name: "tender_bardeen",
		},
	}

	c2 := &Container{
		CommonContainer: CommonContainer{
			ID:   "3cdbd1aa394fd68559fd1441d6eff2ab7c1e6363582c82febfaa8045df3bd8de",
			Name: "drunk_hawking",
		},
	}

	c3 := &Container{
		CommonContainer: CommonContainer{
			ID:   "3cdbd1aa394fd68559fd1441d6eff2abfafdcba06e72d2febdba229008b0bf57",
			Name: "3cdbd1aa",
		},
	}

	c4 := &Container{
		CommonContainer: CommonContainer{
			ID:   "75fb0b800922abdbef2d27e60abcdfaf7fb0698b2a96d22d3354da361a6ff4a5",
			Name: "5a4ff6a163ad4533d22d69a2b8960bf7fafdcba06e72d2febdba229008b0bf57",
		},
	}

	c5 := &Container{
		CommonContainer: CommonContainer{
			ID:   "d22d69a2b8960bf7fafdcba06e72d2febdba960bf7fafdcba06e72d2f9008b060b",
			Name: "d22d69a2b896",
		},
	}

	store := &contStore{
		s: map[string]*Container{
			c1.ID: c1,
			c2.ID: c2,
			c3.ID: c3,
			c4.ID: c4,
			c5.ID: c5,
		},
	}

	index := truncindex.NewTruncIndex([]string{})
	index.Add(c1.ID)
	index.Add(c2.ID)
	index.Add(c3.ID)
	index.Add(c4.ID)
	index.Add(c5.ID)

	daemonTestDbPath := path.Join(os.TempDir(), "daemon_test.db")
	graph, err := graphdb.NewSqliteConn(daemonTestDbPath)
	if err != nil {
		t.Fatalf("Failed to create daemon test sqlite database at %s", daemonTestDbPath)
	}
	graph.Set(c1.Name, c1.ID)
	graph.Set(c2.Name, c2.ID)
	graph.Set(c3.Name, c3.ID)
	graph.Set(c4.Name, c4.ID)
	graph.Set(c5.Name, c5.ID)

	daemon := &Daemon{
		containers:       store,
		idIndex:          index,
		containerGraphDB: graph,
	}

	if container, _ := daemon.Get("3cdbd1aa394fd68559fd1441d6eff2ab7c1e6363582c82febfaa8045df3bd8de"); container != c2 {
		t.Fatal("Should explicitly match full container IDs")
	}

	if container, _ := daemon.Get("75fb0b8009"); container != c4 {
		t.Fatal("Should match a partial ID")
	}

	if container, _ := daemon.Get("drunk_hawking"); container != c2 {
		t.Fatal("Should match a full name")
	}

	// c3.Name is a partial match for both c3.ID and c2.ID
	if c, _ := daemon.Get("3cdbd1aa"); c != c3 {
		t.Fatal("Should match a full name even though it collides with another container's ID")
	}

	if container, _ := daemon.Get("d22d69a2b896"); container != c5 {
		t.Fatal("Should match a container where the provided prefix is an exact match to the it's name, and is also a prefix for it's ID")
	}

	if _, err := daemon.Get("3cdbd1"); err == nil {
		t.Fatal("Should return an error when provided a prefix that partially matches multiple container ID's")
	}

	if _, err := daemon.Get("nothing"); err == nil {
		t.Fatal("Should return an error when provided a prefix that is neither a name or a partial match to an ID")
	}

	os.Remove(daemonTestDbPath)
}

func initDaemonWithVolumeStore(tmp string) (*Daemon, error) {
	daemon := &Daemon{
		repository: tmp,
		root:       tmp,
		volumes:    store.New(),
	}

	volumesDriver, err := local.New(tmp, 0, 0)
	if err != nil {
		return nil, err
	}
	volumedrivers.Register(volumesDriver, volumesDriver.Name())

	return daemon, nil
}

func TestParseSecurityOpt(t *testing.T) {
	container := &Container{}
	config := &runconfig.HostConfig{}

	// test apparmor
	config.SecurityOpt = []string{"apparmor:test_profile"}
	if err := parseSecurityOpt(container, config); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}
	if container.AppArmorProfile != "test_profile" {
		t.Fatalf("Unexpected AppArmorProfile, expected: \"test_profile\", got %q", container.AppArmorProfile)
	}

	// test valid label
	config.SecurityOpt = []string{"label:user:USER"}
	if err := parseSecurityOpt(container, config); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}

	// test invalid label
	config.SecurityOpt = []string{"label"}
	if err := parseSecurityOpt(container, config); err == nil {
		t.Fatal("Expected parseSecurityOpt error, got nil")
	}

	// test invalid opt
	config.SecurityOpt = []string{"test"}
	if err := parseSecurityOpt(container, config); err == nil {
		t.Fatal("Expected parseSecurityOpt error, got nil")
	}
}

func TestNetworkOptions(t *testing.T) {
	daemon := &Daemon{}
	dconfigCorrect := &Config{
		CommonConfig: CommonConfig{
			ClusterStore:     "consul://localhost:8500",
			ClusterAdvertise: "192.168.0.1:8000",
		},
	}

	if _, err := daemon.networkOptions(dconfigCorrect); err != nil {
		t.Fatalf("Expect networkOptions sucess, got error: %v", err)
	}

	dconfigWrong := &Config{
		CommonConfig: CommonConfig{
			ClusterStore: "consul://localhost:8500://test://bbb",
		},
	}

	if _, err := daemon.networkOptions(dconfigWrong); err == nil {
		t.Fatalf("Expected networkOptions error, got nil")
	}
}
