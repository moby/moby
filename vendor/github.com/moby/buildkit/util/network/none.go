package network

import (
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func NewNoneProvider() Provider {
	return &none{}
}

type none struct {
}

func (h *none) New() (Namespace, error) {
	return &noneNS{}, nil
}

type noneNS struct {
}

func (h *noneNS) Set(s *specs.Spec) {
}

func (h *noneNS) Close() error {
	return nil
}
