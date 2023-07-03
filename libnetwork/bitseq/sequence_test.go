package bitseq

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/libnetwork/datastore"
	store "github.com/docker/docker/libnetwork/internal/kvstore"
	"github.com/docker/docker/libnetwork/internal/kvstore/boltdb"
)

var defaultPrefix = filepath.Join(os.TempDir(), "libnetwork", "test", "bitseq")

func init() {
	boltdb.Register()
}

func randomLocalStore() (datastore.DataStore, error) {
	tmp, err := os.CreateTemp("", "libnetwork-")
	if err != nil {
		return nil, fmt.Errorf("Error creating temp file: %v", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("Error closing temp file: %v", err)
	}
	return datastore.NewDataStore(datastore.ScopeCfg{
		Client: datastore.ScopeClientCfg{
			Provider: "boltdb",
			Address:  filepath.Join(defaultPrefix, filepath.Base(tmp.Name())),
			Config: &store.Config{
				Bucket:            "libnetwork",
				ConnectionTimeout: 3 * time.Second,
			},
		},
	})
}

const blockLen = 32

// This one tests an allocation pattern which unveiled an issue in pushReservation
// Specifically a failure in detecting when we are in the (B) case (the bit to set
// belongs to the last block of the current sequence). Because of a bug, code
// was assuming the bit belonged to a block in the middle of the current sequence.
// Which in turn caused an incorrect allocation when requesting a bit which is not
// in the first or last sequence block.
func TestSetAnyInRange(t *testing.T) {
	numBits := uint64(8 * blockLen)
	hnd, err := NewHandle("", nil, "", numBits)
	if err != nil {
		t.Fatal(err)
	}

	if err := hnd.Set(0); err != nil {
		t.Fatal(err)
	}

	if err := hnd.Set(255); err != nil {
		t.Fatal(err)
	}

	o, err := hnd.SetAnyInRange(128, 255, false)
	if err != nil {
		t.Fatal(err)
	}
	if o != 128 {
		t.Fatalf("Unexpected ordinal: %d", o)
	}

	o, err = hnd.SetAnyInRange(128, 255, false)
	if err != nil {
		t.Fatal(err)
	}

	if o != 129 {
		t.Fatalf("Unexpected ordinal: %d", o)
	}

	o, err = hnd.SetAnyInRange(246, 255, false)
	if err != nil {
		t.Fatal(err)
	}
	if o != 246 {
		t.Fatalf("Unexpected ordinal: %d", o)
	}

	o, err = hnd.SetAnyInRange(246, 255, false)
	if err != nil {
		t.Fatal(err)
	}
	if o != 247 {
		t.Fatalf("Unexpected ordinal: %d", o)
	}
}

func TestRandomAllocateDeallocate(t *testing.T) {
	ds, err := randomLocalStore()
	if err != nil {
		t.Fatal(err)
	}

	numBits := int(16 * blockLen)
	hnd, err := NewHandle("bitseq-test/data/", ds, "test1", uint64(numBits))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := hnd.Destroy(); err != nil {
			t.Fatal(err)
		}
	}()

	seed := time.Now().Unix()
	rng := rand.New(rand.NewSource(seed))

	// Allocate all bits using a random pattern
	pattern := rng.Perm(numBits)
	for _, bit := range pattern {
		err := hnd.Set(uint64(bit))
		if err != nil {
			t.Errorf("Unexpected failure on allocation of %d: %v.\nSeed: %d.\n%s", bit, err, seed, hnd)
		}
	}
	if unselected := hnd.Unselected(); unselected != 0 {
		t.Errorf("Expected full sequence. Instead found %d free bits. Seed: %d.\n%s", unselected, seed, hnd)
	}

	// Deallocate all bits using a random pattern
	pattern = rng.Perm(numBits)
	for _, bit := range pattern {
		err := hnd.Unset(uint64(bit))
		if err != nil {
			t.Errorf("Unexpected failure on deallocation of %d: %v.\nSeed: %d.\n%s", bit, err, seed, hnd)
		}
	}
	if unselected := hnd.Unselected(); unselected != uint64(numBits) {
		t.Errorf("Expected full sequence. Instead found %d free bits. Seed: %d.\n%s", unselected, seed, hnd)
	}
}

func TestRetrieveFromStore(t *testing.T) {
	ds, err := randomLocalStore()
	if err != nil {
		t.Fatal(err)
	}

	numBits := int(8 * blockLen)
	hnd, err := NewHandle("bitseq-test/data/", ds, "test1", uint64(numBits))
	if err != nil {
		t.Fatal(err)
	}

	// Allocate first half of the bits
	for i := 0; i < numBits/2; i++ {
		_, err := hnd.SetAny(false)
		if err != nil {
			t.Fatalf("Unexpected failure on allocation %d: %v\n%s", i, err, hnd)
		}
	}
	hnd0 := hnd.String()

	// Retrieve same handle
	hnd, err = NewHandle("bitseq-test/data/", ds, "test1", uint64(numBits))
	if err != nil {
		t.Fatal(err)
	}
	hnd1 := hnd.String()

	if hnd1 != hnd0 {
		t.Fatalf("%v\n%v", hnd0, hnd1)
	}

	err = hnd.Destroy()
	if err != nil {
		t.Fatal(err)
	}
}

func testSetRollover(t *testing.T, serial bool) {
	ds, err := randomLocalStore()
	if err != nil {
		t.Fatal(err)
	}

	numBlocks := uint32(8)
	numBits := int(numBlocks * blockLen)
	hnd, err := NewHandle("bitseq-test/data/", ds, "test1", uint64(numBits))
	if err != nil {
		t.Fatal(err)
	}

	// Allocate first half of the bits
	for i := 0; i < numBits/2; i++ {
		_, err := hnd.SetAny(serial)
		if err != nil {
			t.Fatalf("Unexpected failure on allocation %d: %v\n%s", i, err, hnd)
		}
	}

	if unselected := hnd.Unselected(); unselected != uint64(numBits/2) {
		t.Fatalf("Expected full sequence. Instead found %d free bits. %s", unselected, hnd)
	}

	seed := time.Now().Unix()
	rng := rand.New(rand.NewSource(seed))

	// Deallocate half of the allocated bits following a random pattern
	pattern := rng.Perm(numBits / 2)
	for i := 0; i < numBits/4; i++ {
		bit := pattern[i]
		err := hnd.Unset(uint64(bit))
		if err != nil {
			t.Fatalf("Unexpected failure on deallocation of %d: %v.\nSeed: %d.\n%s", bit, err, seed, hnd)
		}
	}
	if unselected := hnd.Unselected(); unselected != uint64(3*numBits/4) {
		t.Fatalf("Unexpected free bits: found %d free bits.\nSeed: %d.\n%s", unselected, seed, hnd)
	}

	// request to allocate for remaining half of the bits
	for i := 0; i < numBits/2; i++ {
		_, err := hnd.SetAny(serial)
		if err != nil {
			t.Fatalf("Unexpected failure on allocation %d: %v\nSeed: %d\n%s", i, err, seed, hnd)
		}
	}

	// At this point all the bits must be allocated except the randomly unallocated bits
	// which were unallocated in the first half of the bit sequence
	if unselected := hnd.Unselected(); unselected != uint64(numBits/4) {
		t.Fatalf("Unexpected number of unselected bits %d, Expected %d", unselected, numBits/4)
	}

	for i := 0; i < numBits/4; i++ {
		_, err := hnd.SetAny(serial)
		if err != nil {
			t.Fatalf("Unexpected failure on allocation %d: %v\nSeed: %d\n%s", i, err, seed, hnd)
		}
	}
	// Now requesting to allocate the unallocated random bits (qurter of the number of bits) should
	// leave no more bits that can be allocated.
	if hnd.Unselected() != 0 {
		t.Fatalf("Unexpected number of unselected bits %d, Expected %d", hnd.Unselected(), 0)
	}

	err = hnd.Destroy()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetRollover(t *testing.T) {
	testSetRollover(t, false)
}

func TestSetRolloverSerial(t *testing.T) {
	testSetRollover(t, true)
}

func TestMarshalJSON(t *testing.T) {
	const expectedID = "my-bitseq"
	expected := []byte("hello libnetwork")
	hnd, err := NewHandle("", nil, expectedID, uint64(len(expected)*8))
	if err != nil {
		t.Fatal(err)
	}

	for i, c := range expected {
		for j := 0; j < 8; j++ {
			if c&(1<<j) == 0 {
				continue
			}
			if err := hnd.Set(uint64(i*8 + j)); err != nil {
				t.Fatal(err)
			}
		}
	}

	hstr := hnd.String()
	t.Log(hstr)
	marshaled, err := hnd.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() err = %v", err)
	}
	t.Logf("%s", marshaled)

	// Serializations of hnd as would be marshaled by versions of the code
	// found in the wild. We need to support unmarshaling old versions to
	// maintain backwards compatibility with sequences persisted on disk.
	const (
		goldenV0 = `{"id":"my-bitseq","sequence":"AAAAAAAAAIAAAAAAAAAAPRamNjYAAAAAAAAAAfYENpYAAAAAAAAAAUZ2pi4AAAAAAAAAAe72TtYAAAAAAAAAAQ=="}`
	)

	if string(marshaled) != goldenV0 {
		t.Errorf("MarshalJSON() output differs from golden. Please add a new golden case to this test.")
	}

	for _, tt := range []struct {
		name string
		data []byte
	}{
		{name: "Live", data: marshaled},
		{name: "Golden-v0", data: []byte(goldenV0)},
	} {
		tt := tt
		t.Run("UnmarshalJSON="+tt.name, func(t *testing.T) {
			hnd2, err := NewHandle("", nil, "", 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := hnd2.UnmarshalJSON(tt.data); err != nil {
				t.Errorf("UnmarshalJSON() err = %v", err)
			}

			h2str := hnd2.String()
			t.Log(h2str)
			if hstr != h2str {
				t.Errorf("Unmarshaled a different bitseq: want %q, got %q", hstr, h2str)
			}
		})
	}
}
