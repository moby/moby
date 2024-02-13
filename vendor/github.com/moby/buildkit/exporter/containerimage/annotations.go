package containerimage

import (
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	"github.com/containerd/containerd/platforms"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
)

type Annotations struct {
	Index              map[string]string
	IndexDescriptor    map[string]string
	Manifest           map[string]string
	ManifestDescriptor map[string]string
}

// AnnotationsGroup is a map of annotations keyed by the reference key
type AnnotationsGroup map[string]*Annotations

func ParseAnnotations(data map[string][]byte) (AnnotationsGroup, map[string][]byte, error) {
	ag := make(AnnotationsGroup)
	rest := make(map[string][]byte)

	for k, v := range data {
		a, ok, err := exptypes.ParseAnnotationKey(k)
		if !ok {
			rest[k] = v
			continue
		}
		if err != nil {
			return nil, nil, err
		}

		p := a.PlatformString()

		if ag[p] == nil {
			ag[p] = &Annotations{
				IndexDescriptor:    make(map[string]string),
				Index:              make(map[string]string),
				Manifest:           make(map[string]string),
				ManifestDescriptor: make(map[string]string),
			}
		}

		switch a.Type {
		case exptypes.AnnotationIndex:
			ag[p].Index[a.Key] = string(v)
		case exptypes.AnnotationIndexDescriptor:
			ag[p].IndexDescriptor[a.Key] = string(v)
		case exptypes.AnnotationManifest:
			ag[p].Manifest[a.Key] = string(v)
		case exptypes.AnnotationManifestDescriptor:
			ag[p].ManifestDescriptor[a.Key] = string(v)
		default:
			return nil, nil, errors.Errorf("unrecognized annotation type %s", a.Type)
		}
	}
	return ag, rest, nil
}

func (ag AnnotationsGroup) Platform(p *ocispecs.Platform) *Annotations {
	res := &Annotations{
		IndexDescriptor:    make(map[string]string),
		Index:              make(map[string]string),
		Manifest:           make(map[string]string),
		ManifestDescriptor: make(map[string]string),
	}

	ps := []string{""}
	if p != nil {
		ps = append(ps, platforms.Format(*p))
	}

	for _, a := range ag {
		for k, v := range a.Index {
			res.Index[k] = v
		}
		for k, v := range a.IndexDescriptor {
			res.IndexDescriptor[k] = v
		}
	}
	for _, pk := range ps {
		if _, ok := ag[pk]; !ok {
			continue
		}

		for k, v := range ag[pk].Manifest {
			res.Manifest[k] = v
		}
		for k, v := range ag[pk].ManifestDescriptor {
			res.ManifestDescriptor[k] = v
		}
	}
	return res
}

func (ag AnnotationsGroup) Merge(other AnnotationsGroup) AnnotationsGroup {
	if other == nil {
		return ag
	}
	if ag == nil {
		ag = make(AnnotationsGroup)
	}

	for k, v := range other {
		ag[k] = ag[k].merge(v)
	}
	return ag
}

func (a *Annotations) merge(other *Annotations) *Annotations {
	if other == nil {
		return a
	}
	if a == nil {
		a = &Annotations{
			IndexDescriptor:    make(map[string]string),
			Index:              make(map[string]string),
			Manifest:           make(map[string]string),
			ManifestDescriptor: make(map[string]string),
		}
	}

	for k, v := range other.Index {
		a.Index[k] = v
	}
	for k, v := range other.IndexDescriptor {
		a.IndexDescriptor[k] = v
	}
	for k, v := range other.Manifest {
		a.Manifest[k] = v
	}
	for k, v := range other.ManifestDescriptor {
		a.ManifestDescriptor[k] = v
	}

	return a
}
