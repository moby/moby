package libnetwork

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"github.com/docker/docker/libnetwork/osl"
	"github.com/docker/docker/libnetwork/testutils"
	"gotest.tools/v3/skip"
)

func getTestEnv(t *testing.T, opts ...[]NetworkOption) (NetworkController, []Network) {
	skip.If(t, runtime.GOOS == "windows", "test only works on linux")

	netType := "bridge"

	option := options.Generic{
		"EnableIPForwarding": true,
	}
	genericOption := make(map[string]interface{})
	genericOption[netlabel.GenericData] = option

	cfgOptions, err := OptionBoltdbWithRandomDBFile()
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(append(cfgOptions, config.OptionDriverConfig(netType, genericOption))...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Stop)

	if len(opts) == 0 {
		return c, nil
	}

	nwList := make([]Network, 0, len(opts))
	for i, opt := range opts {
		name := fmt.Sprintf("test_nw_%d", i)
		netOption := options.Generic{
			netlabel.GenericData: options.Generic{
				"BridgeName": name,
			},
		}
		newOptions := make([]NetworkOption, 1, len(opt)+1)
		newOptions[0] = NetworkOptionGeneric(netOption)
		newOptions = append(newOptions, opt...)
		n, err := c.NewNetwork(netType, name, "", newOptions...)
		if err != nil {
			t.Fatal(err)
		}

		nwList = append(nwList, n)
	}

	return c, nwList
}

func TestSandboxAddEmpty(t *testing.T) {
	c, _ := getTestEnv(t)
	ctrlr := c.(*controller)

	sbx, err := ctrlr.NewSandbox("sandbox0")
	if err != nil {
		t.Fatal(err)
	}

	if err := sbx.Delete(); err != nil {
		t.Fatal(err)
	}

	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller sandboxes is not empty. len = %d", len(ctrlr.sandboxes))
	}

	osl.GC()
}

// // If different priorities are specified, internal option and ipv6 addresses mustn't influence endpoint order
func TestSandboxAddMultiPrio(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()

	opts := [][]NetworkOption{
		{NetworkOptionEnableIPv6(true), NetworkOptionIpam(ipamapi.DefaultIPAM, "", nil, []*IpamConf{{PreferredPool: "fe90::/64"}}, nil)},
		{NetworkOptionInternalNetwork()},
		{},
	}

	c, nws := getTestEnv(t, opts...)
	ctrlr := c.(*controller)

	sbx, err := ctrlr.NewSandbox("sandbox1")
	if err != nil {
		t.Fatal(err)
	}
	sid := sbx.ID()

	ep1, err := nws[0].CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	ep2, err := nws[1].CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}
	ep3, err := nws[2].CreateEndpoint("ep3")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep1.Join(sbx, JoinOptionPriority(1)); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Join(sbx, JoinOptionPriority(2)); err != nil {
		t.Fatal(err)
	}

	if err := ep3.Join(sbx, JoinOptionPriority(3)); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sid].endpoints[0].ID() != ep3.ID() {
		t.Fatal("Expected ep3 to be at the top of the heap. But did not find ep3 at the top of the heap")
	}

	if len(sbx.Endpoints()) != 3 {
		t.Fatal("Expected 3 endpoints to be connected to the sandbox.")
	}

	if err := ep3.Leave(sbx); err != nil {
		t.Fatal(err)
	}
	if ctrlr.sandboxes[sid].endpoints[0].ID() != ep2.ID() {
		t.Fatal("Expected ep2 to be at the top of the heap after removing ep3. But did not find ep2 at the top of the heap")
	}

	if err := ep2.Leave(sbx); err != nil {
		t.Fatal(err)
	}
	if ctrlr.sandboxes[sid].endpoints[0].ID() != ep1.ID() {
		t.Fatal("Expected ep1 to be at the top of the heap after removing ep2. But did not find ep1 at the top of the heap")
	}

	// Re-add ep3 back
	if err := ep3.Join(sbx, JoinOptionPriority(3)); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sid].endpoints[0].ID() != ep3.ID() {
		t.Fatal("Expected ep3 to be at the top of the heap after adding ep3 back. But did not find ep3 at the top of the heap")
	}

	if err := sbx.Delete(); err != nil {
		t.Fatal(err)
	}

	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller sandboxes is not empty. len = %d", len(ctrlr.sandboxes))
	}

	osl.GC()
}

func TestSandboxAddSamePrio(t *testing.T) {
	defer testutils.SetupTestOSContext(t)()

	opts := [][]NetworkOption{
		{},
		{},
		{NetworkOptionEnableIPv6(true), NetworkOptionIpam(ipamapi.DefaultIPAM, "", nil, []*IpamConf{{PreferredPool: "fe90::/64"}}, nil)},
		{NetworkOptionInternalNetwork()},
	}

	c, nws := getTestEnv(t, opts...)

	ctrlr := c.(*controller)

	sbx, err := ctrlr.NewSandbox("sandbox1")
	if err != nil {
		t.Fatal(err)
	}
	sid := sbx.ID()

	epNw1, err := nws[1].CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	epIPv6, err := nws[2].CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}

	epInternal, err := nws[3].CreateEndpoint("ep3")
	if err != nil {
		t.Fatal(err)
	}

	epNw0, err := nws[0].CreateEndpoint("ep4")
	if err != nil {
		t.Fatal(err)
	}

	if err := epNw1.Join(sbx); err != nil {
		t.Fatal(err)
	}

	if err := epIPv6.Join(sbx); err != nil {
		t.Fatal(err)
	}

	if err := epInternal.Join(sbx); err != nil {
		t.Fatal(err)
	}

	if err := epNw0.Join(sbx); err != nil {
		t.Fatal(err)
	}

	// order should now be: epIPv6, epNw0, epNw1, epInternal
	if len(sbx.Endpoints()) != 4 {
		t.Fatal("Expected 4 endpoints to be connected to the sandbox.")
	}

	// IPv6 has precedence over IPv4
	if ctrlr.sandboxes[sid].endpoints[0].ID() != epIPv6.ID() {
		t.Fatal("Expected epIPv6 to be at the top of the heap. But did not find epIPv6 at the top of the heap")
	}

	// internal network has lowest precedence
	if ctrlr.sandboxes[sid].endpoints[3].ID() != epInternal.ID() {
		t.Fatal("Expected epInternal to be at the bottom of the heap. But did not find epInternal at the bottom of the heap")
	}

	if err := epIPv6.Leave(sbx); err != nil {
		t.Fatal(err)
	}

	// 'test_nw_0' has precedence over 'test_nw_1'
	if ctrlr.sandboxes[sid].endpoints[0].ID() != epNw0.ID() {
		t.Fatal("Expected epNw0 to be at the top of the heap after removing epIPv6. But did not find epNw0 at the top of the heap")
	}

	if err := epNw1.Leave(sbx); err != nil {
		t.Fatal(err)
	}

	if err := sbx.Delete(); err != nil {
		t.Fatal(err)
	}

	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller containers is not empty. len = %d", len(ctrlr.sandboxes))
	}

	osl.GC()
}
