package driverapi

// EndpointDriver is a driver managing endpoints.
type EndpointDriver interface {
	Driver

	// CreateEndpoint invokes the driver method to create an endpoint
	// passing the network id, endpoint id, endpoint information and driver
	// specific config. The endpoint information can be either consumed by
	// the driver or populated by the driver.
	CreateEndpoint(nid, eid string, ifInfo InterfaceInfo, options map[string]interface{}) error

	// DeleteEndpoint invokes the driver method to delete an endpoint passing
	// the network id and endpoint id.
	DeleteEndpoint(nid, eid string) error

	// EndpointOperInfo retrieves from the driver the operational data related
	// to the specified endpoint.
	EndpointOperInfo(nid, eid string) (map[string]interface{}, error)

	// Join method is invoked when a Sandbox is attached to an endpoint.
	Join(nid, eid string, sboxKey string, jinfo JoinInfo, options map[string]interface{}) error

	// Leave method is invoked when a Sandbox detaches from an endpoint.
	Leave(nid, eid string) error
}
