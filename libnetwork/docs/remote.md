Remote Driver
=============

Remote driver is a special built-in driver. This driver in itself doesn't provide any networking functionality. But it depends on actual remote drivers aka `Dynamic Drivers` to provide the required networking between the containers. The dynamic drivers (such as : Weave, OVS, OVN, ACI, Calico and external networking plugins) registers itself with the Build-In `Remote` Driver.

## LibNetwork Integration

When LibNetwork creates an instance of the Built-in `Remote` Driver via the `New()` function, it provides a `DriverCallback` which implements the `RegisterDynamicDriver()` to let the Built-in Remote Driver to register any of the `Dynamic` Drivers/Plugins with LibNetwork's `NetworkController`

Refer to [Remote Driver Test](https://github.com/docker/libnetwork/blob/drivers/remote/driver_test.go) which provides an example of how the Built-In Remote driver can register any Dynamic driver with LibNetwork.

This design ensures that the implementation details of Dynamic Driver Registration mechanism is completely owned by the inbuilt-Remote driver and it also doesnt expose any of the driver layer to the North of LibNetwork (none of the LibNetwork client APIs are impacted).

When the inbuilt `Remote` driver detects a `Dynamic` Driver it will have to call the `registerRemoteDriver` method. This Method will take care of creating a new `Remote` Driver instance and associate it with the new `NetworkType` which is handled by the `Dynamic` Driver.

## Implementation

The actual implementation of how the Inbuilt Remote Driver registers with the Dynamic Driver is Work-In-Progress. But, the Design Goal is to Honor the bigger goals of LibNetwork by keeping it Highly modular and make sure that LibNetwork is fully composable in nature. 

## Usage

The In-Built Remote Driver follows all the rules of any other In-Built Driver and has exactly the same Driver APIs exposed. LibNetwork will also support driver-specific `options` and User-supplied `Labels` which the Dynamic Drivers can make use for its operations.
