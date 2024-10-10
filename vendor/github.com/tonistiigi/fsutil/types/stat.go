package types

import "os"

func (s *Stat) IsDir() bool {
	return os.FileMode(s.Mode).IsDir()
}

func (s *Stat) Marshal() ([]byte, error) {
	return s.MarshalVTStrict()
}

func (s *Stat) Unmarshal(dAtA []byte) error {
	return s.UnmarshalVT(dAtA)
}

func (s *Stat) Clone() *Stat {
	return s.CloneVT()
}
