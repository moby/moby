// Package identity provides functionality for generating and managing
// identifiers within a swarm. This includes entity identification, such as for
// Services, Tasks and Networks but also cryptographically-secure Node identities.
//
// Random Identifiers
//
// Identifiers provided by this package are cryptographically-strong, random
// 128 bit numbers encoded in Base36. This method is preferred over UUID4 since
// it requires less storage and leverages the full 128 bits of entropy.
//
// Generating an identifier is simple. Simply call the `NewID` function:
//
// 	id := NewID()
//
// If an error occurs while generating the ID, it will panic.
package identity
