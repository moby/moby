// Package addrset implements a set of IP addresses.
package addrset

import (
	"errors"
	"fmt"
	"math/bits"
	"net"
	"net/netip"
	"strings"

	"github.com/moby/moby/v2/daemon/internal/netiputil"
	"github.com/moby/moby/v2/daemon/libnetwork/bitmap"
	"github.com/moby/moby/v2/daemon/libnetwork/ipbits"
)

var (
	// ErrNotAvailable is returned when no more addresses are available to set
	ErrNotAvailable = errors.New("address not available")
	// ErrAllocated is returned when the specific address requested is already allocated
	ErrAllocated = errors.New("address already allocated")
)

const (
	// maxBitsPerBitmap is the max size for a single bitmap in the address set.
	//
	// [bitmap.Bitmap] is initialised with a uint64 num-bits. So, it can't contain
	// enough bits for a 64-bit range (it's one bit short, the last address in the
	// range can't be represented). If that's fixed, this max can be increased, but
	// addrsPerBitmap() will need updating to deal with the overflow.
	//
	// A max of 63-bits means a 64-bit address range (the norm for IPv6) is
	// represented by up-to two bitmaps.
	maxBitsPerBitmap = 63
	// minPrefixLen is the prefix length corresponding to maxBitsPerBitmap
	minPrefixLen = (net.IPv6len * 8) - maxBitsPerBitmap
)

// AddrSet is a set of IP addresses.
type AddrSet struct {
	pool    netip.Prefix
	bitmaps map[netip.Prefix]*bitmap.Bitmap
}

// New returns an AddrSet for the range of addresses in pool.
func New(pool netip.Prefix) *AddrSet {
	return &AddrSet{
		pool:    pool.Masked(),
		bitmaps: map[netip.Prefix]*bitmap.Bitmap{},
	}
}

// Add adds address addr to the set. If addr is already in the set, it returns a
// wrapped [ErrAllocated]. If addr is not in the set's address range, it returns
// an error.
func (as *AddrSet) Add(addr netip.Addr) error {
	if !as.pool.Contains(addr) {
		return fmt.Errorf("cannot add %s to '%s'", addr, as.pool)
	}
	bm, _, err := as.getBitmap(addr)
	if err != nil {
		return fmt.Errorf("finding bitmap for %s in '%s': %w", addr, as.pool, err)
	}
	bit := netiputil.HostID(addr, as.prefixLenPerBitmap())
	if err := bm.Set(bit); err != nil {
		return fmt.Errorf("setting bit %d for %s in pool '%s': %w", bit, addr, as.pool, mapErr(err))
	}
	return nil
}

// AddAny adds an arbitrary address to the set, and returns that address. Or, if
// no addresses are available, it returns a wrapped [ErrNotAvailable].
//
// If the address set's pool contains fewer than 1<<maxBitsPerBitmap addresses,
// AddAny will add any address from the entire set. If the pool is bigger than
// that, AddAny will only consider the first 1<<maxBitsPerBitmap addresses. If
// those are all allocated, it returns [ErrNotAvailable].
//
// When serial=true, the set is scanned starting from the address following
// the address most recently set by [AddrSet.AddAny] (or [AddrSet.AddAnyInRange]
// if the range is in the same 1<<maxBitsPerBitmap .
func (as *AddrSet) AddAny(serial bool) (netip.Addr, error) {
	// Only look at the first bitmap. It either contains the whole address range or
	// the first 1<<maxBitsPerBitmap addresses, which is a lot. (So, no need to
	// search other bitmaps, or work out if more bitmaps could be created).
	bm, _, err := as.getBitmap(as.pool.Addr())
	if err != nil {
		return netip.Addr{}, fmt.Errorf("no bitmap to add-any to '%s': %w", as.pool.Addr(), err)
	}
	ordinal, err := bm.SetAny(serial)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("add-any to '%s': %w", as.pool.Addr(), mapErr(err))
	}
	return ipbits.Add(as.pool.Addr(), ordinal, 0), nil
}

// AddAnyInRange adds an arbitrary address from ipr to the set, and returns that
// address. Or, if no addresses are available, it returns a wrapped [ErrNotAvailable].
// If ipr is not fully contained within the set's range, it returns an error.
//
// When serial=true, the set is scanned starting from the address following
// the address most recently set by [AddrSet.AddAny] or [AddrSet.AddAnyInRange].
func (as *AddrSet) AddAnyInRange(ipr netip.Prefix, serial bool) (netip.Addr, error) {
	if !as.pool.Contains(ipr.Addr()) || ipr.Bits() < as.pool.Bits() {
		return netip.Addr{}, fmt.Errorf("add-any, range '%s' is not in subnet '%s'", ipr, as.pool)
	}
	iprMasked := ipr.Masked()
	bm, bmKey, err := as.getBitmap(iprMasked.Addr())
	if err != nil {
		return netip.Addr{}, fmt.Errorf("no bitmap to add-any in '%s' range '%s': %w", as.pool, ipr, err)
	}
	var ordinal uint64
	if ipr.Bits() <= bmKey.Bits() {
		ordinal, err = bm.SetAny(serial)
	} else {
		start, end := netiputil.SubnetRange(bmKey, iprMasked)
		ordinal, err = bm.SetAnyInRange(start, end, serial)
	}
	if err != nil {
		return netip.Addr{}, fmt.Errorf("add-any in '%s' range '%s': %w", as.pool, ipr, mapErr(err))
	}
	return ipbits.Add(bmKey.Addr(), ordinal, 0), nil
}

// Remove removes addr from the set or, if addr is not in the set's address range it
// returns an error. If addr is not in the set, it returns nil (removing an address
// that's not in the set is not an error).
func (as *AddrSet) Remove(addr netip.Addr) error {
	if !as.pool.Contains(addr) {
		return fmt.Errorf("%s cannot be removed from '%s'", addr, as.pool)
	}
	bm, bmKey, err := as.getBitmap(addr)
	if err != nil {
		return fmt.Errorf("remove '%s' from '%s': %w", addr, as.pool, err)
	}
	bit := netiputil.HostID(addr, as.prefixLenPerBitmap())
	if err := bm.Unset(bit); err != nil {
		return fmt.Errorf("unset bit %d for '%s' in '%s': %w", bit, addr, as.pool, err)
	}
	if bm.Bits()-bm.Unselected() == 0 {
		delete(as.bitmaps, bmKey)
	}
	return nil
}

// String returns a description of the address set.
func (as *AddrSet) String() string {
	if len(as.bitmaps) == 0 {
		return "empty address set"
	}
	if as.pool.Addr().BitLen()-as.pool.Bits() <= maxBitsPerBitmap {
		return as.bitmaps[as.pool].String()
	}
	bmStrings := make([]string, 0, len(as.bitmaps))
	for bmKey, bm := range as.bitmaps {
		bmStrings = append(bmStrings, fmt.Sprintf("range %s %s", bmKey, bm))
	}
	return strings.Join(bmStrings, " ")
}

// Len returns the number of addresses in the set.
func (as *AddrSet) Len() (hi, lo uint64) {
	for _, bm := range as.bitmaps {
		var carry uint64
		lo, carry = bits.Add64(lo, bm.Bits()-bm.Unselected(), 0)
		hi += carry
	}
	return hi, lo
}

// AddrsInPrefix returns the number of addresses in the set which have the given prefix.
func (as *AddrSet) AddrsInPrefix(prefix netip.Prefix) (hi, lo uint64) {
	prefix = prefix.Masked()
	if !as.pool.Overlaps(prefix) {
		return 0, 0
	}
	if prefix.Bits() <= as.pool.Bits() {
		return as.Len()
	}
	for bmKey, bm := range as.bitmaps {
		if !prefix.Overlaps(bmKey) {
			continue
		}
		var ones uint64
		if prefix.Bits() <= bmKey.Bits() {
			ones = bm.Bits() - bm.Unselected()
		} else {
			var err error
			ones, err = bm.OnesCount(netiputil.SubnetRange(bmKey, prefix))
			// Since OnesCount only returns an error if the range
			// exceeds the bitmap bounds, and we are responsible for
			// picking the bitmap and the range to count, any error
			// returned here is therefore a programming error.
			if err != nil {
				panic(fmt.Sprintf("OnesCount failed for bitmap key %v and prefix %v: %v", bmKey, prefix, err))
			}
		}
		var carry uint64
		lo, carry = bits.Add64(lo, ones, 0)
		hi += carry
	}
	return hi, lo
}

func (as *AddrSet) getBitmap(addr netip.Addr) (*bitmap.Bitmap, netip.Prefix, error) {
	bits := min(as.pool.Addr().BitLen()-as.pool.Bits(), maxBitsPerBitmap)
	bmKey, err := addr.Prefix(as.pool.Addr().BitLen() - bits)
	if err != nil {
		return nil, netip.Prefix{}, err
	}
	bm, ok := as.bitmaps[bmKey]
	if !ok {
		bm = bitmap.New(as.addrsPerBitmap())
		as.bitmaps[bmKey] = bm
	}
	return bm, bmKey, nil
}

func (as *AddrSet) addrsPerBitmap() uint64 {
	bits := min(as.pool.Addr().BitLen()-as.pool.Bits(), maxBitsPerBitmap)
	return uint64(1) << bits
}

func (as *AddrSet) prefixLenPerBitmap() uint {
	bits := as.pool.Bits()
	if as.pool.Addr().Is6() && bits < minPrefixLen {
		return minPrefixLen
	}
	return uint(bits)
}

func mapErr(err error) error {
	if errors.Is(err, bitmap.ErrBitAllocated) {
		return ErrAllocated
	}
	if errors.Is(err, bitmap.ErrNoBitAvailable) {
		return ErrNotAvailable
	}
	return err
}
