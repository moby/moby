package lint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	"github.com/moby/buildkit/frontend/gateway/client"
	"github.com/moby/buildkit/frontend/subrequests"
	"github.com/moby/buildkit/solver/errdefs"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
)

type SourceInfoMap func(*pb.SourceInfo) *pb.SourceInfo

const RequestLint = "frontend.lint"

var SubrequestLintDefinition = subrequests.Request{
	Name:        RequestLint,
	Version:     "1.0.0",
	Type:        subrequests.TypeRPC,
	Description: "Lint a Dockerfile",
	Opts:        []subrequests.Named{},
	Metadata: []subrequests.Named{
		{Name: "result.json"},
		{Name: "result.txt"},
		{Name: "result.statuscode"},
	},
}

type Warning struct {
	RuleName    string       `json:"ruleName"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Detail      string       `json:"detail,omitempty"`
	Location    *pb.Location `json:"location,omitempty"`
}

func (w *Warning) PrintTo(wr io.Writer, sources []*pb.SourceInfo, scb SourceInfoMap) error {
	fmt.Fprintf(wr, "\nWARNING: %s", w.RuleName)
	if w.URL != "" {
		fmt.Fprintf(wr, " - %s", w.URL)
	}
	fmt.Fprintf(wr, "\n%s\n", w.Detail)

	if w.Location.SourceIndex < 0 {
		return nil
	}
	sourceInfo := sources[w.Location.SourceIndex]
	if scb != nil {
		sourceInfo = scb(sourceInfo)
	}
	source := errdefs.Source{
		Info:   sourceInfo,
		Ranges: w.Location.Ranges,
	}
	return source.Print(wr)
}

type BuildError struct {
	Message  string      `json:"message"`
	Location pb.Location `json:"location"`
}

type LintResults struct {
	Warnings []Warning        `json:"warnings"`
	Sources  []*pb.SourceInfo `json:"sources"`
	Error    *BuildError      `json:"buildError,omitempty"`
}

func (results *LintResults) AddSource(sourceMap *llb.SourceMap) int {
	newSource := &pb.SourceInfo{
		Filename:   sourceMap.Filename,
		Language:   sourceMap.Language,
		Definition: sourceMap.Definition.ToPB(),
		Data:       sourceMap.Data,
	}
	for i, source := range results.Sources {
		if sourceInfoEqual(source, newSource) {
			return i
		}
	}
	results.Sources = append(results.Sources, newSource)
	return len(results.Sources) - 1
}

func (results *LintResults) AddWarning(rulename, description, url, fmtmsg string, sourceIndex int, location []parser.Range) {
	sourceLocation := []*pb.Range{}
	for _, loc := range location {
		sourceLocation = append(sourceLocation, &pb.Range{
			Start: &pb.Position{
				Line:      int32(loc.Start.Line),
				Character: int32(loc.Start.Character),
			},
			End: &pb.Position{
				Line:      int32(loc.End.Line),
				Character: int32(loc.End.Character),
			},
		})
	}
	pbLocation := &pb.Location{
		SourceIndex: int32(sourceIndex),
		Ranges:      sourceLocation,
	}
	results.Warnings = append(results.Warnings, Warning{
		RuleName:    rulename,
		Description: description,
		URL:         url,
		Detail:      fmtmsg,
		Location:    pbLocation,
	})
}

func (results *LintResults) ToResult(scb SourceInfoMap) (*client.Result, error) {
	res := client.NewResult()
	dt, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, err
	}
	res.AddMeta("result.json", dt)

	b := bytes.NewBuffer(nil)
	if err := PrintLintViolations(dt, b, scb); err != nil {
		return nil, err
	}
	res.AddMeta("result.txt", b.Bytes())

	status := 0
	if len(results.Warnings) > 0 || results.Error != nil {
		status = 1
	}
	res.AddMeta("result.statuscode", []byte(fmt.Sprintf("%d", status)))

	res.AddMeta("version", []byte(SubrequestLintDefinition.Version))
	return res, nil
}

func (results *LintResults) PrintTo(w io.Writer, scb SourceInfoMap) error {
	if err := results.validateWarnings(); err != nil {
		return err
	}

	sort.Slice(results.Warnings, func(i, j int) bool {
		warningI := results.Warnings[i]
		warningJ := results.Warnings[j]
		sourceIndexI := warningI.Location.SourceIndex
		sourceIndexJ := warningJ.Location.SourceIndex
		if sourceIndexI < 0 && sourceIndexJ < 0 {
			return warningI.RuleName < warningJ.RuleName
		} else if sourceIndexI < 0 || sourceIndexJ < 0 {
			return sourceIndexI < 0
		}

		sourceInfoI := results.Sources[warningI.Location.SourceIndex]
		sourceInfoJ := results.Sources[warningJ.Location.SourceIndex]
		if sourceInfoI.Filename != sourceInfoJ.Filename {
			return sourceInfoI.Filename < sourceInfoJ.Filename
		}
		if len(warningI.Location.Ranges) == 0 && len(warningJ.Location.Ranges) == 0 {
			return warningI.RuleName < warningJ.RuleName
		} else if len(warningI.Location.Ranges) == 0 || len(warningJ.Location.Ranges) == 0 {
			return len(warningI.Location.Ranges) == 0
		}

		return warningI.Location.Ranges[0].Start.Line < warningJ.Location.Ranges[0].Start.Line
	})

	for _, warning := range results.Warnings {
		err := warning.PrintTo(w, results.Sources, scb)
		if err != nil {
			return err
		}
	}

	return nil
}

func (results *LintResults) PrintErrorTo(w io.Writer, scb SourceInfoMap) {
	// This prints out the error in LintResults to the writer in a format that
	// is consistent with the warnings for easier downstream consumption.
	if results.Error == nil {
		return
	}

	fmt.Fprintln(w, results.Error.Message)
	sourceInfo := results.Sources[results.Error.Location.SourceIndex]
	if scb != nil {
		sourceInfo = scb(sourceInfo)
	}
	source := errdefs.Source{
		Info:   sourceInfo,
		Ranges: results.Error.Location.Ranges,
	}
	source.Print(w)
}

func (results *LintResults) validateWarnings() error {
	for _, warning := range results.Warnings {
		if int(warning.Location.SourceIndex) >= len(results.Sources) {
			return errors.Errorf("sourceIndex is out of range")
		}
		if warning.Location.SourceIndex > 0 {
			warningSource := results.Sources[warning.Location.SourceIndex]
			if warningSource == nil {
				return errors.Errorf("sourceIndex points to nil source")
			}
		}
	}
	return nil
}

func PrintLintViolations(dt []byte, w io.Writer, scb SourceInfoMap) error {
	var results LintResults

	if err := json.Unmarshal(dt, &results); err != nil {
		return err
	}

	return results.PrintTo(w, scb)
}

func sourceInfoEqual(a, b *pb.SourceInfo) bool {
	if a.Filename != b.Filename || a.Language != b.Language {
		return false
	}
	return bytes.Equal(a.Data, b.Data)
}
