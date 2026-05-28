package client

import (
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	digest "github.com/opencontainers/go-digest"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var emptyLogVertexSize int

func init() {
	emptyLogVertex := controlapi.VertexLog{}
	emptyLogVertexSize = emptyLogVertex.SizeVT()
}

func NewSolveStatus(resp *controlapi.StatusResponse) *SolveStatus {
	s := &SolveStatus{}
	for _, v := range resp.Vertexes {
		s.Vertexes = append(s.Vertexes, &Vertex{
			Digest:        digest.Digest(v.Digest),
			Inputs:        digestSliceFromPB(v.Inputs),
			Name:          v.Name,
			Started:       timestampFromPB(v.Started),
			Completed:     timestampFromPB(v.Completed),
			Error:         v.Error,
			Cached:        v.Cached,
			ProgressGroup: v.ProgressGroup,
		})
	}
	for _, v := range resp.Statuses {
		s.Statuses = append(s.Statuses, &VertexStatus{
			ID:        v.ID,
			Vertex:    digest.Digest(v.Vertex),
			Name:      v.Name,
			Total:     v.Total,
			Current:   v.Current,
			Timestamp: v.Timestamp.AsTime(),
			Started:   timestampFromPB(v.Started),
			Completed: timestampFromPB(v.Completed),
		})
	}
	for _, v := range resp.Logs {
		s.Logs = append(s.Logs, &VertexLog{
			Vertex:    digest.Digest(v.Vertex),
			Stream:    int(v.Stream),
			Data:      v.Msg,
			Timestamp: v.Timestamp.AsTime(),
		})
	}
	for _, v := range resp.Warnings {
		s.Warnings = append(s.Warnings, &VertexWarning{
			Vertex:     digest.Digest(v.Vertex),
			Level:      int(v.Level),
			Short:      v.Short,
			Detail:     v.Detail,
			URL:        v.Url,
			SourceInfo: v.Info,
			Range:      v.Ranges,
		})
	}
	return s
}

func (ss *SolveStatus) Marshal() (out []*controlapi.StatusResponse) {
	logSize := 0
	for {
		retry := false
		sr := controlapi.StatusResponse{}
		for _, v := range ss.Vertexes {
			sr.Vertexes = append(sr.Vertexes, &controlapi.Vertex{
				Digest:        string(v.Digest),
				Inputs:        digestSliceToPB(v.Inputs),
				Name:          v.Name,
				Started:       timestampToPB(v.Started),
				Completed:     timestampToPB(v.Completed),
				Error:         v.Error,
				Cached:        v.Cached,
				ProgressGroup: v.ProgressGroup,
			})
		}
		for _, v := range ss.Statuses {
			sr.Statuses = append(sr.Statuses, &controlapi.VertexStatus{
				ID:        v.ID,
				Vertex:    string(v.Vertex),
				Name:      v.Name,
				Current:   v.Current,
				Total:     v.Total,
				Timestamp: timestamppb.New(v.Timestamp),
				Started:   timestampToPB(v.Started),
				Completed: timestampToPB(v.Completed),
			})
		}
		for i, v := range ss.Logs {
			sr.Logs = append(sr.Logs, &controlapi.VertexLog{
				Vertex:    string(v.Vertex),
				Stream:    int64(v.Stream),
				Msg:       v.Data,
				Timestamp: timestamppb.New(v.Timestamp),
			})
			logSize += len(v.Data) + emptyLogVertexSize
			// avoid logs growing big and split apart if they do
			if logSize > 1024*1024 {
				ss.Vertexes = nil
				ss.Statuses = nil
				ss.Logs = ss.Logs[i+1:]
				retry = true
				break
			}
		}
		for _, v := range ss.Warnings {
			sr.Warnings = append(sr.Warnings, &controlapi.VertexWarning{
				Vertex: string(v.Vertex),
				Level:  int64(v.Level),
				Short:  v.Short,
				Detail: v.Detail,
				Info:   v.SourceInfo,
				Ranges: v.Range,
				Url:    v.URL,
			})
		}
		out = append(out, &sr)
		if !retry {
			break
		}
	}
	return
}

func digestSliceFromPB(elems []string) []digest.Digest {
	clone := make([]digest.Digest, len(elems))
	for i, e := range elems {
		clone[i] = digest.Digest(e)
	}
	return clone
}

func digestSliceToPB(elems []digest.Digest) []string {
	clone := make([]string, len(elems))
	for i, e := range elems {
		clone[i] = string(e)
	}
	return clone
}

func timestampFromPB(ts *timestamppb.Timestamp) *time.Time {
	if ts != nil {
		t := ts.AsTime()
		return &t
	}
	return nil
}

func timestampToPB(ts *time.Time) *timestamppb.Timestamp {
	if ts != nil {
		return timestamppb.New(*ts)
	}
	return nil
}
