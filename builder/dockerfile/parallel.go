package dockerfile

import (
	"fmt"
	"io"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder/dockerfile/parallel"
	"github.com/docker/docker/pkg/dag"
	"github.com/docker/docker/pkg/dag/scheduler"
)

// parallelBuilder is a parallel image builder
//  - Parses the Dockerfile, create stage DAG, and determine scheduling
//  - Calls NewBuilder() for each of stage in parallel, with Parallel=false
//  - Return map[stageIdx]imageID
type parallelBuilder struct {
	b      *Builder
	stdout io.Writer
	stderr io.Writer
	output io.Writer

	imageIDs   map[int]string // map[stageIdx]imageID
	imageIDsMu sync.Mutex
}

func (parb *parallelBuilder) prepare() ([]*parallel.Stage, *dag.Graph, *scheduler.ScheduleRoot, error) {
	stages, err := parallel.ParseStages(parb.b.dockerfile)
	if err != nil {
		return nil, nil, nil, err
	}
	logrus.Debugf("[PARALLEL BUILDER] Detected %d build stages", len(stages))
	daggy, err := parallel.CreateDAG(stages)
	if err != nil {
		return nil, nil, nil, err
	}
	logrus.Debugf("[PARALLEL BUILDER] DAG: %+v", daggy)
	sched := scheduler.DetermineSchedule(daggy)
	logrus.Debugf("[PARALLEL BUILDER] Schedule: %s", sched.String())

	fmt.Fprintf(parb.stdout, "Prepared the experimental parallel builder.\n"+
		" * Total stages: %d\n"+
		" * DAG         : %+v\n"+
		" * Schedule    : %+v\n"+
		"Note that this feature is not stable yet. Use carefully.\n"+
		"Notes:\n"+
		" * you might need to pull base images manually before running this. Otherwise they would be puled redundantly by the child worker goroutines.\n"+
		" * --parallelism is just ignored currently, and the maximum parallelism is forcibly used.\n"+
		"\n",
		len(stages),
		daggy,
		sched.String())
	return stages, daggy, sched, nil
}

// ensureBaseImages pulls the base images so that parallel child proc won't need to pull them redundantly
func (parb *parallelBuilder) ensureBaseImages(stages []*parallel.Stage) error {
	logrus.Warnf("ensureBaseImages() is not implemented yet. child proc may pull images redundantly")
	return nil
}

// BuildStages build stages and returns map[stageIdx]imageID
func (parb *parallelBuilder) BuildStages() (map[int]string, error) {
	stages, daggy, sched, err := parb.prepare()
	if err != nil {
		return nil, err
	}
	if err = parb.ensureBaseImages(stages); err != nil {
		return nil, err
	}
	err = scheduler.ExecuteSchedule(daggy, sched,
		int(parb.b.options.Parallelism),
		func(n dag.Node) error {
			imageID, err2 := parb.buildStage(stages, int(n))
			if err2 != nil {
				return err2
			}
			parb.imageIDsMu.Lock()
			parb.imageIDs[int(n)] = imageID
			parb.imageIDsMu.Unlock()
			return nil
		})
	return parb.imageIDs, err
}

func cloneImageBuildOptionsForBuildingStage(c *types.ImageBuildOptions, lastStage bool) (*types.ImageBuildOptions, error, []error) {
	var (
		warns []error
		tags  []string
	)
	if lastStage {
		tags = c.Tags
	}
	if c.Context != nil {
		return nil, fmt.Errorf("Unsupported for parallel: Context (%v)", c.Context), warns
	}
	if c.Squash {
		return nil, fmt.Errorf("Unsupported for parallel: Squash"), warns
	}
	if len(c.CacheFrom) != 0 {
		return nil, fmt.Errorf("Unsupported for parallel: CacheFrom (%v)", c.CacheFrom), warns
	}
	cloned := &types.ImageBuildOptions{
		Tags:           tags, // !
		SuppressOutput: c.SuppressOutput,
		RemoteContext:  c.RemoteContext,
		NoCache:        c.NoCache,
		Remove:         c.Remove,
		ForceRemove:    c.ForceRemove,
		PullParent:     c.PullParent,
		Isolation:      c.Isolation,
		CPUSetCPUs:     c.CPUSetCPUs,
		CPUShares:      c.CPUShares,
		CPUQuota:       c.CPUQuota,
		Memory:         c.Memory,
		MemorySwap:     c.MemorySwap,
		CgroupParent:   c.CgroupParent,
		NetworkMode:    c.NetworkMode,
		ShmSize:        c.ShmSize,
		Dockerfile:     "", // !
		Ulimits:        c.Ulimits,
		BuildArgs:      c.BuildArgs,
		AuthConfigs:    c.AuthConfigs,
		Context:        nil, // !
		Labels:         c.Labels,
		Squash:         false, // !
		CacheFrom:      nil,   // !
		SecurityOpt:    c.SecurityOpt,
		ExtraHosts:     c.ExtraHosts,
		Parallelism:    0,     // !
		Parallel:       false, // !
	}
	return cloned, nil, warns
}

// returns map[stageName]imageID
func (parb *parallelBuilder) getDependencyStageImageIDs(stages []*parallel.Stage, idx int) map[string]string {
	res := make(map[string]string, 0)
	depStages := parallel.ComputeAllDependencyStages(stages, idx)
	for _, depIdx := range depStages {
		depName := stages[depIdx].Name
		parb.imageIDsMu.Lock()
		depImageID := parb.imageIDs[int(depIdx)]
		parb.imageIDsMu.Unlock()
		res[depName] = depImageID
	}
	return res
}

func (parb *parallelBuilder) buildStage(stages []*parallel.Stage, idx int) (string, error) {
	depStageImageIDs := parb.getDependencyStageImageIDs(stages, idx)
	logrus.Debugf("[PARALLEL BUILDER] Building stage %d with deps %v", idx, depStageImageIDs)
	fmt.Fprintf(parb.stdout, "Building stage %d with deps %v\n", idx, depStageImageIDs)

	df := parallel.InjectDependencyStageImageIDsToDockerfile(stages[idx].Dockerfile, depStageImageIDs)
	lastStage := idx == len(stages)-1
	config, err, warns := cloneImageBuildOptionsForBuildingStage(parb.b.options, lastStage)
	if err != nil {
		return "", err
	}
	for _, warn := range warns {
		logrus.Warnf("[PARALLEL BUILDER]: %v", warn)
		fmt.Fprintf(parb.stderr, "WARNING: %v\n", warn)
	}
	newb, err := NewBuilder(parb.b.clientCtx, config, parb.b.docker, parb.b.context, nil)
	if err != nil {
		return "", err
	}
	newb.dockerfile = df
	imageID, err := newb.build(
		&stageBuildStdioWriter{w: parb.stdout, stage: idx},
		&stageBuildStdioWriter{w: parb.stderr, stage: idx},
		parb.output)
	if err != nil {
		return "", err
	}
	logrus.Debugf("[PARALLEL BUILDER] Built stage %d as %s", idx, imageID)
	fmt.Fprintf(parb.stdout, "Built stage %d as %s\n", idx, imageID)
	return imageID, nil
}

type stageBuildStdioWriter struct {
	w     io.Writer
	stage int
}

func (w *stageBuildStdioWriter) Write(p []byte) (int, error) {
	prefix := fmt.Sprintf("(parallel) Stage %d ", w.stage)
	return w.w.Write(append([]byte(prefix), p...))
}
