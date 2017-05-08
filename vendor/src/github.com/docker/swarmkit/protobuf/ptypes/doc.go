// Package ptypes is a copy of the golang/protobuf/ptypes that we'll need to
// use with our regenerated ptypes until google gets their act together and
// makes their "Well Known Types" actually usable by other parties.
//
// It is more likely that this issue will be resolved by gogo.
//
// Note that this is not a vendoring of the package. We have to change the
// types to match the generated types.
package ptypes
