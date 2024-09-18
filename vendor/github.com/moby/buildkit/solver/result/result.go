package result

import (
	"maps"
	"sync"

	"github.com/pkg/errors"
)

type Result[T comparable] struct {
	mu           sync.Mutex
	Ref          T
	Refs         map[string]T
	Metadata     map[string][]byte
	Attestations map[string][]Attestation[T]
}

func (r *Result[T]) Clone() *Result[T] {
	return &Result[T]{
		Ref:          r.Ref,
		Refs:         maps.Clone(r.Refs),
		Metadata:     maps.Clone(r.Metadata),
		Attestations: maps.Clone(r.Attestations),
	}
}

func (r *Result[T]) AddMeta(k string, v []byte) {
	r.mu.Lock()
	if r.Metadata == nil {
		r.Metadata = map[string][]byte{}
	}
	r.Metadata[k] = v
	r.mu.Unlock()
}

func (r *Result[T]) AddRef(k string, ref T) {
	r.mu.Lock()
	if r.Refs == nil {
		r.Refs = map[string]T{}
	}
	r.Refs[k] = ref
	r.mu.Unlock()
}

func (r *Result[T]) AddAttestation(k string, v Attestation[T]) {
	r.mu.Lock()
	if r.Attestations == nil {
		r.Attestations = map[string][]Attestation[T]{}
	}
	r.Attestations[k] = append(r.Attestations[k], v)
	r.mu.Unlock()
}

func (r *Result[T]) SetRef(ref T) {
	r.Ref = ref
}

func (r *Result[T]) SingleRef() (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var zero T
	if r.Refs != nil && r.Ref == zero {
		var t T
		return t, errors.Errorf("invalid map result")
	}
	return r.Ref, nil
}

func (r *Result[T]) FindRef(key string) (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Refs != nil {
		if ref, ok := r.Refs[key]; ok {
			return ref, true
		}
		if len(r.Refs) == 1 {
			for _, ref := range r.Refs {
				return ref, true
			}
		}
		var t T
		return t, false
	}
	return r.Ref, true
}

func (r *Result[T]) EachRef(fn func(T) error) (err error) {
	var zero T
	if r.Ref != zero {
		err = fn(r.Ref)
	}
	for _, r := range r.Refs {
		if r != zero {
			if err1 := fn(r); err1 != nil && err == nil {
				err = err1
			}
		}
	}
	for _, as := range r.Attestations {
		for _, a := range as {
			if a.Ref != zero {
				if err1 := fn(a.Ref); err1 != nil && err == nil {
					err = err1
				}
			}
		}
	}
	return err
}

// EachRef iterates over references in both a and b.
// a and b are assumed to be of the same size and map their references
// to the same set of keys
func EachRef[U comparable, V comparable](a *Result[U], b *Result[V], fn func(U, V) error) (err error) {
	var (
		zeroU U
		zeroV V
	)
	if a.Ref != zeroU && b.Ref != zeroV {
		err = fn(a.Ref, b.Ref)
	}
	for k, r := range a.Refs {
		r2, ok := b.Refs[k]
		if !ok {
			continue
		}
		if r != zeroU && r2 != zeroV {
			if err1 := fn(r, r2); err1 != nil && err == nil {
				err = err1
			}
		}
	}
	for k, atts := range a.Attestations {
		atts2, ok := b.Attestations[k]
		if !ok {
			continue
		}
		for i, att := range atts {
			if i >= len(atts2) {
				break
			}
			att2 := atts2[i]
			if att.Ref != zeroU && att2.Ref != zeroV {
				if err1 := fn(att.Ref, att2.Ref); err1 != nil && err == nil {
					err = err1
				}
			}
		}
	}
	return err
}

// ConvertResult transforms a Result[U] into a Result[V], using a transfomer
// function that converts a U to a V. Zero values of type U are converted to
// zero values of type V directly, without passing through the transformer
// function.
func ConvertResult[U comparable, V comparable](r *Result[U], fn func(U) (V, error)) (*Result[V], error) {
	var zero U

	r2 := &Result[V]{}
	var err error

	if r.Ref != zero {
		r2.Ref, err = fn(r.Ref)
		if err != nil {
			return nil, err
		}
	}

	if r.Refs != nil {
		r2.Refs = map[string]V{}
	}
	for k, r := range r.Refs {
		if r == zero {
			var zero V
			r2.Refs[k] = zero
			continue
		}
		r2.Refs[k], err = fn(r)
		if err != nil {
			return nil, err
		}
	}

	if r.Attestations != nil {
		r2.Attestations = map[string][]Attestation[V]{}
	}
	for k, as := range r.Attestations {
		for _, a := range as {
			a2, err := ConvertAttestation(&a, fn)
			if err != nil {
				return nil, err
			}
			r2.Attestations[k] = append(r2.Attestations[k], *a2)
		}
	}

	r2.Metadata = r.Metadata

	return r2, nil
}
