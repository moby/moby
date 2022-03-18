package control

import controlapi "github.com/moby/buildkit/api/services/control"

var emptyLogVertexSize int

func init() {
	emptyLogVertex := controlapi.VertexLog{}
	emptyLogVertexSize = emptyLogVertex.Size()
}
