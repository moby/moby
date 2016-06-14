// Package identity provides functionality for generating and manager
// identifiers within swarm. This includes entity identification, such as that
// of Service, Task and Network but also cryptographically-secure Node identity.
//
// Random Identifiers
//
// Identifiers provided by this package are cryptographically-strong, random
// 128 bit numbers encoded in Base36. This method is preferred over UUID4 since
// it requires less storage and leverages the full 128 bits of entropy.
//
// Generating an identifier is simple. Simply call the `NewID` function, check
// the error and proceed:
//
// 	id, err := NewID()
// 	if err != nil { /* ... handle it, please ... */ }
//
package identity
