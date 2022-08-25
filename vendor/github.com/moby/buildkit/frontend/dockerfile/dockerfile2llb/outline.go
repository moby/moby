package dockerfile2llb

import (
	"sort"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/subrequests/outline"
	pb "github.com/moby/buildkit/solver/pb"
)

type outlineCapture struct {
	allArgs  map[string]argInfo
	usedArgs map[string]struct{}
	secrets  map[string]secretInfo
	ssh      map[string]sshInfo
}

type argInfo struct {
	value      string
	definition instructions.KeyValuePairOptional
	deps       map[string]struct{}
	location   []parser.Range
}

type secretInfo struct {
	required bool
	location []parser.Range
}

type sshInfo struct {
	required bool
	location []parser.Range
}

func newOutlineCapture() outlineCapture {
	return outlineCapture{
		allArgs:  map[string]argInfo{},
		usedArgs: map[string]struct{}{},
		secrets:  map[string]secretInfo{},
		ssh:      map[string]sshInfo{},
	}
}

func (o outlineCapture) clone() outlineCapture {
	allArgs := map[string]argInfo{}
	for k, v := range o.allArgs {
		allArgs[k] = v
	}
	usedArgs := map[string]struct{}{}
	for k := range o.usedArgs {
		usedArgs[k] = struct{}{}
	}
	secrets := map[string]secretInfo{}
	for k, v := range o.secrets {
		secrets[k] = v
	}
	ssh := map[string]sshInfo{}
	for k, v := range o.ssh {
		ssh[k] = v
	}
	return outlineCapture{
		allArgs:  allArgs,
		usedArgs: usedArgs,
		secrets:  secrets,
		ssh:      ssh,
	}
}

func (o outlineCapture) markAllUsed(in map[string]struct{}) {
	for k := range in {
		if a, ok := o.allArgs[k]; ok {
			o.markAllUsed(a.deps)
		}
		o.usedArgs[k] = struct{}{}
	}
}

func (ds *dispatchState) args(visited map[string]struct{}) []outline.Arg {
	ds.outline.markAllUsed(ds.outline.usedArgs)

	args := make([]outline.Arg, 0, len(ds.outline.usedArgs))
	for k := range ds.outline.usedArgs {
		if a, ok := ds.outline.allArgs[k]; ok {
			if _, ok := visited[k]; !ok {
				args = append(args, outline.Arg{
					Name:        a.definition.Key,
					Value:       a.value,
					Description: a.definition.Comment,
					Location:    toSourceLocation(a.location),
				})
				visited[k] = struct{}{}
			}
		}
	}

	if ds.base != nil {
		args = append(args, ds.base.args(visited)...)
	}
	for d := range ds.deps {
		args = append(args, d.args(visited)...)
	}

	return args
}

func (ds *dispatchState) secrets(visited map[string]struct{}) []outline.Secret {
	secrets := make([]outline.Secret, 0, len(ds.outline.secrets))
	for k, v := range ds.outline.secrets {
		if _, ok := visited[k]; !ok {
			secrets = append(secrets, outline.Secret{
				Name:     k,
				Required: v.required,
				Location: toSourceLocation(v.location),
			})
			visited[k] = struct{}{}
		}
	}
	if ds.base != nil {
		secrets = append(secrets, ds.base.secrets(visited)...)
	}
	for d := range ds.deps {
		secrets = append(secrets, d.secrets(visited)...)
	}
	return secrets
}

func (ds *dispatchState) ssh(visited map[string]struct{}) []outline.SSH {
	ssh := make([]outline.SSH, 0, len(ds.outline.secrets))
	for k, v := range ds.outline.ssh {
		if _, ok := visited[k]; !ok {
			ssh = append(ssh, outline.SSH{
				Name:     k,
				Required: v.required,
				Location: toSourceLocation(v.location),
			})
			visited[k] = struct{}{}
		}
	}
	if ds.base != nil {
		ssh = append(ssh, ds.base.ssh(visited)...)
	}
	for d := range ds.deps {
		ssh = append(ssh, d.ssh(visited)...)
	}
	return ssh
}

func (ds *dispatchState) Outline(dt []byte) outline.Outline {
	args := ds.args(map[string]struct{}{})
	sort.Slice(args, func(i, j int) bool {
		return compLocation(args[i].Location, args[j].Location)
	})

	secrets := ds.secrets(map[string]struct{}{})
	sort.Slice(secrets, func(i, j int) bool {
		return compLocation(secrets[i].Location, secrets[j].Location)
	})

	ssh := ds.ssh(map[string]struct{}{})
	sort.Slice(ssh, func(i, j int) bool {
		return compLocation(ssh[i].Location, ssh[j].Location)
	})

	out := outline.Outline{
		Name:        ds.stage.Name,
		Description: ds.stage.Comment,
		Sources:     [][]byte{dt},
		Args:        args,
		Secrets:     secrets,
		SSH:         ssh,
	}

	return out
}

func toSourceLocation(r []parser.Range) *pb.Location {
	if len(r) == 0 {
		return nil
	}
	arr := make([]*pb.Range, len(r))
	for i, r := range r {
		arr[i] = &pb.Range{
			Start: pb.Position{
				Line:      int32(r.Start.Line),
				Character: int32(r.Start.Character),
			},
			End: pb.Position{
				Line:      int32(r.End.Line),
				Character: int32(r.End.Character),
			},
		}
	}
	return &pb.Location{Ranges: arr}
}

func compLocation(a, b *pb.Location) bool {
	if a.SourceIndex != b.SourceIndex {
		return a.SourceIndex < b.SourceIndex
	}
	linea := 0
	lineb := 0
	if len(a.Ranges) > 0 {
		linea = int(a.Ranges[0].Start.Line)
	}
	if len(b.Ranges) > 0 {
		lineb = int(b.Ranges[0].Start.Line)
	}
	return linea < lineb
}
