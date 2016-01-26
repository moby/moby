package libnetwork

import (
	"testing"

	"github.com/docker/libnetwork/config"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
	"github.com/docker/libnetwork/osl"
	"github.com/docker/libnetwork/testutils"
)

func createEmptyCtrlr() *controller {
	return &controller{sandboxes: sandboxTable{}}
}

func getTestEnv(t *testing.T) (NetworkController, Network, Network) {
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

	name1 := "test_nw_1"
	netOption1 := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": name1,
		},
	}
	n1, err := c.NewNetwork(netType, name1, NetworkOptionGeneric(netOption1))
	if err != nil {
		t.Fatal(err)
	}

	name2 := "test_nw_2"
	netOption2 := options.Generic{
		netlabel.GenericData: options.Generic{
			"BridgeName": name2,
		},
	}
	n2, err := c.NewNetwork(netType, name2, NetworkOptionGeneric(netOption2))
	if err != nil {
		t.Fatal(err)
	}

	return c, n1, n2
}

func TestSandboxAddEmpty(t *testing.T) {
	ctrlr := createEmptyCtrlr()

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

func TestSandboxAddMultiPrio(t *testing.T) {
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	c, nw, _ := getTestEnv(t)
	ctrlr := c.(*controller)

	sbx, err := ctrlr.NewSandbox("sandbox1")
	if err != nil {
		t.Fatal(err)
	}
	sid := sbx.ID()

	ep1, err := nw.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	ep2, err := nw.CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}
	ep3, err := nw.CreateEndpoint("ep3")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep1.Join(sbx, JoinOptionPriority(ep1, 1)); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Join(sbx, JoinOptionPriority(ep2, 2)); err != nil {
		t.Fatal(err)
	}

	if err := ep3.Join(sbx, JoinOptionPriority(ep3, 3)); err != nil {
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
	if err := ep3.Join(sbx, JoinOptionPriority(ep3, 3)); err != nil {
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
	if !testutils.IsRunningInContainer() {
		defer testutils.SetupTestOSContext(t)()
	}

	c, nw1, nw2 := getTestEnv(t)

	ctrlr := c.(*controller)

	sbx, err := ctrlr.NewSandbox("sandbox1")
	if err != nil {
		t.Fatal(err)
	}
	sid := sbx.ID()

	ep1, err := nw1.CreateEndpoint("ep1")
	if err != nil {
		t.Fatal(err)
	}
	ep2, err := nw2.CreateEndpoint("ep2")
	if err != nil {
		t.Fatal(err)
	}

	if err := ep1.Join(sbx); err != nil {
		t.Fatal(err)
	}

	if err := ep2.Join(sbx); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sid].endpoints[0].ID() != ep1.ID() {
		t.Fatal("Expected ep1 to be at the top of the heap. But did not find ep1 at the top of the heap")
	}

	if err := ep1.Leave(sbx); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sid].endpoints[0].ID() != ep2.ID() {
		t.Fatal("Expected ep2 to be at the top of the heap after removing ep3. But did not find ep2 at the top of the heap")
	}

	if err := ep2.Leave(sbx); err != nil {
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
