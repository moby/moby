package llb

import (
	"io"
	"io/ioutil"

	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
)

// Definition is the LLB definition structure with per-vertex metadata entries
// Corresponds to the Definition structure defined in solver/pb.Definition.
type Definition struct {
	Def      [][]byte
	Metadata map[digest.Digest]OpMetadata
}

func (def *Definition) ToPB() *pb.Definition {
	md := make(map[digest.Digest]OpMetadata)
	for k, v := range def.Metadata {
		md[k] = v
	}
	return &pb.Definition{
		Def:      def.Def,
		Metadata: md,
	}
}

func (def *Definition) FromPB(x *pb.Definition) {
	def.Def = x.Def
	def.Metadata = make(map[digest.Digest]OpMetadata)
	for k, v := range x.Metadata {
		def.Metadata[k] = v
	}
}

type OpMetadata = pb.OpMetadata

func WriteTo(def *Definition, w io.Writer) error {
	b, err := def.ToPB().Marshal()
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func ReadFrom(r io.Reader) (*Definition, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var pbDef pb.Definition
	if err := pbDef.Unmarshal(b); err != nil {
		return nil, err
	}
	var def Definition
	def.FromPB(&pbDef)
	return &def, nil
}
