# Ipam Driver

During the Network and Endpoints lifecyle the CNM model controls the ip address assignment for network and endpoint interfaces via the IPAM driver(s).
Libnetwork has a default, built-in ipam driver and allows third party ipam drivers to be dinamically plugged. On network creation user can specify which ipam driver libnetwork needs to use for the network's ip address management. Current document explains the APIs with which IPAM driver needs to comply and the corresponding HTTPS request/response body relevant for remote drivers.


## Remote Ipam driver

On the same line of remote network driver registration (see [remote.md] for more details), Libnetwork initializes the `ipams.remote` package with the `Init()` function, it passes a `ipamapi.Callback` as a parameter, which implements `RegisterOpamDriver()`. The remote driver package uses this interface to register remote drivers with Libnetwork's `NetworkController`, by supplying it in a `plugins.Handle` callback.  The remote drivers register and communicate with libnetwork via the Docker plugin package. The `ipams.remote` provides the proxy for the remote driver processes.


## Protocol

Communication protocol is same as remote network driver.

## Handshake

During driver registration, libnetwork will query the remote driver about the default local and global address spaces strings.
More detailed information can be found in the respective section in this document.

## Datastore Requirements

It is remote driver responsibility to manage its database. 

## Ipam Contract

The ipam driver (internal or remote) has to comply with the contract specified in `ipamapi.contract.go`:


	// Ipam represents the interface the IPAM service plugins must implement
	// in order to allow injection/modification of IPAM database.
	type Ipam interface {
		// GetDefaultAddressSpaces returns the default local and global address spaces for this ipam
		GetDefaultAddressSpaces() (string, string, error)
		// RequestPool returns an address pool along with its unique id. Address space is a mandatory field
		// which denotes a set of non-overlapping pools. pool describes the pool of addresses in CIDR notation.
		// subpool indicates a smaller range of addresses from the pool, for now it is specified in CIDR notation.
		// Both pool and subpool are non mandatory fields. When they are not specified, Ipam driver may choose to
		// return a self chosen pool for this request. In such case the v6 flag needs to be set appropriately so
		// that the driver would return the expected ip version pool.
		RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error)
		// ReleasePool releases the address pool identified by the passed id
		ReleasePool(poolID string) error
		// Request address from the specified pool ID. Input options or preferred IP can be passed.
		RequestAddress(string, net.IP, map[string]string) (*net.IPNet, map[string]string, error)
		// Release the address from the specified pool ID
		ReleaseAddress(string, net.IP) error
	}

The following sections explain the each of the above API's semantic, when they are called during network/endpoint lyfecyle and the correspondent payload for remote driver HTTP request/responses.


## Ipam Configuration and flow

Libnetwork user can provide ipam related configuration when creating a network, via the `NetworkOptionIpam` setter function. 

`func NetworkOptionIpam(ipamDriver string, addrSpace string, ipV4 []*IpamConf, ipV6 []*IpamConf) NetworkOption`

Caller has to provide the ipam driver name and may provide the address space and a list of `IpamConf` structures for ipv4 and a list for ipv6. The ipam driver name is the only mandatory field. If not provided, network creation will fail.

In the list of configurations, each element has the following form:


	// IpamConf contains all the ipam related configurations for a network
	type IpamConf struct {
		// The master address pool for containers and network interfaces
		PreferredPool string
		// A subset of the master pool. If specified,
		// this becomes the container pool
		SubPool string
		// Input options for IPAM Driver (optional)
		Options map[string]string
		// Preferred Network Gateway address (optional)
		Gateway string
		// Auxiliary addresses for network driver. Must be within the master pool.
		// libnetwork will reserve them if they fall into the container pool
		AuxAddresses map[string]string
	}


On network creation, libnetwork will iterate the list and perform the following requests to ipam driver:
1) Request the address pool and pass the options along via `RequestPool()`.
2) Request the network gateway address if specified. Otherwise request any address from the pool to be used as network gateway. This is done via `RequestAddress()`.
3) Request each of the specified auxiliary addresses via `RequestAddress()`.

If the list of ipv4 configurations is empty, libnetwork will automatically add one empty `IpamConf` structure. This will cause libnetwork to request ipam driver an ipv4 address pool of the driver choice on the configured address space, if specified, or on the ipam driver default address space otherwise. If the ipam driver is not able to provide an address pool, network creation will fail.
If the list of ipv6 configurations is empty, libnetwork will not take any action.
The data retrieved from the ipam driver during the execution of point 1) to 3) will be stored in the network structure as a list of `IpamInfo` structures for IPv6 and for IPv6.

On endpoint creation, libnetwork will iterate over the list of configs and perform the following operation:
1) Request an IPv4 address from the ipv4 pool and assign it to the endpoint interface ipv4 address. If successful, stop iterating.
2) Request an IPv6 address from the ipv6 pool (if exists) and assign it to the endpoint interface ipv6 address. If successful, stop iterating.

Endpoint creation will fail if any of the above operation does not succeed

On endpoint deletion, libnetwork will perform the following operations:
1) Release the endpoint interface IPv4 address
2) Release the endpoint interface IPv6 address if present

On Network deletion libnetwork will iterate the list of `IpamData` structures and perform the following requests to ipam driver:
1) Release the network gateway address via `ReleaseAddress()`
2) Release each of the auxiliary addresses via `ReleaseAddress()`
3) Release the pool via `ReleasePool()`

### GetDefaultAddressSpaces

GetDefaultAddressSpaces returns the default local and global address space names for this ipam. An address space is a set of non-overlapping address pools isolated from other address spaces' pools. In other words, same pool can exist on N different address spaces. An address space naturally maps to a tenant name. 
In libnetwork the meaning associated to `local` or `global` address space is that a local address space doesn't need to get synchronized across the
cluster where as the global address spaces does. Unless specified otherwise in the ipam configuration, libnetwork will request address pools from the default local or default global address space based on the scope of the network being created. For example, if not specified otherwise in the configuration, libnetwork will request address pool from the default local address space for a bridge network, whereas from the default global address space for an overlay network.

During registration, the remote driver will receive a POST message to the URL `/IpamDriver.GetDefaultAddressSpaces` with no payload. The driver's response should have the form:


	{
		"LocalDefaultAddressSpace": string
		"GlobalDefaultAddressSpace": string
	}



### RequestPool

This API is for registering a address pool with the ipam driver. Multiple identical calls must return the same result.
it is ipam driver responsibility to keep a reference count for the pool.

`RequestPool(addressSpace, pool, subPool string, options map[string]string, v6 bool) (string, *net.IPNet, map[string]string, error)`


For this API, the remote driver will receive a POST message to the URL `/IpamDriver.RequestPool` with the following payload:

    {
		"AddressSpace": string
		"Pool":         string 
		"SubPool":      string 
		"Options":      map[string]string 
		"V6":           bool 
    }

    
Where:
    * `AddressSpace` the ip address space
    * `Pool` The IPv4 or IPv6 address pool in CIDR format
    * `SubPool` An optional subset of the address pool, an ip range in CIDR format
    * `Options` A map of ipam driver specific options
    * `V6` Whether a ipam self chosen pool should be IPv6
    
Address space is the only mandatory field. If no `Pool` is specified ipam driver may return a self chosen address pool. In such case, `V6` flag must be set if caller wants an ipam chosen IPv6 pool. A request with empty `Pool` and non-empty `SubPool` should be rejected as invalid.
If a Pool is not specified IPAM will allocate one of the default pools. When the Pool is not specified V6 flag should be set if the network needs IPv6 addresses to be allocated.

A successful response is in the form:


	{
		"PoolID": string
		"Pool":   string
		"Data":   map[string]string
	}


Where:
* `PoolID` is an identifier for this pool. Same pools must have same pool id.
* `Pool` is the pool in CIDR format
* `Data` is the ipam driver supplied metadata for this pool


### ReleasePool

This API is for releasing a previously registered address pool.

`ReleasePool(poolID string) error`

For this API, the remote driver will receive a POST message to the URL `/IpamDriver.ReleasePool` with the following payload:

	{
		"PoolID": string
	}

Where:
* `PoolID` is the pool identifier

A successful response is empty:

    {}
    
### RequestAddress

This API is for reserving an ip address.

`RequestAddress(string, net.IP, map[string]string) (*net.IPNet, map[string]string, error)`

For this API, the remote driver will receive a POST message to the URL `/IpamDriver.RequestAddress` with the following payload:

    {
		"PoolID":  string
		"Address": string
		"Options": map[string]string
    }
    
Where:
* `PoolID` is the pool identifier
* `Address` is the preferred address in regular IP form (A.B.C.D). If empty, the ipam driver chooses any available address on the pool
* `Options` are ipam driver specific options


A successful response is in the form:


	{
		Address: string
		Data:    map[string]string
	}


Where:
* `Address` is the allocated address in CIDR format (A.B.C.D/MM)
* `Data` is some ipam driver specific metadata

### ReleaseAddress

This API is for releasing an IP address.

For this API, the remote driver will receive a POST message to the URL `/IpamDriver.RleaseAddress` with the following payload:

    {
		"PoolID": string
		"Address: string
    }
    
Where:
* `PoolID` is the pool identifier
* `Address` is the ip address to release

