package osl

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/moby/moby/v2/daemon/internal/sliceutil"
	"github.com/moby/moby/v2/daemon/libnetwork/nlwrap"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"gotest.tools/v3/assert"
)

func TestGenerateIfaceName(t *testing.T) {
	testcases := []struct {
		names []string
		want  string
	}{
		{names: []string{"test0", "test1"}, want: "test2"},
		{names: []string{"test0", "test2"}, want: "test1"},
		{names: []string{"test2"}, want: "test0"},
		{names: []string{"test-0", "test-1"}, want: "test0"},
		{names: []string{}, want: "test0"},
	}

	for _, tc := range testcases {
		ns := &Namespace{
			iFaces: sliceutil.Map(tc.names, func(name string) *Interface {
				return &Interface{dstName: name}
			}),
		}

		got := ns.generateIfaceName("test")
		assert.Equal(t, got, tc.want)
	}
}

// TestAddInterfaceInParallel tests that interface name are correctly generated
// even when many interfaces are added in parallel.
func TestAddInterfaceInParallel(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	nsh, err := netns.NewNamed(t.Name())
	assert.NilError(t, err)
	defer netns.DeleteNamed(t.Name())
	defer nsh.Close()

	nlh, err := nlwrap.NewHandleAt(nsh)
	assert.NilError(t, err)

	ns := &Namespace{
		path:     "/run/netns/" + t.Name(),
		nlHandle: nlh,
	}

	// Create a few dummy interfaces with a dummy name. The call to
	// AddInterface below will rename them into their final name (ie. ethX).
	for i := range 10 {
		nlh.LinkAdd(&netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Name: fmt.Sprintf("dummy%d", i),
			},
		})
	}

	wg := sync.WaitGroup{}
	for i := range 10 {
		src := fmt.Sprintf("dummy%d", i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := ns.AddInterface(context.Background(), src, "eth", "", WithCreatedInContainer(true))
			assert.NilError(t, err)
		}()
	}
	wg.Wait()

	links, err := nlwrap.LinkList()
	assert.NilError(t, err)
	var eths []string
	for _, link := range links {
		if strings.HasPrefix(link.Attrs().Name, "eth") {
			eths = append(eths, link.Attrs().Name)
		}
	}

	sort.Strings(eths)
	assert.DeepEqual(t, eths, []string{"eth0", "eth1", "eth2", "eth3", "eth4", "eth5", "eth6", "eth7", "eth8", "eth9"})
}
