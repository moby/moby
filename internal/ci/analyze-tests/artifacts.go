package main

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
)

const (
	artifactPageSize = 100 // GitHub's REST API caps per_page at 100.
	maxArtifactPages = 40
)

// artifact is the subset of a GitHub Actions artifact response used here.
type artifact struct {
	ID          int64
	Name        string
	CreatedAt   time.Time
	Expired     bool
	WorkflowRun artifactWorkflowRun
}

// artifactWorkflowRun is the workflow metadata attached to an artifact.
type artifactWorkflowRun struct {
	ID               int64
	HeadBranch       string
	HeadRepositoryID int64
}

// artifactFilterStats records why artifacts were kept or skipped.
type artifactFilterStats struct {
	Seen                 int
	Selected             int
	Expired              int
	MissingWorkflowRunID int
	WrongPrefix          int
}

// fetchArtifacts collects the non-expired test-result artifacts visible in the
// configured GitHub artifact page limit.
func (g *githubClient) fetchArtifacts(ctx context.Context) ([]artifact, error) {
	slog.Info("fetching test result artifacts")
	owner, repo, err := g.repoParts()
	if err != nil {
		return nil, err
	}
	api := g.githubAPI()
	var out []artifact
	var total artifactFilterStats
	for page := 1; page <= maxArtifactPages; page++ {
		list, _, err := api.Actions.ListArtifacts(ctx, owner, repo, &github.ListArtifactsOptions{
			ListOptions: github.ListOptions{PerPage: artifactPageSize, Page: page},
		})
		if err != nil {
			return nil, g.githubError("list workflow artifacts", err)
		}
		artifacts := normalizeArtifacts(list.Artifacts)
		if g.verbose {
			slog.Info("received artifact page", "page", page, "count", len(artifacts))
			if first, last, ok := artifactTimeRange(artifacts); ok {
				slog.Info("artifact page time range", "page", page, "newest", first.Format(time.RFC3339), "oldest", last.Format(time.RFC3339))
			}
		}
		if len(artifacts) == 0 {
			break
		}

		selected, stats := filterArtifacts(artifacts)
		out = append(out, selected...)
		total.add(stats)
		if g.verbose {
			slog.Info("filtered artifact page", "page", page, "selected", stats.Selected, "expired", stats.Expired, "missing_run", stats.MissingWorkflowRunID, "wrong_prefix", stats.WrongPrefix)
		}
	}
	if g.verbose {
		slog.Info("artifact scan summary", "seen", total.Seen, "selected", total.Selected, "expired", total.Expired, "missing_run", total.MissingWorkflowRunID, "wrong_prefix", total.WrongPrefix)
		if total.Seen == maxArtifactPages*artifactPageSize {
			slog.Info("artifact scan stopped at page limit", "pages", maxArtifactPages)
		}
	}
	return out, nil
}

// normalizeArtifacts converts go-github's pointer-heavy artifact type into the
// small value type used by the analyzer.
func normalizeArtifacts(in []*github.Artifact) []artifact {
	out := make([]artifact, 0, len(in))
	for _, a := range in {
		if a == nil {
			continue
		}
		na := artifact{
			ID:        a.GetID(),
			Name:      a.GetName(),
			CreatedAt: a.GetCreatedAt().Time,
			Expired:   a.GetExpired(),
		}
		if wr := a.GetWorkflowRun(); wr != nil {
			na.WorkflowRun = artifactWorkflowRun{
				ID:               wr.GetID(),
				HeadBranch:       wr.GetHeadBranch(),
				HeadRepositoryID: wr.GetHeadRepositoryID(),
			}
		}
		out = append(out, na)
	}
	return out
}

// filterArtifacts keeps only test-result artifacts that can be traced back to a
// workflow run.
func filterArtifacts(artifacts []artifact) ([]artifact, artifactFilterStats) {
	stats := artifactFilterStats{Seen: len(artifacts)}
	selected := make([]artifact, 0, len(artifacts))
	for _, a := range artifacts {
		switch {
		case a.Expired:
			stats.Expired++
		case a.WorkflowRun.ID == 0:
			stats.MissingWorkflowRunID++
		case !strings.HasPrefix(a.Name, "test-results-"):
			stats.WrongPrefix++
		default:
			selected = append(selected, a)
			stats.Selected++
		}
	}
	return selected, stats
}

// add folds another page's filter statistics into the receiver.
func (s *artifactFilterStats) add(other artifactFilterStats) {
	s.Seen += other.Seen
	s.Selected += other.Selected
	s.Expired += other.Expired
	s.MissingWorkflowRunID += other.MissingWorkflowRunID
	s.WrongPrefix += other.WrongPrefix
}

// artifactTimeRange returns the newest and oldest timestamps in a page.
func artifactTimeRange(artifacts []artifact) (time.Time, time.Time, bool) {
	if len(artifacts) == 0 {
		return time.Time{}, time.Time{}, false
	}
	first := artifacts[0].CreatedAt
	last := artifacts[0].CreatedAt
	for _, a := range artifacts[1:] {
		if a.CreatedAt.After(first) {
			first = a.CreatedAt
		}
		if a.CreatedAt.Before(last) {
			last = a.CreatedAt
		}
	}
	return first, last, true
}
