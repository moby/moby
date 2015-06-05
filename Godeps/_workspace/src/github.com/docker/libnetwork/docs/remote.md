Remote Drivers
==============

The remote driver package provides the integration point for dynamically-registered drivers.

## LibNetwork Integration

When LibNetwork initialises the `Remote` package with the `Init()` function, it passes a `DriverCallback` as a parameter, which implements the `RegisterDriver()`. The Remote Driver package can use this interface to register any of the `Dynamic` Drivers/Plugins with LibNetwork's `NetworkController`.

This design ensures that the implementation details (TBD) of Dynamic Driver Registration mechanism is completely owned by the inbuilt-Remote driver, and it doesn't expose any of the driver layer to the North of LibNetwork (none of the LibNetwork client APIs are impacted).

## Implementation

The actual implementation of how the Inbuilt Remote Driver registers with the Dynamic Driver is Work-In-Progress. But, the Design Goal is to Honor the bigger goals of LibNetwork by keeping it Highly modular and make sure that LibNetwork is fully composable in nature. 

## Usage

The In-Built Remote Driver follows all the rules of any other In-Built Driver and has exactly the same Driver APIs exposed. LibNetwork will also support driver-specific `options` and User-supplied `Labels` which the Dynamic Drivers can make use for its operations.
