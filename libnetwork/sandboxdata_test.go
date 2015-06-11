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

	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes["sandbox1"].refCnt != 1 {
		t.Fatalf("Unexpected sandbox ref count. Expected 1, got %d",
			ctrlr.sandboxes["sandbox1"].refCnt)
	}

	ctrlr.sandboxRm("sandbox1", ep)
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

	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep1); err != nil {
		t.Fatal(err)
	}

	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep2); err != nil {
		t.Fatal(err)
	}

	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep3); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes["sandbox1"].refCnt != 3 {
		t.Fatalf("Unexpected sandbox ref count. Expected 3, got %d",
			ctrlr.sandboxes["sandbox1"].refCnt)
	}

	if ctrlr.sandboxes["sandbox1"].endpoints[0] != ep3 {
		t.Fatal("Expected ep3 to be at the top of the heap. But did not find ep3 at the top of the heap")
	}

	ctrlr.sandboxRm("sandbox1", ep3)

	if ctrlr.sandboxes["sandbox1"].endpoints[0] != ep2 {
		t.Fatal("Expected ep2 to be at the top of the heap after removing ep3. But did not find ep2 at the top of the heap")
	}

	ctrlr.sandboxRm("sandbox1", ep2)

	if ctrlr.sandboxes["sandbox1"].endpoints[0] != ep1 {
		t.Fatal("Expected ep1 to be at the top of the heap after removing ep2. But did not find ep1 at the top of the heap")
	}

	// Re-add ep3 back
	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep3); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes["sandbox1"].endpoints[0] != ep3 {
		t.Fatal("Expected ep3 to be at the top of the heap after adding ep3 back. But did not find ep3 at the top of the heap")
	}

	ctrlr.sandboxRm("sandbox1", ep3)
	ctrlr.sandboxRm("sandbox1", ep1)
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

	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep1); err != nil {
		t.Fatal(err)
	}

	if _, err := ctrlr.sandboxAdd("sandbox1", true, ep2); err != nil {
		t.Fatal(err)
	}

	if ctrlr.sandboxes["sandbox1"].refCnt != 2 {
		t.Fatalf("Unexpected sandbox ref count. Expected 2, got %d",
			ctrlr.sandboxes["sandbox1"].refCnt)
	}

	if ctrlr.sandboxes["sandbox1"].endpoints[0] != ep1 {
		t.Fatal("Expected ep1 to be at the top of the heap. But did not find ep1 at the top of the heap")
	}

	ctrlr.sandboxRm("sandbox1", ep1)

	if ctrlr.sandboxes["sandbox1"].endpoints[0] != ep2 {
		t.Fatal("Expected ep2 to be at the top of the heap after removing ep3. But did not find ep2 at the top of the heap")
	}

	ctrlr.sandboxRm("sandbox1", ep2)
	if len(ctrlr.sandboxes) != 0 {
		t.Fatalf("controller sandboxes is not empty. len = %d", len(ctrlr.sandboxes))
	}

	sandbox.GC()
}
