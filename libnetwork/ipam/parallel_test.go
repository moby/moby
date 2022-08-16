package ipam

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/docker/docker/libnetwork/ipamapi"
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
	a, err := getAllocator(false)
	if err != nil {
		t.Fatal(err)
	}
	a.addrSpaces["giallo"] = &addrSpace{
		id:      dsConfigKey + "/" + "giallo",
		ds:      a.addrSpaces[localAddressSpace].ds,
		alloc:   a.addrSpaces[localAddressSpace].alloc,
		scope:   a.addrSpaces[localAddressSpace].scope,
		subnets: map[SubnetKey]*PoolData{},
	}

	network := fmt.Sprintf("192.168.100.0/%d", mask)
	// total ips 2^(32-mask) - 2 (network and broadcast)
	totalIps := 1<<uint(32-mask) - 2

	pid, _, _, err := a.RequestPool("giallo", network, "", nil, false)
	if err != nil {
		t.Fatal(err)
	}

	return &testContext{
		a:      a,
		opts:   options,
		ipList: make([]*net.IPNet, 0, totalIps),
		ipMap:  make(map[string]bool),
		pid:    pid,
		maxIP:  totalIps,
	}
}

func TestDebug(t *testing.T) {
	tctx := newTestContext(t, 23, map[string]string{ipamapi.AllocSerialPrefix: "true"})
	tctx.a.RequestAddress(tctx.pid, nil, map[string]string{ipamapi.AllocSerialPrefix: "true"})
	tctx.a.RequestAddress(tctx.pid, nil, map[string]string{ipamapi.AllocSerialPrefix: "true"})
}

type op struct {
	id   int32
	add  bool
	name string
}

func (o *op) String() string {
	return fmt.Sprintf("%+v", *o)
}

func TestRequestPoolParallel(t *testing.T) {
	a, err := getAllocator(false)
	if err != nil {
		t.Fatal(err)
	}
	var operationIndex int32
	ch := make(chan *op, 240)

	group := new(errgroup.Group)
	defer func() {
		if err := group.Wait(); err != nil {
			t.Fatal(err)
		}
	}()

	for i := 0; i < 120; i++ {
		group.Go(func() error {
			name, _, _, err := a.RequestPool("GlobalDefault", "", "", nil, false)
			if err != nil {
				t.Log(err) // log so we can see the error in real time rather than at the end when we actually call "Wait".
				return fmt.Errorf("request error %v", err)
			}
			idx := atomic.AddInt32(&operationIndex, 1)
			ch <- &op{idx, true, name}
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			idx = atomic.AddInt32(&operationIndex, 1)
			err = a.ReleasePool(name)
			if err != nil {
				t.Log(err) // log so we can see the error in real time rather than at the end when we actually call "Wait".
				return fmt.Errorf("release error %v", err)
			}
			ch <- &op{idx, false, name}
			return nil
		})
	}

	// map of events
	m := make(map[string][]*op)
	for i := 0; i < 240; i++ {
		x := <-ch
		ops, ok := m[x.name]
		if !ok {
			ops = make([]*op, 0, 10)
		}
		ops = append(ops, x)
		m[x.name] = ops
	}

	// Post processing to avoid event reordering on the channel
	for pool, ops := range m {
		sort.Slice(ops[:], func(i, j int) bool {
			return ops[i].id < ops[j].id
		})
		expected := true
		for _, op := range ops {
			if op.add != expected {
				t.Fatalf("Operations for %v not valid %v, operations %v", pool, op, ops)
			}
			expected = !expected
		}
	}
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
