package solver

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/moby/buildkit/solver/internal/pipe"
	"github.com/moby/buildkit/util/bklog"
	"github.com/tonistiigi/go-csvvalue"
)

var (
	debugScheduler      = false // TODO: replace with logs in build trace
	debugSchedulerSteps = sync.OnceValue(parseSchedulerDebugSteps)
)

func init() {
	if os.Getenv("BUILDKIT_SCHEDULER_DEBUG") == "1" {
		debugScheduler = true
	}
}

func parseSchedulerDebugSteps() []string {
	if s := os.Getenv("BUILDKIT_SCHEDULER_DEBUG_STEPS"); s != "" {
		fields, err := csvvalue.Fields(s, nil)
		if err != nil {
			return nil
		}
		return fields
	}
	return nil
}

// debugSchedulerCheckEdge determines if this edge should be debugged
// depending on the set environment variables.
func debugSchedulerCheckEdge(e *edge) bool {
	if debugScheduler {
		return true
	}

	if steps := debugSchedulerSteps(); len(steps) > 0 {
		withParents := strings.HasSuffix(steps[0], "^")
		name := strings.TrimSuffix(steps[0], "^")
		for _, v := range steps {
			if strings.Contains(name, v) {
				return true
			}
		}

		if withParents {
			for _, vtx := range e.edge.Vertex.Inputs() {
				name := strings.TrimSuffix(vtx.Vertex.Name(), "^")
				for _, v := range steps {
					if strings.Contains(name, v) {
						return true
					}
				}
			}
		}
	}
	return false
}

func debugSchedulerSkipMergeDueToDependency(e, origEdge *edge) {
	bklog.G(context.TODO()).
		WithField("edge_vertex_name", e.edge.Vertex.Name()).
		WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
		WithField("edge_index", e.edge.Index).
		WithField("origEdge_vertex_name", origEdge.edge.Vertex.Name()).
		WithField("origEdge_vertex_digest", origEdge.edge.Vertex.Digest()).
		WithField("origEdge_index", origEdge.edge.Index).
		Debug("skip merge due to dependency")
}

func debugSchedulerSwapMergeDueToOwner(e, origEdge *edge) {
	bklog.G(context.TODO()).
		WithField("edge_vertex_name", e.edge.Vertex.Name()).
		WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
		WithField("edge_index", e.edge.Index).
		WithField("origEdge_vertex_name", origEdge.edge.Vertex.Name()).
		WithField("origEdge_vertex_digest", origEdge.edge.Vertex.Digest()).
		WithField("origEdge_index", origEdge.edge.Index).
		Debug("swap merge due to owner")
}

func debugSchedulerMergingEdges(src, dest *edge) {
	bklog.G(context.TODO()).
		WithField("source_edge_vertex_name", src.edge.Vertex.Name()).
		WithField("source_edge_vertex_digest", src.edge.Vertex.Digest()).
		WithField("source_edge_index", src.edge.Index).
		WithField("dest_vertex_name", dest.edge.Vertex.Name()).
		WithField("dest_vertex_digest", dest.edge.Vertex.Digest()).
		WithField("dest_index", dest.edge.Index).
		Debug("merging edges")
}

func debugSchedulerMergingEdgesSkipped(src, dest *edge) {
	bklog.G(context.TODO()).
		WithField("source_edge_vertex_name", src.edge.Vertex.Name()).
		WithField("source_edge_vertex_digest", src.edge.Vertex.Digest()).
		WithField("source_edge_index", src.edge.Index).
		WithField("dest_vertex_name", dest.edge.Vertex.Name()).
		WithField("dest_vertex_digest", dest.edge.Vertex.Digest()).
		WithField("dest_index", dest.edge.Index).
		Debug("merging edges skipped")
}

func debugSchedulerPreUnpark(e *edge, inc []pipeSender, updates, allPipes []pipeReceiver) {
	if e.debug {
		debugSchedulerPreUnparkSlow(e, inc, updates, allPipes)
	}
}

func debugSchedulerPreUnparkSlow(e *edge, inc []pipeSender, updates, allPipes []pipeReceiver) {
	log := bklog.G(context.TODO()).
		WithField("edge_vertex_name", e.edge.Vertex.Name()).
		WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
		WithField("edge_index", e.edge.Index)

	log.
		WithField("edge_state", e.state).
		WithField("req", len(inc)).
		WithField("upt", len(updates)).
		WithField("out", len(allPipes)).
		Debug(">> unpark")

	for i, dep := range e.deps {
		des := edgeStatusInitial
		if dep.req != nil {
			des = dep.req.Request().desiredState
		}
		log.
			WithField("dep_index", i).
			WithField("dep_vertex_name", e.edge.Vertex.Inputs()[i].Vertex.Name()).
			WithField("dep_vertex_digest", e.edge.Vertex.Inputs()[i].Vertex.Digest()).
			WithField("dep_state", dep.state).
			WithField("dep_desired_state", des).
			WithField("dep_keys", len(dep.keys)).
			WithField("dep_has_slow_cache", e.slowCacheFunc(dep) != nil).
			WithField("dep_preprocess_func", e.preprocessFunc(dep) != nil).
			Debug(":: dep")
	}

	for i, in := range inc {
		req := in.Request()
		log.
			WithField("incoming_index", i).
			WithField("incoming_pointer", in).
			WithField("incoming_desired_state", req.Payload.desiredState).
			WithField("incoming_canceled", req.Canceled).
			Debug("> incoming")
	}

	for i, up := range updates {
		if up == e.cacheMapReq {
			log.
				WithField("update_index", i).
				WithField("update_pointer", up).
				WithField("update_complete", up.Status().Completed).
				Debug("> update cacheMapReq")
		} else if up == e.execReq {
			log.
				WithField("update_index", i).
				WithField("update_pointer", up).
				WithField("update_complete", up.Status().Completed).
				Debug("> update execReq")
		} else {
			st, ok := up.Status().Value.(*edgeState)
			if ok {
				index := -1
				if dep, ok := e.depRequests[up]; ok {
					index = int(dep.index)
				}
				log.
					WithField("update_index", i).
					WithField("update_pointer", up).
					WithField("update_complete", up.Status().Completed).
					WithField("update_input_index", index).
					WithField("update_keys", len(st.keys)).
					WithField("update_state", st.state).
					Debugf("> update edgeState")
			} else {
				log.
					WithField("update_index", i).
					Debug("> update unknown")
			}
		}
	}
}

func debugSchedulerPostUnpark(e *edge, inc []pipeSender) {
	if e.debug {
		debugSchedulerPostUnparkSlow(e, inc)
	}
}

func debugSchedulerPostUnparkSlow(e *edge, inc []pipeSender) {
	log := bklog.G(context.TODO())
	for i, in := range inc {
		log.
			WithField("incoming_index", i).
			WithField("incoming_pointer", in).
			WithField("incoming_complete", in.Status().Completed).
			Debug("< incoming")
	}
	log.
		WithField("edge_vertex_name", e.edge.Vertex.Name()).
		WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
		WithField("edge_index", e.edge.Index).
		WithField("edge_state", e.state).
		Debug("<< unpark")
}

func debugSchedulerNewPipe(e *edge, p *pipe.Pipe[*edgeRequest, any], req *edgeRequest) {
	if e.debug {
		bklog.G(context.TODO()).Debugf("> newPipe %s %p desiredState=%s", e.edge.Vertex.Name(), p, req.desiredState)
	}
}

func debugSchedulerNewFunc(e *edge, p pipeReceiver) {
	if e.debug {
		bklog.G(context.TODO()).Debugf("> newFunc %p", p)
	}
}

func debugSchedulerInconsistentGraphState(ee Edge) {
	bklog.G(context.TODO()).
		WithField("edge_vertex_name", ee.Vertex.Name()).
		WithField("edge_vertex_digest", ee.Vertex.Digest()).
		WithField("edge_index", ee.Index).
		Error("failed to get edge: inconsistent graph state")
}

func debugSchedulerFinishIncoming(e *edge, err error, req pipeSender) {
	if e.debug {
		bklog.G(context.TODO()).Debugf("finishIncoming %s %v %#v desired=%s", e.edge.Vertex.Name(), err, e.edgeState, req.Request().Payload.desiredState)
	}
}

func debugSchedulerUpdateIncoming(e *edge, req pipeSender) {
	if e.debug {
		bklog.G(context.TODO()).Debugf("updateIncoming %s %#v desired=%s", e.edge.Vertex.Name(), e.edgeState, req.Request().Payload.desiredState)
	}
}

func debugSchedulerUpgradeCacheSlow(e *edge) {
	if e.debug {
		bklog.G(context.TODO()).Debugf("upgrade to cache-slow because no open keys")
	}
}

func debugSchedulerRespondToIncomingStatus(e *edge, allIncomingCanComplete bool) {
	if e.debug {
		bklog.G(context.TODO()).Debugf("status state=%s cancomplete=%v hasouts=%v noPossibleCache=%v depsCacheFast=%v keys=%d cacheRecords=%d", e.state, allIncomingCanComplete, e.hasActiveOutgoing, e.noCacheMatchPossible, e.allDepsCompletedCacheFast, len(e.keys), len(e.cacheRecords))
	}
}

func debugSchedulerCancelInputRequest(e *edge, dep *dep, desiredStateDep edgeStatusType) {
	if e.debug {
		bklog.G(context.TODO()).
			WithField("edge_vertex_name", e.edge.Vertex.Name()).
			WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
			WithField("dep_index", dep.index).
			WithField("dep_req_desired_state", dep.req.Request().desiredState).
			WithField("dep_desired_state", desiredStateDep).
			WithField("dep_state", dep.state).
			Debug("cancel input request")
	}
}

func debugSchedulerSkipInputRequestBasedOnExistingRequest(e *edge, dep *dep, desiredStateDep edgeStatusType) {
	if e.debug {
		bklog.G(context.TODO()).
			WithField("edge_vertex_name", e.edge.Vertex.Name()).
			WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
			WithField("dep_index", dep.index).
			WithField("dep_req_desired_state", dep.req.Request().desiredState).
			WithField("dep_desired_state", desiredStateDep).
			WithField("dep_state", dep.state).
			Debug("skip input request based on existing request")
	}
}

func debugSchedulerAddInputRequest(e *edge, dep *dep, desiredStateDep edgeStatusType) {
	if e.debug {
		bklog.G(context.TODO()).
			WithField("edge_vertex_name", e.edge.Vertex.Name()).
			WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
			WithField("dep_index", dep.index).
			WithField("dep_desired_state", desiredStateDep).
			WithField("dep_state", dep.state).
			WithField("dep_vertex_name", e.edge.Vertex.Inputs()[dep.index].Vertex.Name()).
			WithField("dep_vertex_digest", e.edge.Vertex.Inputs()[dep.index].Vertex.Digest()).
			Debug("add input request")
	}
}

func debugSchedulerSkipInputRequestBasedOnDepState(e *edge, dep *dep, desiredStateDep edgeStatusType) {
	if e.debug {
		bklog.G(context.TODO()).
			WithField("edge_vertex_name", e.edge.Vertex.Name()).
			WithField("edge_vertex_digest", e.edge.Vertex.Digest()).
			WithField("dep_index", dep.index).
			WithField("dep_desired_state", desiredStateDep).
			WithField("dep_state", dep.state).
			WithField("dep_vertex_name", e.edge.Vertex.Inputs()[dep.index].Vertex.Name()).
			WithField("dep_vertex_digest", e.edge.Vertex.Inputs()[dep.index].Vertex.Digest()).
			Debug("skip input request based on dep state")
	}
}
