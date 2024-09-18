package defaultipam

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	all = iota
	even
	odd
)

type releaseMode uint

type testContext struct {
	a      *Allocator
	opts   map[string]string
	ipList []*net.IPNet
	ipMap  map[string]bool
	pid    string
	maxIP  int
}

func newTestContext(t *testing.T, mask int, options map[string]string) *testContext {
	a, err := NewAllocator(ipamutils.GetLocalScopeDefaultNetworks(), ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		t.Fatal(err)
	}
	network := fmt.Sprintf("192.168.100.0/%d", mask)
	// total ips 2^(32-mask) - 2 (network and broadcast)
	totalIps := 1<<uint(32-mask) - 2

	alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace, Pool: network})
	if err != nil {
		t.Fatal(err)
	}

	return &testContext{
		a:      a,
		opts:   options,
		ipList: make([]*net.IPNet, 0, totalIps),
		ipMap:  make(map[string]bool),
		pid:    alloc.PoolID,
		maxIP:  totalIps,
	}
}

func TestDebug(t *testing.T) {
	tctx := newTestContext(t, 23, map[string]string{ipamapi.AllocSerialPrefix: "true"})
	tctx.a.RequestAddress(tctx.pid, nil, map[string]string{ipamapi.AllocSerialPrefix: "true"})
	tctx.a.RequestAddress(tctx.pid, nil, map[string]string{ipamapi.AllocSerialPrefix: "true"})
}

func TestRequestPoolParallel(t *testing.T) {
	a, err := NewAllocator([]*ipamutils.NetworkToSplit{
		{Base: netip.MustParsePrefix("10.0.0.0/10"), Size: 24},
	}, nil)
	assert.NilError(t, err)

	var expected []string
	var eg errgroup.Group
	imax := 1 << (a.local4.predefined[0].Size - a.local4.predefined[0].Base.Bits())
	allocCh := make(chan string, imax)

	for i := 0; i < imax; i++ {
		expected = append(expected, fmt.Sprintf("10.%d.%d.0/24", uint(i/256), i%256))

		eg.Go(func() error {
			t.Helper()

			alloc, err := a.RequestPool(ipamapi.PoolRequest{AddressSpace: localAddressSpace})
			if err != nil {
				return err
			}

			allocCh <- alloc.Pool.String()

			return nil
		})
	}

	assert.NilError(t, eg.Wait())
	close(allocCh)

	var allocated []string
	for alloc := range allocCh {
		allocated = append(allocated, alloc)
	}

	for _, exp := range expected {
		assert.Check(t, slices.Contains(allocated, exp) == true)
	}
	assert.Equal(t, len(allocated), len(expected))
}

func TestFullAllocateRelease(t *testing.T) {
	for _, parallelism := range []int64{2, 4, 8} {
		for _, mask := range []int{29, 25, 24, 21} {
			tctx := newTestContext(t, mask, map[string]string{ipamapi.AllocSerialPrefix: "true"})
			allocate(t, tctx, parallelism)
			release(t, tctx, all, parallelism)
		}
	}
}

func TestOddAllocateRelease(t *testing.T) {
	for _, parallelism := range []int64{2, 4, 8} {
		for _, mask := range []int{29, 25, 24, 21} {
			tctx := newTestContext(t, mask, map[string]string{ipamapi.AllocSerialPrefix: "true"})
			allocate(t, tctx, parallelism)
			release(t, tctx, odd, parallelism)
		}
	}
}

func TestFullAllocateSerialReleaseParallel(t *testing.T) {
	for _, parallelism := range []int64{1, 4, 8} {
		tctx := newTestContext(t, 23, map[string]string{ipamapi.AllocSerialPrefix: "true"})
		allocate(t, tctx, 1)
		release(t, tctx, all, parallelism)
	}
}

func TestOddAllocateSerialReleaseParallel(t *testing.T) {
	for _, parallelism := range []int64{1, 4, 8} {
		tctx := newTestContext(t, 23, map[string]string{ipamapi.AllocSerialPrefix: "true"})
		allocate(t, tctx, 1)
		release(t, tctx, odd, parallelism)
	}
}

func TestEvenAllocateSerialReleaseParallel(t *testing.T) {
	for _, parallelism := range []int64{1, 4, 8} {
		tctx := newTestContext(t, 23, map[string]string{ipamapi.AllocSerialPrefix: "true"})
		allocate(t, tctx, 1)
		release(t, tctx, even, parallelism)
	}
}

func allocate(t *testing.T, tctx *testContext, parallel int64) {
	// Allocate the whole space
	parallelExec := semaphore.NewWeighted(parallel)
	routineNum := tctx.maxIP + 10
	ch := make(chan *net.IPNet, routineNum)
	var id int
	var wg sync.WaitGroup
	// routine loop
	for {
		wg.Add(1)
		go func(id int) {
			parallelExec.Acquire(context.Background(), 1)
			ip, _, _ := tctx.a.RequestAddress(tctx.pid, nil, tctx.opts)
			ch <- ip
			parallelExec.Release(1)
			wg.Done()
		}(id)
		id++
		if id == routineNum {
			break
		}
	}

	// give time to all the go routines to finish
	wg.Wait()

	// process results
	for i := 0; i < routineNum; i++ {
		ip := <-ch
		if ip == nil {
			continue
		}
		if there, ok := tctx.ipMap[ip.String()]; ok && there {
			t.Fatalf("Got duplicate IP %s", ip.String())
		}
		tctx.ipList = append(tctx.ipList, ip)
		tctx.ipMap[ip.String()] = true
	}

	assert.Assert(t, is.Len(tctx.ipList, tctx.maxIP))
}

func release(t *testing.T, tctx *testContext, mode releaseMode, parallel int64) {
	var startIndex, increment, stopIndex, length int
	switch mode {
	case all:
		startIndex = 0
		increment = 1
		stopIndex = tctx.maxIP - 1
		length = tctx.maxIP
	case odd, even:
		if mode == odd {
			startIndex = 1
		}
		increment = 2
		stopIndex = tctx.maxIP - 1
		length = tctx.maxIP / 2
		if tctx.maxIP%2 > 0 {
			length++
		}
	default:
		t.Fatal("unsupported mode yet")
	}

	ipIndex := make([]int, 0, length)
	// calculate the index to release from the ipList
	for i := startIndex; ; i += increment {
		ipIndex = append(ipIndex, i)
		if i+increment > stopIndex {
			break
		}
	}

	var id int
	parallelExec := semaphore.NewWeighted(parallel)
	ch := make(chan *net.IPNet, len(ipIndex))
	group := new(errgroup.Group)
	for index := range ipIndex {
		index := index
		group.Go(func() error {
			parallelExec.Acquire(context.Background(), 1)
			err := tctx.a.ReleaseAddress(tctx.pid, tctx.ipList[index].IP)
			if err != nil {
				return fmt.Errorf("routine %d got %v", id, err)
			}
			ch <- tctx.ipList[index]
			parallelExec.Release(1)
			return nil
		})
		id++
	}

	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < len(ipIndex); i++ {
		ip := <-ch

		// check if it is really free
		_, _, err := tctx.a.RequestAddress(tctx.pid, ip.IP, nil)
		assert.Check(t, err, "ip %v not properly released", ip)
		if err != nil {
			t.Fatalf("ip %v not properly released, error:%v", ip, err)
		}
		err = tctx.a.ReleaseAddress(tctx.pid, ip.IP)
		assert.NilError(t, err)

		if there, ok := tctx.ipMap[ip.String()]; !ok || !there {
			t.Fatalf("ip %v got double deallocated", ip)
		}
		tctx.ipMap[ip.String()] = false
		for j, v := range tctx.ipList {
			if v == ip {
				tctx.ipList = append(tctx.ipList[:j], tctx.ipList[j+1:]...)
				break
			}
		}
	}

	assert.Check(t, is.Len(tctx.ipList, tctx.maxIP-length))
}
