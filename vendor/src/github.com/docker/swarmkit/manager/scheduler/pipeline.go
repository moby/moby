package scheduler

import "github.com/docker/swarmkit/api"

var (
	defaultFilters = []Filter{
		// Always check for readiness first.
		&ReadyFilter{},
		&ResourceFilter{},

		// TODO(stevvooe): Do not filter based on plugins since they are lazy
		// loaded in the engine. We can add this back when we can schedule
		// plugins in the future.
		// &PluginFilter{},

		&ConstraintFilter{},
	}
)

type checklistEntry struct {
	f       Filter
	enabled bool
}

// Pipeline runs a set of filters against nodes.
type Pipeline struct {
	checklist []checklistEntry
}

// NewPipeline returns a pipeline with the default set of filters.
func NewPipeline() *Pipeline {
	p := &Pipeline{}

	for _, f := range defaultFilters {
		p.checklist = append(p.checklist, checklistEntry{f: f})
	}

	return p
}

// Process a node through the filter pipeline.
// Returns true if all filters pass, false otherwise.
func (p *Pipeline) Process(n *NodeInfo) bool {
	for _, entry := range p.checklist {
		if entry.enabled && !entry.f.Check(n) {
			// Immediately stop on first failure.
			return false
		}
	}
	return true
}

// SetTask sets up the filters to process a new task. Once this is called,
// Process can be called repeatedly to try to assign the task various nodes.
func (p *Pipeline) SetTask(t *api.Task) {
	for i := range p.checklist {
		p.checklist[i].enabled = p.checklist[i].f.SetTask(t)
	}
}
