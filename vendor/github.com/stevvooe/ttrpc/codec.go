package ttrpc

import (
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
)

type codec struct{}

func (c codec) Marshal(msg interface{}) ([]byte, error) {
	switch v := msg.(type) {
	case proto.Message:
		return proto.Marshal(v)
	default:
		return nil, errors.Errorf("ttrpc: cannot marshal unknown type: %T", msg)
	}
}

func (c codec) Unmarshal(p []byte, msg interface{}) error {
	switch v := msg.(type) {
	case proto.Message:
		return proto.Unmarshal(p, v)
	default:
		return errors.Errorf("ttrpc: cannot unmarshal into unknown type: %T", msg)
	}
}
