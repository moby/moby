package ipam

import (
	"fmt"
	"net"
	"sync"

	"github.com/docker/libnetwork/bitseq"
	"github.com/docker/libnetwork/datastore"
)

const (
	// The biggest configurable host subnets
	minNetSize   = 8
	minNetSizeV6 = 64
	// The effective network size for v6
	minNetSizeV6Eff = 96
	// The size of the host subnet used internally, it's the most granular sequence addresses
	defaultInternalHostSize = 16
)

// Allocator provides per address space ipv4/ipv6 book keeping
type Allocator struct {
	// The internal subnets host size
	internalHostSize int
	// Static subnet information
	subnetsInfo map[subnetKey]*SubnetInfo
	// Allocated addresses in each address space's internal subnet
	addresses map[subnetKey]*bitmask
	// Datastore
	store datastore.DataStore
	sync.Mutex
}

// NewAllocator returns an instance of libnetwork ipam
func NewAllocator(ds datastore.DataStore) *Allocator {
	a := &Allocator{}
	a.subnetsInfo = make(map[subnetKey]*SubnetInfo)
	a.addresses = make(map[subnetKey]*bitmask)
	a.internalHostSize = defaultInternalHostSize
	a.store = ds
	return a
}

// Pointer to the configured subnets in each address space
type subnetKey struct {
	addressSpace AddressSpace
	subnet       string
}

func (s *subnetKey) String() string {
	return fmt.Sprintf("%s/%s", s.addressSpace, s.subnet)
}

// The structs containing the address allocation bitmask for the internal subnet.
// The bitmask is stored a run-length encoded seq.Sequence of 4 bytes blcoks.
type bitmask struct {
	subnet        *net.IPNet
	addressMask   *bitseq.Handle
	freeAddresses int
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
	if a.contains(addrSpace, subnetInfo) {
		return ErrOverlapSubnet
	}

	// Convert to smaller internal subnets (if needed)
	subnetList, err := getInternalSubnets(subnetInfo.Subnet, a.internalHostSize)
	if err != nil {
		return err
	}

	// Store the configured subnet information
	key := subnetKey{addrSpace, subnetInfo.Subnet.String()}
	a.Lock()
	a.subnetsInfo[key] = subnetInfo
	a.Unlock()

	// Create and insert the internal subnet(s) addresses masks into the address database
	for _, sub := range subnetList {
		ones, bits := sub.Mask.Size()
		numAddresses := 1 << uint(bits-ones)
		smallKey := subnetKey{addrSpace, sub.String()}

		// Add the new address masks
		a.Lock()
		a.addresses[smallKey] = &bitmask{
			subnet:        sub,
			addressMask:   bitseq.NewHandle("ipam", a.store, smallKey.String(), uint32(numAddresses)),
			freeAddresses: numAddresses,
		}
		a.Unlock()
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
	for k, v := range a.subnetsInfo {
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

	// Look for the respective subnet configuration data
	// Remove it along with the internal subnets
	subKey := subnetKey{addrSpace, subnet.String()}
	a.Lock()
	_, ok := a.subnetsInfo[subKey]
	a.Unlock()
	if !ok {
		return ErrSubnetNotFound
	}

	// Get the list of smaller internal subnets
	subnetList, err := getInternalSubnets(subnet, a.internalHostSize)
	if err != nil {
		return err
	}

	for _, s := range subnetList {
		a.Lock()
		delete(a.addresses, subnetKey{addrSpace, s.String()})
		a.Unlock()
	}

	a.Lock()
	delete(a.subnetsInfo, subKey)
	a.Unlock()

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
		response.Subnet = *a.subnetsInfo[subnetKey{addrSpace, req.Subnet.String()}]
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
		sub := a.addresses[subKey].subnet
		if sub.Contains(address) {
			// Retrieve correspondent ordinal in the subnet
			space := a.addresses[subnetKey{addrSpace, sub.String()}]
			ordinal := ipToInt(getHostPortionIP(address, space.subnet))
			// Release it
			space.addressMask.PushReservation(ordinal/8, ordinal%8, true)
			space.freeAddresses++
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
			keyList = append(keyList, subnetKey{addrSpace, s.String()})
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
		smallSubnet := a.addresses[key]
		a.Unlock()
		address, err := a.getAddress(smallSubnet, prefAddress, ver)
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
		_, s, _ := net.ParseCIDR(subKey.subnet)
		subVer := getAddressVersion(s.IP)
		if subKey.addressSpace == addrSpace && subVer == ver {
			list[ind] = subKey
			ind++
		}
	}
	a.Unlock()
	return list[0:ind]
}

func (a *Allocator) getAddress(smallSubnet *bitmask, prefAddress net.IP, ver ipVersion) (net.IP, error) {
	var (
		bytePos, bitPos int
		err             error
	)
	// Look for free IP, skip .0 and .255, they will be automatically reserved
again:
	if smallSubnet.freeAddresses <= 0 {
		return nil, ErrNoAvailableIPs
	}
	if prefAddress == nil {
		bytePos, bitPos, err = smallSubnet.addressMask.GetFirstAvailable()
	} else {
		ordinal := ipToInt(getHostPortionIP(prefAddress, smallSubnet.subnet))
		bytePos, bitPos, err = smallSubnet.addressMask.CheckIfAvailable(ordinal)
	}
	if err != nil {
		return nil, ErrNoAvailableIPs
	}

	// Lock it
	smallSubnet.addressMask.PushReservation(bytePos, bitPos, false)
	smallSubnet.freeAddresses--

	// Build IP ordinal
	ordinal := bitPos + bytePos*8

	// For v4, let reservation of .0 and .255 happen automatically
	if ver == v4 && !isValidIP(ordinal) {
		goto again
	}

	// Convert IP ordinal for this subnet into IP address
	return generateAddress(ordinal, smallSubnet.subnet), nil
}

// DumpDatabase dumps the internal info
func (a *Allocator) DumpDatabase() {
	a.Lock()
	defer a.Unlock()
	for k, config := range a.subnetsInfo {
		fmt.Printf("\n\n%s:", config.Subnet.String())
		subnetList, _ := getInternalSubnets(config.Subnet, a.internalHostSize)
		for _, s := range subnetList {
			internKey := subnetKey{k.addressSpace, s.String()}
			bm := a.addresses[internKey]
			fmt.Printf("\n\t%s: %s\n\t%d", bm.subnet, bm.addressMask, bm.freeAddresses)
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
