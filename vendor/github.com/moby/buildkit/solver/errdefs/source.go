package errdefs

import (
	"fmt"
	"io"
	"strings"

	pb "github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/grpcerrors"
	"github.com/pkg/errors"
)

func WithSource(err error, src Source) error {
	if err == nil {
		return nil
	}
	return &ErrorSource{Source: src, error: err}
}

type ErrorSource struct {
	Source
	error
}

func (e *ErrorSource) Unwrap() error {
	return e.error
}

func (e *ErrorSource) ToProto() grpcerrors.TypedErrorProto {
	return &e.Source
}

func Sources(err error) []*Source {
	var out []*Source
	var es *ErrorSource
	if errors.As(err, &es) {
		out = Sources(es.Unwrap())
		out = append(out, &es.Source)
	}
	return out
}

func (s *Source) WrapError(err error) error {
	return &ErrorSource{error: err, Source: *s}
}

func (s *Source) Print(w io.Writer) error {
	si := s.Info
	if si == nil {
		return nil
	}
	lines := strings.Split(string(si.Data), "\n")

	start, end, ok := getStartEndLine(s.Ranges)
	if !ok {
		return nil
	}
	if start > len(lines) || start < 1 {
		return nil
	}
	if end > len(lines) {
		end = len(lines)
	}

	pad := 2
	if end == start {
		pad = 4
	}
	var p int

	prepadStart := start
	for {
		if p >= pad {
			break
		}
		if start > 1 {
			start--
			p++
		}
		if end != len(lines) {
			end++
			p++
		}
		p++
	}

	fmt.Fprintf(w, "%s:%d\n--------------------\n", si.Filename, prepadStart)
	for i := start; i <= end; i++ {
		pfx := "   "
		if containsLine(s.Ranges, i) {
			pfx = ">>>"
		}
		fmt.Fprintf(w, " %3d | %s %s\n", i, pfx, lines[i-1])
	}
	fmt.Fprintf(w, "--------------------\n")
	return nil
}

func containsLine(rr []*pb.Range, l int) bool {
	for _, r := range rr {
		e := r.End.Line
		if e < r.Start.Line {
			e = r.Start.Line
		}
		if r.Start.Line <= int32(l) && e >= int32(l) {
			return true
		}
	}
	return false
}

func getStartEndLine(rr []*pb.Range) (start int, end int, ok bool) {
	first := true
	for _, r := range rr {
		e := r.End.Line
		if e < r.Start.Line {
			e = r.Start.Line
		}
		if first || int(r.Start.Line) < start {
			start = int(r.Start.Line)
		}
		if int(e) > end {
			end = int(e)
		}
		first = false
	}
	return start, end, !first
}
