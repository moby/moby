package pb

import proto "google.golang.org/protobuf/proto"

func (m *Definition) IsNil() bool {
	return m == nil || m.Metadata == nil
}

func (m *Definition) Marshal() ([]byte, error) {
	return m.MarshalVT()
}

func (m *Definition) Unmarshal(dAtA []byte) error {
	return m.UnmarshalVT(dAtA)
}

func (m *Op) Marshal() ([]byte, error) {
	return proto.MarshalOptions{Deterministic: true}.Marshal(m)
}

func (m *Op) Unmarshal(dAtA []byte) error {
	return m.UnmarshalVT(dAtA)
}
