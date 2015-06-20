package libnetwork

import (
	"testing"

	"github.com/docker/libnetwork/sandbox"
)

func createEmptyCtrlr() *controller {
	return &controller{sandboxes: sandboxTable{}}
}

func createEmptyEndpoint() *endpoint {
	return &endpoint{
		container: &containerInfo{},
		joinInfo:  &endpointJoinInfo{},
		iFaces:    []*endpointInterface{},
	}
}

func TestSandboxAddEmpty(t *testing.T) {
	ctrlr := createEmptyCtrlr()
	ep := createEmptyEndpoint()

	if _, err := ctrlr.sandboxAdd(sandbox.GenerateKey("sandbox1"), true, ep); err != nil {
		t.Fatal(err)
	}

	ctrlr.sandboxRm(sandbox.GenerateKey("sandbox1"), ep)

	ctrlr.LeaveAll("sandbox1")
	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller sandboxes is not empty. len = %d", len(ctrlr.sandboxes))
	}

	sandbox.GC()
}

func TestSandboxAddMultiPrio(t *testing.T) {
	ctrlr := createEmptyCtrlr()
	ep1 := createEmptyEndpoint()
	ep2 := createEmptyEndpoint()
	ep3 := createEmptyEndpoint()

	ep1.container.config.prio = 1
	ep2.container.config.prio = 2
	ep3.container.config.prio = 3

	sKey := sandbox.GenerateKey("sandbox1")

	if _, err := ctrlr.sandboxAdd(sKey, true, ep1); err != nil {
		t.Fatal(err)
	}

	if _, err := ctrlr.sandboxAdd(sKey, true, ep2); err != nil {
		t.Fatal(err)
	}

	if _, err := ctrlr.sandboxAdd(sKey, true, ep3); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sKey].endpoints[0] != ep3 {
		t.Fatal("Expected ep3 to be at the top of the heap. But did not find ep3 at the top of the heap")
	}

	ctrlr.sandboxRm(sKey, ep3)

	if ctrlr.sandboxes[sKey].endpoints[0] != ep2 {
		t.Fatal("Expected ep2 to be at the top of the heap after removing ep3. But did not find ep2 at the top of the heap")
	}

	ctrlr.sandboxRm(sKey, ep2)

	if ctrlr.sandboxes[sKey].endpoints[0] != ep1 {
		t.Fatal("Expected ep1 to be at the top of the heap after removing ep2. But did not find ep1 at the top of the heap")
	}

	// Re-add ep3 back
	if _, err := ctrlr.sandboxAdd(sKey, true, ep3); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sKey].endpoints[0] != ep3 {
		t.Fatal("Expected ep3 to be at the top of the heap after adding ep3 back. But did not find ep3 at the top of the heap")
	}

	ctrlr.sandboxRm(sKey, ep3)
	ctrlr.sandboxRm(sKey, ep1)

	if err := ctrlr.LeaveAll("sandbox1"); err != nil {
		t.Fatal(err)
	}

	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller sandboxes is not empty. len = %d", len(ctrlr.sandboxes))
	}

	sandbox.GC()
}

func TestSandboxAddSamePrio(t *testing.T) {
	ctrlr := createEmptyCtrlr()
	ep1 := createEmptyEndpoint()
	ep2 := createEmptyEndpoint()

	ep1.network = &network{name: "aaa"}
	ep2.network = &network{name: "bbb"}

	sKey := sandbox.GenerateKey("sandbox1")

	if _, err := ctrlr.sandboxAdd(sKey, true, ep1); err != nil {
		t.Fatal(err)
	}

	if _, err := ctrlr.sandboxAdd(sKey, true, ep2); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes[sKey].endpoints[0] != ep1 {
		t.Fatal("Expected ep1 to be at the top of the heap. But did not find ep1 at the top of the heap")
	}

	ctrlr.sandboxRm(sKey, ep1)

	if ctrlr.sandboxes[sKey].endpoints[0] != ep2 {
		t.Fatal("Expected ep2 to be at the top of the heap after removing ep3. But did not find ep2 at the top of the heap")
	}

	ctrlr.sandboxRm(sKey, ep2)

	if err := ctrlr.LeaveAll("sandbox1"); err != nil {
		t.Fatal(err)
	}

	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller sandboxes is not empty. len = %d", len(ctrlr.sandboxes))
	}

	sandbox.GC()
}
