package ipam

import (
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/libkv/store"
	"github.com/docker/libnetwork/bitseq"
	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

const (
	// The biggest configurable host subnets
	minNetSize   = 8
	minNetSizeV6 = 64
	// The effective network size for v6
	minNetSizeV6Eff = 96
	// The size of the host subnet used internally, it's the most granular sequence addresses
	defaultInternalHostSize = 16
	// datastore keyes for ipam obkects
	dsConfigKey = "ipam-config" // ipam-config/<domain>/<map of subent configs>
	dsDataKey   = "ipam-data"   // ipam-data/<domain>/<subnet>/<child-sudbnet>/<bitmask>
)

// Allocator provides per address space ipv4/ipv6 book keeping
type Allocator struct {
	// The internal subnets host size
	internalHostSize int
	// Static subnet information
	subnets map[subnetKey]*SubnetInfo
	// Allocated addresses in each address space's internal subnet
	addresses map[subnetKey]*bitseq.Handle
	// Datastore
	store    datastore.DataStore
	App      string
	ID       string
	dbIndex  uint64
	dbExists bool
	sync.Mutex
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(ds datastore.DataStore) (*Allocator, error) {
	a := &Allocator{}
	a.subnets = make(map[subnetKey]*SubnetInfo)
	a.addresses = make(map[subnetKey]*bitseq.Handle)
	a.internalHostSize = defaultInternalHostSize
	a.store = ds
	a.App = "ipam"
	a.ID = dsConfigKey

	if a.store == nil {
		return a, nil
	}

	// Register for status changes
	a.watchForChanges()

	// Get the initial subnet configs status from the ds if present.
	kvPair, err := a.store.KVStore().Get(datastore.Key(a.Key()...))
	if err != nil {
		if err != store.ErrKeyNotFound {
			return nil, fmt.Errorf("failed to retrieve the ipam subnet configs from datastore: %v", err)
		}
		return a, nil
	}
	a.subnetConfigFromStore(kvPair)

	// Now retrieve the list of small subnets
	var inserterList []func() error
	a.Lock()
	for k, v := range a.subnets {
		inserterList = append(inserterList,
			func() error {
				subnetList, err := getInternalSubnets(v.Subnet, a.internalHostSize)
				if err != nil {
					return fmt.Errorf("failed to load address bitmask for configured subnet %s because of %s", v.Subnet.String(), err.Error())
				}
				a.insertAddressMasks(k, subnetList)
				return nil
			})
	}
	a.Unlock()

	// Add the bitmasks, data could come from datastore
	for _, f := range inserterList {
		if err := f(); err != nil {
			return nil, err
		}
	}

	return a, nil
}

func (a *Allocator) subnetConfigFromStore(kvPair *store.KVPair) {
	a.Lock()
	if a.dbIndex < kvPair.LastIndex {
		a.subnets = byteArrayToSubnets(kvPair.Value)
		a.dbIndex = kvPair.LastIndex
		a.dbExists = true
	}
	a.Unlock()
}

// Pointer to the configured subnets in each address space
type subnetKey struct {
	addressSpace AddressSpace
	subnet       string
	childSubnet  string
}

func (s *subnetKey) String() string {
	k := fmt.Sprintf("%s/%s", s.addressSpace, s.subnet)
	if s.childSubnet != "" {
		k = fmt.Sprintf("%s/%s", k, s.childSubnet)
	}
	return k
}

func (s *subnetKey) FromString(str string) error {
	if str == "" || !strings.Contains(str, "/") {
		return fmt.Errorf("invalid string form for subnetkey: %s", str)
	}

	p := strings.Split(str, "/")
	if len(p) != 3 && len(p) != 5 {
		return fmt.Errorf("invalid string form for subnetkey: %s", str)
	}
	s.addressSpace = AddressSpace(p[0])
	s.subnet = fmt.Sprintf("%s/%s", p[1], p[2])
	if len(p) == 5 {
		s.childSubnet = fmt.Sprintf("%s/%s", p[1], p[2])
	}

	return nil
}

func (s *subnetKey) canonicalSubnet() *net.IPNet {
	if _, sub, err := net.ParseCIDR(s.subnet); err == nil {
		return sub
	}
	return nil
}

func (s *subnetKey) canonicalChildSubnet() *net.IPNet {
	if _, sub, err := net.ParseCIDR(s.childSubnet); err == nil {
		return sub
	}
	return nil
}

type ipVersion int

const (
	v4 = 4
	v6 = 6
)

/*******************
 * IPAMConf Contract
 ********************/

// AddSubnet adds a subnet for the specified address space
func (a *Allocator) AddSubnet(addrSpace AddressSpace, subnetInfo *SubnetInfo) error {
	// Sanity check
	if addrSpace == "" {
		return ErrInvalidAddressSpace
	}
	if subnetInfo == nil || subnetInfo.Subnet == nil {
		return ErrInvalidSubnet
	}
	// Convert to smaller internal subnets (if needed)
	subnetList, err := getInternalSubnets(subnetInfo.Subnet, a.internalHostSize)
	if err != nil {
		return err
	}
retry:
	if a.contains(addrSpace, subnetInfo) {
		return ErrOverlapSubnet
	}

	// Store the configured subnet and sync to datatstore
	key := subnetKey{addrSpace, subnetInfo.Subnet.String(), ""}
	a.Lock()
	a.subnets[key] = subnetInfo
	a.Unlock()
	err = a.writeToStore()
	if err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return types.InternalErrorf("subnet configuration failed because of %s", err.Error())
		}
		// Update to latest
		if erru := a.readFromStore(); erru != nil {
			// Restore and bail out
			a.Lock()
			delete(a.addresses, key)
			a.Unlock()
			return fmt.Errorf("failed to get updated subnets config from datastore (%v) after (%v)", erru, err)
		}
		goto retry
	}

	// Insert respective bitmasks for this subnet
	a.insertAddressMasks(key, subnetList)

	return nil
}

// Create and insert the internal subnet(s) addresses masks into the address database. Mask data may come from the bitseq datastore.
func (a *Allocator) insertAddressMasks(parentKey subnetKey, internalSubnetList []*net.IPNet) error {
	for _, intSub := range internalSubnetList {
		var err error
		ones, bits := intSub.Mask.Size()
		numAddresses := 1 << uint(bits-ones)
		smallKey := subnetKey{parentKey.addressSpace, parentKey.subnet, intSub.String()}

		// Insert the new address masks. AddressMask content may come from datastore
		a.Lock()
		a.addresses[smallKey], err = bitseq.NewHandle(dsDataKey, a.store, smallKey.String(), uint32(numAddresses))
		a.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// Check subnets size. In case configured subnet is v6 and host size is
// greater than 32 bits, adjust subnet to /96.
func adjustAndCheckSubnetSize(subnet *net.IPNet) (*net.IPNet, error) {
	ones, bits := subnet.Mask.Size()
	if v6 == getAddressVersion(subnet.IP) {
		if ones < minNetSizeV6 {
			return nil, ErrInvalidSubnet
		}
		if ones < minNetSizeV6Eff {
			newMask := net.CIDRMask(minNetSizeV6Eff, bits)
			return &net.IPNet{IP: subnet.IP, Mask: newMask}, nil
		}
	} else {
		if ones < minNetSize {
			return nil, ErrInvalidSubnet
		}
	}
	return subnet, nil
}

// Checks whether the passed subnet is a superset or subset of any of the subset in the db
func (a *Allocator) contains(space AddressSpace, subInfo *SubnetInfo) bool {
	a.Lock()
	defer a.Unlock()
	for k, v := range a.subnets {
		if space == k.addressSpace {
			if subInfo.Subnet.Contains(v.Subnet.IP) ||
				v.Subnet.Contains(subInfo.Subnet.IP) {
				return true
			}
		}
	}
	return false
}

// Splits the passed subnet into N internal subnets with host size equal to internalHostSize.
// If the subnet's host size is equal to or smaller than internalHostSize, there won't be any
// split and the return list will contain only the passed subnet.
func getInternalSubnets(inSubnet *net.IPNet, internalHostSize int) ([]*net.IPNet, error) {
	var subnetList []*net.IPNet

	// Sanity check and size adjustment for v6
	subnet, err := adjustAndCheckSubnetSize(inSubnet)
	if err != nil {
		return subnetList, err
	}

	// Get network/host subnet information
	netBits, bits := subnet.Mask.Size()
	hostBits := bits - netBits

	extraBits := hostBits - internalHostSize
	if extraBits <= 0 {
		subnetList = make([]*net.IPNet, 1)
		subnetList[0] = subnet
	} else {
		// Split in smaller internal subnets
		numIntSubs := 1 << uint(extraBits)
		subnetList = make([]*net.IPNet, numIntSubs)

		// Construct one copy of the internal subnets's mask
		intNetBits := bits - internalHostSize
		intMask := net.CIDRMask(intNetBits, bits)

		// Construct the prefix portion for each internal subnet
		for i := 0; i < numIntSubs; i++ {
			intIP := make([]byte, len(subnet.IP))
			copy(intIP, subnet.IP) // IPv6 is too big, just work on the extra portion
			addIntToIP(intIP, i<<uint(internalHostSize))
			subnetList[i] = &net.IPNet{IP: intIP, Mask: intMask}
		}
	}
	return subnetList, nil
}

// RemoveSubnet removes the subnet from the specified address space
func (a *Allocator) RemoveSubnet(addrSpace AddressSpace, subnet *net.IPNet) error {
	if addrSpace == "" {
		return ErrInvalidAddressSpace
	}
	if subnet == nil {
		return ErrInvalidSubnet
	}
retry:
	// Look for the respective subnet configuration data
	// Remove it along with the internal subnets
	subKey := subnetKey{addrSpace, subnet.String(), ""}
	a.Lock()
	current, ok := a.subnets[subKey]
	a.Unlock()
	if !ok {
		return ErrSubnetNotFound
	}

	// Remove config and sync to datastore
	a.Lock()
	delete(a.subnets, subKey)
	a.Unlock()
	err := a.writeToStore()
	if err != nil {
		if _, ok := err.(types.RetryError); !ok {
			return types.InternalErrorf("subnet removal failed because of %s", err.Error())
		}
		// Update to latest
		if erru := a.readFromStore(); erru != nil {
			// Restore and bail out
			a.Lock()
			a.subnets[subKey] = current
			a.Unlock()
			return fmt.Errorf("failed to get updated subnets config from datastore (%v) after (%v)", erru, err)
		}
		goto retry
	}

	// Get the list of smaller internal subnets
	subnetList, err := getInternalSubnets(subnet, a.internalHostSize)
	if err != nil {
		return err
	}

	for _, s := range subnetList {
		sk := subnetKey{addrSpace, subKey.subnet, s.String()}
		a.Lock()
		if bm, ok := a.addresses[sk]; ok {
			bm.Destroy()
		}
		delete(a.addresses, sk)
		a.Unlock()
	}

	return nil

}

// AddVendorInfo adds vendor specific data
func (a *Allocator) AddVendorInfo([]byte) error {
	// no op for us
	return nil
}

/****************
 * IPAM Contract
 ****************/

// Request allows requesting an IPv4 address from the specified address space
func (a *Allocator) Request(addrSpace AddressSpace, req *AddressRequest) (*AddressResponse, error) {
	return a.request(addrSpace, req, v4)
}

// RequestV6 requesting an IPv6 address from the specified address space
func (a *Allocator) RequestV6(addrSpace AddressSpace, req *AddressRequest) (*AddressResponse, error) {
	return a.request(addrSpace, req, v6)
}

func (a *Allocator) request(addrSpace AddressSpace, req *AddressRequest, version ipVersion) (*AddressResponse, error) {
	// Empty response
	response := &AddressResponse{}

	// Sanity check
	if addrSpace == "" {
		return response, ErrInvalidAddressSpace
	}

	// Validate request
	if err := req.Validate(); err != nil {
		return response, err
	}

	// Check ip version congruence
	if &req.Subnet != nil && version != getAddressVersion(req.Subnet.IP) {
		return response, ErrInvalidRequest
	}

	// Look for an address
	ip, _, err := a.reserveAddress(addrSpace, &req.Subnet, req.Address, version)
	if err == nil {
		// Populate response
		response.Address = ip
		a.Lock()
		response.Subnet = *a.subnets[subnetKey{addrSpace, req.Subnet.String(), ""}]
		a.Unlock()
	}

	return response, err
}

// Release allows releasing the address from the specified address space
func (a *Allocator) Release(addrSpace AddressSpace, address net.IP) {
	if address == nil {
		return
	}
	ver := getAddressVersion(address)
	if ver == v4 {
		address = address.To4()
	}
	for _, subKey := range a.getSubnetList(addrSpace, ver) {
		a.Lock()
		space := a.addresses[subKey]
		a.Unlock()
		sub := subKey.canonicalChildSubnet()
		if sub.Contains(address) {
			// Retrieve correspondent ordinal in the subnet
			ordinal := ipToInt(getHostPortionIP(address, sub))
			// Release it
			for {
				var err error
				if err = space.PushReservation(ordinal/8, ordinal%8, true); err == nil {
					break
				}
				if _, ok := err.(types.RetryError); ok {
					// bitmask must have changed, retry delete
					continue
				}
				log.Warnf("Failed to release address %s because of internal error: %s", address.String(), err.Error())
				return
			}
			return
		}

	}
}

func (a *Allocator) reserveAddress(addrSpace AddressSpace, subnet *net.IPNet, prefAddress net.IP, ver ipVersion) (net.IP, *net.IPNet, error) {
	var keyList []subnetKey

	// Get the list of pointers to the internal subnets
	if subnet != nil {
		// Get the list of smaller internal subnets
		subnetList, err := getInternalSubnets(subnet, a.internalHostSize)
		if err != nil {
			return nil, nil, err
		}
		for _, s := range subnetList {
			keyList = append(keyList, subnetKey{addrSpace, subnet.String(), s.String()})
		}
	} else {
		a.Lock()
		keyList = a.getSubnetList(addrSpace, ver)
		a.Unlock()
	}
	if len(keyList) == 0 {
		return nil, nil, ErrNoAvailableSubnet
	}

	for _, key := range keyList {
		a.Lock()
		bitmask, ok := a.addresses[key]
		a.Unlock()
		if !ok {
			fmt.Printf("\nDid not find a bitmask for subnet key: %s", key.String())
			continue
		}
		address, err := a.getAddress(key.canonicalChildSubnet(), bitmask, prefAddress, ver)
		if err == nil {
			return address, subnet, nil
		}
	}

	return nil, nil, ErrNoAvailableIPs
}

// Get the list of available internal subnets for the specified address space and the desired ip version
func (a *Allocator) getSubnetList(addrSpace AddressSpace, ver ipVersion) []subnetKey {
	var list [1024]subnetKey
	ind := 0
	a.Lock()
	for subKey := range a.addresses {
		s := subKey.canonicalSubnet()
		subVer := getAddressVersion(s.IP)
		if subKey.addressSpace == addrSpace && subVer == ver {
			list[ind] = subKey
			ind++
		}
	}
	a.Unlock()
	return list[0:ind]
}

func (a *Allocator) getAddress(subnet *net.IPNet, bitmask *bitseq.Handle, prefAddress net.IP, ver ipVersion) (net.IP, error) {
	var (
		bytePos, bitPos int
		ordinal         int
		err             error
	)

	// Look for free IP, skip .0 and .255, they will be automatically reserved
	for {
		if bitmask.Unselected() <= 0 {
			return nil, ErrNoAvailableIPs
		}
		if prefAddress == nil {
			bytePos, bitPos, err = bitmask.GetFirstAvailable()
		} else {
			ordinal = ipToInt(getHostPortionIP(prefAddress, subnet))
			bytePos, bitPos, err = bitmask.CheckIfAvailable(ordinal)
		}
		if err != nil {
			return nil, ErrNoAvailableIPs
		}

		// Lock it
		if err = bitmask.PushReservation(bytePos, bitPos, false); err != nil {
			if _, ok := err.(types.RetryError); !ok {
				return nil, fmt.Errorf("internal failure while reserving the address: %s", err.Error())
			}
			continue
		}

		// Build IP ordinal
		ordinal = bitPos + bytePos*8

		// For v4, let reservation of .0 and .255 happen automatically
		if ver == v4 && !isValidIP(ordinal) {
			continue
		}
		break
	}

	// Convert IP ordinal for this subnet into IP address
	return generateAddress(ordinal, subnet), nil
}

// DumpDatabase dumps the internal info
func (a *Allocator) DumpDatabase() {
	a.Lock()
	defer a.Unlock()
	for k, config := range a.subnets {
		fmt.Printf("\n\n%s:", config.Subnet.String())
		subnetList, _ := getInternalSubnets(config.Subnet, a.internalHostSize)
		for _, s := range subnetList {
			internKey := subnetKey{k.addressSpace, config.Subnet.String(), s.String()}
			bm := a.addresses[internKey]
			fmt.Printf("\n\t%s: %s\n\t%d", internKey.childSubnet, bm, bm.Unselected())
		}
	}
}

// It generates the ip address in the passed subnet specified by
// the passed host address ordinal
func generateAddress(ordinal int, network *net.IPNet) net.IP {
	var address [16]byte

	// Get network portion of IP
	if network.IP.To4() != nil {
		copy(address[:], network.IP.To4())
	} else {
		copy(address[:], network.IP)
	}

	end := len(network.Mask)
	addIntToIP(address[:end], ordinal)

	return net.IP(address[:end])
}

func getAddressVersion(ip net.IP) ipVersion {
	if ip.To4() == nil {
		return v6
	}
	return v4
}

// .0 and .255 will return false
func isValidIP(i int) bool {
	lastByte := i & 0xff
	return lastByte != 0xff && lastByte != 0
}

// Adds the ordinal IP to the current array
// 192.168.0.0 + 53 => 192.168.53
func addIntToIP(array []byte, ordinal int) {
	for i := len(array) - 1; i >= 0; i-- {
		array[i] |= (byte)(ordinal & 0xff)
		ordinal >>= 8
	}
}

// Convert an ordinal to the respective IP address
func ipToInt(ip []byte) int {
	value := 0
	for i := 0; i < len(ip); i++ {
		j := len(ip) - 1 - i
		value += int(ip[i]) << uint(j*8)
	}
	return value
}

// Given an address and subnet, returns the host portion address
func getHostPortionIP(address net.IP, subnet *net.IPNet) net.IP {
	hostPortion := make([]byte, len(address))
	for i := 0; i < len(subnet.Mask); i++ {
		hostPortion[i] = address[i] &^ subnet.Mask[i]
	}
	return hostPortion
}

func printLine(head *bitseq.Sequence) {
	fmt.Println()
	for head != nil {
		fmt.Printf("-")
		head = head.Next
	}
}
