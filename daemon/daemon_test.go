package daemon

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/container"
	"github.com/docker/docker/pkg/graphdb"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/docker/docker/volume"
	volumedrivers "github.com/docker/docker/volume/drivers"
	"github.com/docker/docker/volume/local"
	"github.com/docker/docker/volume/store"
)

//
// https://github.com/docker/docker/issues/8069
//

func TestGetContainer(t *testing.T) {
	c1 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "5a4ff6a163ad4533d22d69a2b8960bf7fafdcba06e72d2febdba229008b0bf57",
			Name: "tender_bardeen",
		},
	}

	c2 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "3cdbd1aa394fd68559fd1441d6eff2ab7c1e6363582c82febfaa8045df3bd8de",
			Name: "drunk_hawking",
		},
	}

	c3 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "3cdbd1aa394fd68559fd1441d6eff2abfafdcba06e72d2febdba229008b0bf57",
			Name: "3cdbd1aa",
		},
	}

	c4 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "75fb0b800922abdbef2d27e60abcdfaf7fb0698b2a96d22d3354da361a6ff4a5",
			Name: "5a4ff6a163ad4533d22d69a2b8960bf7fafdcba06e72d2febdba229008b0bf57",
		},
	}

	c5 := &container.Container{
		CommonContainer: container.CommonContainer{
			ID:   "d22d69a2b8960bf7fafdcba06e72d2febdba960bf7fafdcba06e72d2f9008b060b",
			Name: "d22d69a2b896",
		},
	}

	store := &contStore{
		s: map[string]*container.Container{
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

	if container, _ := daemon.GetContainer("3cdbd1aa394fd68559fd1441d6eff2ab7c1e6363582c82febfaa8045df3bd8de"); container != c2 {
		t.Fatal("Should explicitly match full container IDs")
	}

	if container, _ := daemon.GetContainer("75fb0b8009"); container != c4 {
		t.Fatal("Should match a partial ID")
	}

	if container, _ := daemon.GetContainer("drunk_hawking"); container != c2 {
		t.Fatal("Should match a full name")
	}

	// c3.Name is a partial match for both c3.ID and c2.ID
	if c, _ := daemon.GetContainer("3cdbd1aa"); c != c3 {
		t.Fatal("Should match a full name even though it collides with another container's ID")
	}

	if container, _ := daemon.GetContainer("d22d69a2b896"); container != c5 {
		t.Fatal("Should match a container where the provided prefix is an exact match to the it's name, and is also a prefix for it's ID")
	}

	if _, err := daemon.GetContainer("3cdbd1"); err == nil {
		t.Fatal("Should return an error when provided a prefix that partially matches multiple container ID's")
	}

	if _, err := daemon.GetContainer("nothing"); err == nil {
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
	container := &container.Container{}
	config := &containertypes.HostConfig{}

	// test apparmor
	config.SecurityOpt = []string{"apparmor:test_profile"}
	if err := parseSecurityOpt(container, config); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}
	if container.AppArmorProfile != "test_profile" {
		t.Fatalf("Unexpected AppArmorProfile, expected: \"test_profile\", got %q", container.AppArmorProfile)
	}

	// test seccomp
	sp := "/path/to/seccomp_test.json"
	config.SecurityOpt = []string{"seccomp:" + sp}
	if err := parseSecurityOpt(container, config); err != nil {
		t.Fatalf("Unexpected parseSecurityOpt error: %v", err)
	}
	if container.SeccompProfile != sp {
		t.Fatalf("Unexpected AppArmorProfile, expected: %q, got %q", sp, container.SeccompProfile)
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

func TestGetFullName(t *testing.T) {
	name, err := GetFullContainerName("testing")
	if err != nil {
		t.Fatal(err)
	}
	if name != "/testing" {
		t.Fatalf("Expected /testing got %s", name)
	}
	if _, err := GetFullContainerName(""); err == nil {
		t.Fatal("Error should not be nil")
	}
}

func TestValidContainerNames(t *testing.T) {
	invalidNames := []string{"-rm", "&sdfsfd", "safd%sd"}
	validNames := []string{"word-word", "word_word", "1weoid"}

	for _, name := range invalidNames {
		if validContainerNamePattern.MatchString(name) {
			t.Fatalf("%q is not a valid container name and was returned as valid.", name)
		}
	}

	for _, name := range validNames {
		if !validContainerNamePattern.MatchString(name) {
			t.Fatalf("%q is a valid container name and was returned as invalid.", name)
		}
	}
}

func TestContainerInitDNS(t *testing.T) {
	tmp, err := ioutil.TempDir("", "docker-container-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	containerID := "d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e"
	containerPath := filepath.Join(tmp, containerID)
	if err := os.MkdirAll(containerPath, 0755); err != nil {
		t.Fatal(err)
	}

	config := `{"State":{"Running":true,"Paused":false,"Restarting":false,"OOMKilled":false,"Dead":false,"Pid":2464,"ExitCode":0,
"Error":"","StartedAt":"2015-05-26T16:48:53.869308965Z","FinishedAt":"0001-01-01T00:00:00Z"},
"ID":"d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e","Created":"2015-05-26T16:48:53.7987917Z","Path":"top",
"Args":[],"Config":{"Hostname":"d59df5276e7b","Domainname":"","User":"","Memory":0,"MemorySwap":0,"CpuShares":0,"Cpuset":"",
"AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"PortSpecs":null,"ExposedPorts":null,"Tty":true,"OpenStdin":true,
"StdinOnce":false,"Env":null,"Cmd":["top"],"Image":"ubuntu:latest","Volumes":null,"WorkingDir":"","Entrypoint":null,
"NetworkDisabled":false,"MacAddress":"","OnBuild":null,"Labels":{}},"Image":"07f8e8c5e66084bef8f848877857537ffe1c47edd01a93af27e7161672ad0e95",
"NetworkSettings":{"IPAddress":"172.17.0.1","IPPrefixLen":16,"MacAddress":"02:42:ac:11:00:01","LinkLocalIPv6Address":"fe80::42:acff:fe11:1",
"LinkLocalIPv6PrefixLen":64,"GlobalIPv6Address":"","GlobalIPv6PrefixLen":0,"Gateway":"172.17.42.1","IPv6Gateway":"","Bridge":"docker0","Ports":{}},
"ResolvConfPath":"/var/lib/docker/containers/d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e/resolv.conf",
"HostnamePath":"/var/lib/docker/containers/d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e/hostname",
"HostsPath":"/var/lib/docker/containers/d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e/hosts",
"LogPath":"/var/lib/docker/containers/d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e/d59df5276e7b219d510fe70565e0404bc06350e0d4b43fe961f22f339980170e-json.log",
"Name":"/ubuntu","Driver":"aufs","MountLabel":"","ProcessLabel":"","AppArmorProfile":"","RestartCount":0,
"UpdateDns":false,"Volumes":{},"VolumesRW":{},"AppliedVolumesFrom":null}`

	// Container struct only used to retrieve path to config file
	container := &container.Container{CommonContainer: container.CommonContainer{Root: containerPath}}
	configPath, err := container.ConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	hostConfig := `{"Binds":[],"ContainerIDFile":"","Memory":0,"MemorySwap":0,"CpuShares":0,"CpusetCpus":"",
"Privileged":false,"PortBindings":{},"Links":null,"PublishAllPorts":false,"Dns":null,"DnsOptions":null,"DnsSearch":null,"ExtraHosts":null,"VolumesFrom":null,
"Devices":[],"NetworkMode":"bridge","IpcMode":"","PidMode":"","CapAdd":null,"CapDrop":null,"RestartPolicy":{"Name":"no","MaximumRetryCount":0},
"SecurityOpt":null,"ReadonlyRootfs":false,"Ulimits":null,"LogConfig":{"Type":"","Config":null},"CgroupParent":""}`

	hostConfigPath, err := container.HostConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if err = ioutil.WriteFile(hostConfigPath, []byte(hostConfig), 0644); err != nil {
		t.Fatal(err)
	}

	daemon, err := initDaemonWithVolumeStore(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer volumedrivers.Unregister(volume.DefaultDriverName)

	c, err := daemon.load(containerID)
	if err != nil {
		t.Fatal(err)
	}

	if c.HostConfig.DNS == nil {
		t.Fatal("Expected container DNS to not be nil")
	}

	if c.HostConfig.DNSSearch == nil {
		t.Fatal("Expected container DNSSearch to not be nil")
	}

	if c.HostConfig.DNSOptions == nil {
		t.Fatal("Expected container DNSOptions to not be nil")
	}
}
