package service

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/pkg/testutil/assert"
	"golang.org/x/net/context"
)

func TestUpdateServiceArgs(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("args", "the \"new args\"")

	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec
	cspec.Args = []string{"old", "args"}

	updateService(flags, spec)
	assert.EqualStringSlice(t, cspec.Args, []string{"the", "new args"})
}

func TestUpdateLabels(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-add", "toadd=newlabel")
	flags.Set("label-rm", "toremove")

	labels := map[string]string{
		"toremove": "thelabeltoremove",
		"tokeep":   "value",
	}

	updateLabels(flags, &labels)
	assert.Equal(t, len(labels), 2)
	assert.Equal(t, labels["tokeep"], "value")
	assert.Equal(t, labels["toadd"], "newlabel")
}

func TestUpdateLabelsRemoveALabelThatDoesNotExist(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("label-rm", "dne")

	labels := map[string]string{"foo": "theoldlabel"}
	updateLabels(flags, &labels)
	assert.Equal(t, len(labels), 1)
}

func TestUpdatePlacement(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("constraint-add", "node=toadd")
	flags.Set("constraint-rm", "node!=toremove")

	placement := &swarm.Placement{
		Constraints: []string{"node!=toremove", "container=tokeep"},
	}

	updatePlacement(flags, placement)
	assert.Equal(t, len(placement.Constraints), 2)
	assert.Equal(t, placement.Constraints[0], "container=tokeep")
	assert.Equal(t, placement.Constraints[1], "node=toadd")
}

func TestUpdateEnvironment(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "toadd=newenv")
	flags.Set("env-rm", "toremove")

	envs := []string{"toremove=theenvtoremove", "tokeep=value"}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 2)
	// Order has been removed in updateEnvironment (map)
	sort.Strings(envs)
	assert.Equal(t, envs[0], "toadd=newenv")
	assert.Equal(t, envs[1], "tokeep=value")
}

func TestUpdateEnvironmentWithDuplicateValues(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "foo=newenv")
	flags.Set("env-add", "foo=dupe")
	flags.Set("env-rm", "foo")

	envs := []string{"foo=value"}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 0)
}

func TestUpdateEnvironmentWithDuplicateKeys(t *testing.T) {
	// Test case for #25404
	flags := newUpdateCommand(nil).Flags()
	flags.Set("env-add", "A=b")

	envs := []string{"A=c"}

	updateEnvironment(flags, &envs)
	assert.Equal(t, len(envs), 1)
	assert.Equal(t, envs[0], "A=b")
}

func TestUpdateGroups(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("group-add", "wheel")
	flags.Set("group-add", "docker")
	flags.Set("group-rm", "root")
	flags.Set("group-add", "foo")
	flags.Set("group-rm", "docker")

	groups := []string{"bar", "root"}

	updateGroups(flags, &groups)
	assert.Equal(t, len(groups), 3)
	assert.Equal(t, groups[0], "bar")
	assert.Equal(t, groups[1], "foo")
	assert.Equal(t, groups[2], "wheel")
}

func TestUpdateDNSConfig(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()

	// IPv4, with duplicates
	flags.Set("dns-add", "1.1.1.1")
	flags.Set("dns-add", "1.1.1.1")
	flags.Set("dns-add", "2.2.2.2")
	flags.Set("dns-rm", "3.3.3.3")
	flags.Set("dns-rm", "2.2.2.2")
	// IPv6
	flags.Set("dns-add", "2001:db8:abc8::1")
	// Invalid dns record
	assert.Error(t, flags.Set("dns-add", "x.y.z.w"), "x.y.z.w is not an ip address")

	// domains with duplicates
	flags.Set("dns-search-add", "example.com")
	flags.Set("dns-search-add", "example.com")
	flags.Set("dns-search-add", "example.org")
	flags.Set("dns-search-rm", "example.org")
	// Invalid dns search domain
	assert.Error(t, flags.Set("dns-search-add", "example$com"), "example$com is not a valid domain")

	flags.Set("dns-option-add", "ndots:9")
	flags.Set("dns-option-rm", "timeout:3")

	config := &swarm.DNSConfig{
		Nameservers: []string{"3.3.3.3", "5.5.5.5"},
		Search:      []string{"localdomain"},
		Options:     []string{"timeout:3"},
	}

	updateDNSConfig(flags, &config)

	assert.Equal(t, len(config.Nameservers), 3)
	assert.Equal(t, config.Nameservers[0], "1.1.1.1")
	assert.Equal(t, config.Nameservers[1], "2001:db8:abc8::1")
	assert.Equal(t, config.Nameservers[2], "5.5.5.5")

	assert.Equal(t, len(config.Search), 2)
	assert.Equal(t, config.Search[0], "example.com")
	assert.Equal(t, config.Search[1], "localdomain")

	assert.Equal(t, len(config.Options), 1)
	assert.Equal(t, config.Options[0], "ndots:9")
}

func TestUpdateMounts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("mount-add", "type=volume,source=vol2,target=/toadd")
	flags.Set("mount-rm", "/toremove")

	mounts := []mounttypes.Mount{
		{Target: "/toremove", Source: "vol1", Type: mounttypes.TypeBind},
		{Target: "/tokeep", Source: "vol3", Type: mounttypes.TypeBind},
	}

	updateMounts(flags, &mounts)
	assert.Equal(t, len(mounts), 2)
	assert.Equal(t, mounts[0].Target, "/toadd")
	assert.Equal(t, mounts[1].Target, "/tokeep")

}

func TestUpdateMountsWithDuplicateMounts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("mount-add", "type=volume,source=vol4,target=/toadd")

	mounts := []mounttypes.Mount{
		{Target: "/tokeep1", Source: "vol1", Type: mounttypes.TypeBind},
		{Target: "/toadd", Source: "vol2", Type: mounttypes.TypeBind},
		{Target: "/tokeep2", Source: "vol3", Type: mounttypes.TypeBind},
	}

	updateMounts(flags, &mounts)
	assert.Equal(t, len(mounts), 3)
	assert.Equal(t, mounts[0].Target, "/tokeep1")
	assert.Equal(t, mounts[1].Target, "/tokeep2")
	assert.Equal(t, mounts[2].Target, "/toadd")
}

func TestUpdatePorts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "1000:1000")
	flags.Set("publish-rm", "333/udp")

	portConfigs := []swarm.PortConfig{
		{TargetPort: 333, Protocol: swarm.PortConfigProtocolUDP},
		{TargetPort: 555},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(t, err, nil)
	assert.Equal(t, len(portConfigs), 2)
	// Do a sort to have the order (might have changed by map)
	targetPorts := []int{int(portConfigs[0].TargetPort), int(portConfigs[1].TargetPort)}
	sort.Ints(targetPorts)
	assert.Equal(t, targetPorts[0], 555)
	assert.Equal(t, targetPorts[1], 1000)
}

func TestUpdatePortsDuplicate(t *testing.T) {
	// Test case for #25375
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "80:80")

	portConfigs := []swarm.PortConfig{
		{
			TargetPort:    80,
			PublishedPort: 80,
			Protocol:      swarm.PortConfigProtocolTCP,
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(t, err, nil)
	assert.Equal(t, len(portConfigs), 1)
	assert.Equal(t, portConfigs[0].TargetPort, uint32(80))
}

func TestUpdateHealthcheckTable(t *testing.T) {
	type test struct {
		flags    [][2]string
		initial  *container.HealthConfig
		expected *container.HealthConfig
		err      string
	}
	testCases := []test{
		{
			flags:    [][2]string{{"no-healthcheck", "true"}},
			initial:  &container.HealthConfig{Test: []string{"CMD-SHELL", "cmd1"}, Retries: 10},
			expected: &container.HealthConfig{Test: []string{"NONE"}},
		},
		{
			flags:    [][2]string{{"health-cmd", "cmd1"}},
			initial:  &container.HealthConfig{Test: []string{"NONE"}},
			expected: &container.HealthConfig{Test: []string{"CMD-SHELL", "cmd1"}},
		},
		{
			flags:    [][2]string{{"health-retries", "10"}},
			initial:  &container.HealthConfig{Test: []string{"NONE"}},
			expected: &container.HealthConfig{Retries: 10},
		},
		{
			flags:    [][2]string{{"health-retries", "10"}},
			initial:  &container.HealthConfig{Test: []string{"CMD", "cmd1"}},
			expected: &container.HealthConfig{Test: []string{"CMD", "cmd1"}, Retries: 10},
		},
		{
			flags:    [][2]string{{"health-interval", "1m"}},
			initial:  &container.HealthConfig{Test: []string{"CMD", "cmd1"}},
			expected: &container.HealthConfig{Test: []string{"CMD", "cmd1"}, Interval: time.Minute},
		},
		{
			flags:    [][2]string{{"health-cmd", ""}},
			initial:  &container.HealthConfig{Test: []string{"CMD", "cmd1"}, Retries: 10},
			expected: &container.HealthConfig{Retries: 10},
		},
		{
			flags:    [][2]string{{"health-retries", "0"}},
			initial:  &container.HealthConfig{Test: []string{"CMD", "cmd1"}, Retries: 10},
			expected: &container.HealthConfig{Test: []string{"CMD", "cmd1"}},
		},
		{
			flags: [][2]string{{"health-cmd", "cmd1"}, {"no-healthcheck", "true"}},
			err:   "--no-healthcheck conflicts with --health-* options",
		},
		{
			flags: [][2]string{{"health-interval", "10m"}, {"no-healthcheck", "true"}},
			err:   "--no-healthcheck conflicts with --health-* options",
		},
		{
			flags: [][2]string{{"health-timeout", "1m"}, {"no-healthcheck", "true"}},
			err:   "--no-healthcheck conflicts with --health-* options",
		},
	}
	for i, c := range testCases {
		flags := newUpdateCommand(nil).Flags()
		for _, flag := range c.flags {
			flags.Set(flag[0], flag[1])
		}
		cspec := &swarm.ContainerSpec{
			Healthcheck: c.initial,
		}
		err := updateHealthcheck(flags, cspec)
		if c.err != "" {
			assert.Error(t, err, c.err)
		} else {
			assert.NilError(t, err)
			if !reflect.DeepEqual(cspec.Healthcheck, c.expected) {
				t.Errorf("incorrect result for test %d, expected health config:\n\t%#v\ngot:\n\t%#v", i, c.expected, cspec.Healthcheck)
			}
		}
	}
}

func TestUpdateHosts(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("host-add", "example.net:2.2.2.2")
	flags.Set("host-add", "ipv6.net:2001:db8:abc8::1")
	// remove with ipv6 should work
	flags.Set("host-rm", "example.net:2001:db8:abc8::1")
	// just hostname should work as well
	flags.Set("host-rm", "example.net")
	// bad format error
	assert.Error(t, flags.Set("host-add", "$example.com$"), "bad format for add-host:")

	hosts := []string{"1.2.3.4 example.com", "4.3.2.1 example.org", "2001:db8:abc8::1 example.net"}

	updateHosts(flags, &hosts)
	assert.Equal(t, len(hosts), 3)
	assert.Equal(t, hosts[0], "1.2.3.4 example.com")
	assert.Equal(t, hosts[1], "2001:db8:abc8::1 ipv6.net")
	assert.Equal(t, hosts[2], "4.3.2.1 example.org")
}

func TestUpdatePortsRmWithProtocol(t *testing.T) {
	flags := newUpdateCommand(nil).Flags()
	flags.Set("publish-add", "8081:81")
	flags.Set("publish-add", "8082:82")
	flags.Set("publish-rm", "80")
	flags.Set("publish-rm", "81/tcp")
	flags.Set("publish-rm", "82/udp")

	portConfigs := []swarm.PortConfig{
		{
			TargetPort:    80,
			PublishedPort: 8080,
			Protocol:      swarm.PortConfigProtocolTCP,
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
	}

	err := updatePorts(flags, &portConfigs)
	assert.Equal(t, err, nil)
	assert.Equal(t, len(portConfigs), 2)
	assert.Equal(t, portConfigs[0].TargetPort, uint32(81))
	assert.Equal(t, portConfigs[1].TargetPort, uint32(82))
}

// FIXME(vdemeester) port to opts.PortOpt
func TestValidatePort(t *testing.T) {
	validPorts := []string{"80/tcp", "80", "80/udp"}
	invalidPorts := map[string]string{
		"9999999":   "out of range",
		"80:80/tcp": "invalid port format",
		"53:53/udp": "invalid port format",
		"80:80":     "invalid port format",
		"80/xyz":    "invalid protocol",
		"tcp":       "invalid syntax",
		"udp":       "invalid syntax",
		"":          "invalid protocol",
	}
	for _, port := range validPorts {
		_, err := validatePublishRemove(port)
		assert.Equal(t, err, nil)
	}
	for port, e := range invalidPorts {
		_, err := validatePublishRemove(port)
		assert.Error(t, err, e)
	}
}

type secretAPIClientMock struct {
	listResult []swarm.Secret
}

func (s secretAPIClientMock) SecretList(ctx context.Context, options types.SecretListOptions) ([]swarm.Secret, error) {
	return s.listResult, nil
}
func (s secretAPIClientMock) SecretCreate(ctx context.Context, secret swarm.SecretSpec) (types.SecretCreateResponse, error) {
	return types.SecretCreateResponse{}, nil
}
func (s secretAPIClientMock) SecretRemove(ctx context.Context, id string) error {
	return nil
}
func (s secretAPIClientMock) SecretInspectWithRaw(ctx context.Context, name string) (swarm.Secret, []byte, error) {
	return swarm.Secret{}, []byte{}, nil
}
func (s secretAPIClientMock) SecretUpdate(ctx context.Context, id string, version swarm.Version, secret swarm.SecretSpec) error {
	return nil
}

// TestUpdateSecretUpdateInPlace tests the ability to update the "target" of an secret with "docker service update"
// by combining "--secret-rm" and "--secret-add" for the same secret.
func TestUpdateSecretUpdateInPlace(t *testing.T) {
	apiClient := secretAPIClientMock{
		listResult: []swarm.Secret{
			{
				ID:   "tn9qiblgnuuut11eufquw5dev",
				Spec: swarm.SecretSpec{Annotations: swarm.Annotations{Name: "foo"}},
			},
		},
	}

	flags := newUpdateCommand(nil).Flags()
	flags.Set("secret-add", "source=foo,target=foo2")
	flags.Set("secret-rm", "foo")

	secrets := []*swarm.SecretReference{
		{
			File: &swarm.SecretReferenceFileTarget{
				Name: "foo",
				UID:  "0",
				GID:  "0",
				Mode: 292,
			},
			SecretID:   "tn9qiblgnuuut11eufquw5dev",
			SecretName: "foo",
		},
	}

	updatedSecrets, err := getUpdatedSecrets(apiClient, flags, secrets)

	assert.Equal(t, err, nil)
	assert.Equal(t, len(updatedSecrets), 1)
	assert.Equal(t, updatedSecrets[0].SecretID, "tn9qiblgnuuut11eufquw5dev")
	assert.Equal(t, updatedSecrets[0].SecretName, "foo")
	assert.Equal(t, updatedSecrets[0].File.Name, "foo2")
}

func TestUpdateReadOnly(t *testing.T) {
	spec := &swarm.ServiceSpec{}
	cspec := &spec.TaskTemplate.ContainerSpec

	// Update with --read-only=true, changed to true
	flags := newUpdateCommand(nil).Flags()
	flags.Set("read-only", "true")
	updateService(flags, spec)
	assert.Equal(t, cspec.ReadOnly, true)

	// Update without --read-only, no change
	flags = newUpdateCommand(nil).Flags()
	updateService(flags, spec)
	assert.Equal(t, cspec.ReadOnly, true)

	// Update with --read-only=false, changed to false
	flags = newUpdateCommand(nil).Flags()
	flags.Set("read-only", "false")
	updateService(flags, spec)
	assert.Equal(t, cspec.ReadOnly, false)
}
