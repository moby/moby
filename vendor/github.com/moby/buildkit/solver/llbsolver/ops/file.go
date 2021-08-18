package ops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"runtime"
	"sort"
	"sync"

	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/solver"
	"github.com/moby/buildkit/solver/llbsolver"
	"github.com/moby/buildkit/solver/llbsolver/errdefs"
	"github.com/moby/buildkit/solver/llbsolver/file"
	"github.com/moby/buildkit/solver/llbsolver/ops/fileoptypes"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/flightcontrol"
	"github.com/moby/buildkit/worker"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

const fileCacheType = "buildkit.file.v0"

type fileOp struct {
	op          *pb.FileOp
	md          *metadata.Store
	w           worker.Worker
	solver      *FileOpSolver
	numInputs   int
	parallelism *semaphore.Weighted
}

func NewFileOp(v solver.Vertex, op *pb.Op_File, cm cache.Manager, parallelism *semaphore.Weighted, md *metadata.Store, w worker.Worker) (solver.Op, error) {
	if err := llbsolver.ValidateOp(&pb.Op{Op: op}); err != nil {
		return nil, err
	}
	return &fileOp{
		op:          op.File,
		md:          md,
		numInputs:   len(v.Inputs()),
		w:           w,
		solver:      NewFileOpSolver(w, &file.Backend{}, file.NewRefManager(cm)),
		parallelism: parallelism,
	}, nil
}

func (f *fileOp) CacheMap(ctx context.Context, g session.Group, index int) (*solver.CacheMap, bool, error) {
	selectors := map[int][]llbsolver.Selector{}
	invalidSelectors := map[int]struct{}{}

	actions := make([][]byte, 0, len(f.op.Actions))

	markInvalid := func(idx pb.InputIndex) {
		if idx != -1 {
			invalidSelectors[int(idx)] = struct{}{}
		}
	}

	indexes := make([][]int, 0, len(f.op.Actions))

	for _, action := range f.op.Actions {
		var dt []byte
		var err error
		switch a := action.Action.(type) {
		case *pb.FileAction_Mkdir:
			p := *a.Mkdir
			markInvalid(action.Input)
			processOwner(p.Owner, selectors)
			dt, err = json.Marshal(p)
			if err != nil {
				return nil, false, err
			}
		case *pb.FileAction_Mkfile:
			p := *a.Mkfile
			markInvalid(action.Input)
			processOwner(p.Owner, selectors)
			dt, err = json.Marshal(p)
			if err != nil {
				return nil, false, err
			}
		case *pb.FileAction_Rm:
			p := *a.Rm
			markInvalid(action.Input)
			dt, err = json.Marshal(p)
			if err != nil {
				return nil, false, err
			}
		case *pb.FileAction_Copy:
			p := *a.Copy
			markInvalid(action.Input)
			processOwner(p.Owner, selectors)
			if action.SecondaryInput != -1 && int(action.SecondaryInput) < f.numInputs {
				addSelector(selectors, int(action.SecondaryInput), p.Src, p.AllowWildcard, p.FollowSymlink, p.IncludePatterns, p.ExcludePatterns)
				p.Src = path.Base(p.Src)
			}
			dt, err = json.Marshal(p)
			if err != nil {
				return nil, false, err
			}
		}

		actions = append(actions, dt)
		indexes = append(indexes, []int{int(action.Input), int(action.SecondaryInput), int(action.Output)})
	}

	if isDefaultIndexes(indexes) {
		indexes = nil
	}

	dt, err := json.Marshal(struct {
		Type    string
		Actions [][]byte
		Indexes [][]int `json:"indexes,omitempty"`
	}{
		Type:    fileCacheType,
		Actions: actions,
		Indexes: indexes,
	})
	if err != nil {
		return nil, false, err
	}

	cm := &solver.CacheMap{
		Digest: digest.FromBytes(dt),
		Deps: make([]struct {
			Selector          digest.Digest
			ComputeDigestFunc solver.ResultBasedCacheFunc
			PreprocessFunc    solver.PreprocessFunc
		}, f.numInputs),
	}

	for idx, m := range selectors {
		if _, ok := invalidSelectors[idx]; ok {
			continue
		}
		dgsts := make([][]byte, 0, len(m))
		for _, k := range m {
			dgsts = append(dgsts, []byte(k.Path))
		}
		sort.Slice(dgsts, func(i, j int) bool {
			return bytes.Compare(dgsts[i], dgsts[j]) > 0
		})
		cm.Deps[idx].Selector = digest.FromBytes(bytes.Join(dgsts, []byte{0}))

		cm.Deps[idx].ComputeDigestFunc = llbsolver.NewContentHashFunc(dedupeSelectors(m))
	}
	for idx := range cm.Deps {
		cm.Deps[idx].PreprocessFunc = llbsolver.UnlazyResultFunc
	}

	return cm, true, nil
}

func (f *fileOp) Exec(ctx context.Context, g session.Group, inputs []solver.Result) ([]solver.Result, error) {
	inpRefs := make([]fileoptypes.Ref, 0, len(inputs))
	for _, inp := range inputs {
		workerRef, ok := inp.Sys().(*worker.WorkerRef)
		if !ok {
			return nil, errors.Errorf("invalid reference for exec %T", inp.Sys())
		}
		inpRefs = append(inpRefs, workerRef.ImmutableRef)
	}

	outs, err := f.solver.Solve(ctx, inpRefs, f.op.Actions, g)
	if err != nil {
		return nil, err
	}

	outResults := make([]solver.Result, 0, len(outs))
	for _, out := range outs {
		outResults = append(outResults, worker.NewWorkerRefResult(out.(cache.ImmutableRef), f.w))
	}

	return outResults, nil
}

func (f *fileOp) Acquire(ctx context.Context) (solver.ReleaseFunc, error) {
	if f.parallelism == nil {
		return func() {}, nil
	}
	err := f.parallelism.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	return func() {
		f.parallelism.Release(1)
	}, nil
}

func addSelector(m map[int][]llbsolver.Selector, idx int, sel string, wildcard, followLinks bool, includePatterns, excludePatterns []string) {
	s := llbsolver.Selector{
		Path:            sel,
		FollowLinks:     followLinks,
		Wildcard:        wildcard && containsWildcards(sel),
		IncludePatterns: includePatterns,
		ExcludePatterns: excludePatterns,
	}

	m[idx] = append(m[idx], s)
}

func containsWildcards(name string) bool {
	isWindows := runtime.GOOS == "windows"
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch == '\\' && !isWindows {
			i++
		} else if ch == '*' || ch == '?' || ch == '[' {
			return true
		}
	}
	return false
}

func dedupeSelectors(m []llbsolver.Selector) []llbsolver.Selector {
	paths := make([]string, 0, len(m))
	pathsFollow := make([]string, 0, len(m))
	for _, sel := range m {
		if !sel.HasWildcardOrFilters() {
			if sel.FollowLinks {
				pathsFollow = append(pathsFollow, sel.Path)
			} else {
				paths = append(paths, sel.Path)
			}
		}
	}
	paths = dedupePaths(paths)
	pathsFollow = dedupePaths(pathsFollow)
	selectors := make([]llbsolver.Selector, 0, len(m))

	for _, p := range paths {
		selectors = append(selectors, llbsolver.Selector{Path: p})
	}
	for _, p := range pathsFollow {
		selectors = append(selectors, llbsolver.Selector{Path: p, FollowLinks: true})
	}

	for _, sel := range m {
		if sel.HasWildcardOrFilters() {
			selectors = append(selectors, sel)
		}
	}

	sort.Slice(selectors, func(i, j int) bool {
		return selectors[i].Path < selectors[j].Path
	})

	return selectors
}

func processOwner(chopt *pb.ChownOpt, selectors map[int][]llbsolver.Selector) error {
	if chopt == nil {
		return nil
	}
	if chopt.User != nil {
		if u, ok := chopt.User.User.(*pb.UserOpt_ByName); ok {
			if u.ByName.Input < 0 {
				return errors.Errorf("invalid user index %d", u.ByName.Input)
			}
			addSelector(selectors, int(u.ByName.Input), "/etc/passwd", false, true, nil, nil)
		}
	}
	if chopt.Group != nil {
		if u, ok := chopt.Group.User.(*pb.UserOpt_ByName); ok {
			if u.ByName.Input < 0 {
				return errors.Errorf("invalid user index %d", u.ByName.Input)
			}
			addSelector(selectors, int(u.ByName.Input), "/etc/group", false, true, nil, nil)
		}
	}
	return nil
}

func NewFileOpSolver(w worker.Worker, b fileoptypes.Backend, r fileoptypes.RefManager) *FileOpSolver {
	return &FileOpSolver{
		w:    w,
		b:    b,
		r:    r,
		outs: map[int]int{},
		ins:  map[int]input{},
	}
}

type FileOpSolver struct {
	w worker.Worker
	b fileoptypes.Backend
	r fileoptypes.RefManager

	mu   sync.Mutex
	outs map[int]int
	ins  map[int]input
	g    flightcontrol.Group
}

type input struct {
	requiresCommit bool
	mount          fileoptypes.Mount
	ref            fileoptypes.Ref
}

func (s *FileOpSolver) Solve(ctx context.Context, inputs []fileoptypes.Ref, actions []*pb.FileAction, g session.Group) ([]fileoptypes.Ref, error) {
	for i, a := range actions {
		if int(a.Input) < -1 || int(a.Input) >= len(inputs)+len(actions) {
			return nil, errors.Errorf("invalid input index %d, %d provided", a.Input, len(inputs)+len(actions))
		}
		if int(a.SecondaryInput) < -1 || int(a.SecondaryInput) >= len(inputs)+len(actions) {
			return nil, errors.Errorf("invalid secondary input index %d, %d provided", a.Input, len(inputs))
		}

		inp, ok := s.ins[int(a.Input)]
		if ok {
			inp.requiresCommit = true
		}
		s.ins[int(a.Input)] = inp

		inp, ok = s.ins[int(a.SecondaryInput)]
		if ok {
			inp.requiresCommit = true
		}
		s.ins[int(a.SecondaryInput)] = inp

		if a.Output != -1 {
			if _, ok := s.outs[int(a.Output)]; ok {
				return nil, errors.Errorf("duplicate output %d", a.Output)
			}
			idx := len(inputs) + i
			s.outs[int(a.Output)] = idx
			s.ins[idx] = input{requiresCommit: true}
		}
	}

	if len(s.outs) == 0 {
		return nil, errors.Errorf("no outputs specified")
	}

	for i := 0; i < len(s.outs); i++ {
		if _, ok := s.outs[i]; !ok {
			return nil, errors.Errorf("missing output index %d", i)
		}
	}

	defer func() {
		for _, in := range s.ins {
			if in.ref == nil && in.mount != nil {
				in.mount.Release(context.TODO())
			}
		}
	}()

	outs := make([]fileoptypes.Ref, len(s.outs))

	eg, ctx := errgroup.WithContext(ctx)
	for i, idx := range s.outs {
		func(i, idx int) {
			eg.Go(func() error {
				if err := s.validate(idx, inputs, actions, nil); err != nil {
					return err
				}
				inp, err := s.getInput(ctx, idx, inputs, actions, g)
				if err != nil {
					return errdefs.WithFileActionError(err, idx-len(inputs))
				}
				outs[i] = inp.ref
				return nil
			})
		}(i, idx)
	}

	if err := eg.Wait(); err != nil {
		for _, r := range outs {
			if r != nil {
				r.Release(context.TODO())
			}
		}
		return nil, err
	}

	return outs, nil
}

func (s *FileOpSolver) validate(idx int, inputs []fileoptypes.Ref, actions []*pb.FileAction, loaded []int) error {
	for _, check := range loaded {
		if idx == check {
			return errors.Errorf("loop from index %d", idx)
		}
	}
	if idx < len(inputs) {
		return nil
	}
	loaded = append(loaded, idx)
	action := actions[idx-len(inputs)]
	for _, inp := range []int{int(action.Input), int(action.SecondaryInput)} {
		if err := s.validate(inp, inputs, actions, loaded); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileOpSolver) getInput(ctx context.Context, idx int, inputs []fileoptypes.Ref, actions []*pb.FileAction, g session.Group) (input, error) {
	inp, err := s.g.Do(ctx, fmt.Sprintf("inp-%d", idx), func(ctx context.Context) (_ interface{}, err error) {
		s.mu.Lock()
		inp := s.ins[idx]
		s.mu.Unlock()
		if inp.mount != nil || inp.ref != nil {
			return inp, nil
		}

		if idx < len(inputs) {
			inp.ref = inputs[idx]
			s.mu.Lock()
			s.ins[idx] = inp
			s.mu.Unlock()
			return inp, nil
		}

		var inpMount, inpMountSecondary fileoptypes.Mount
		var toRelease []fileoptypes.Mount
		action := actions[idx-len(inputs)]

		defer func() {
			if err != nil && inpMount != nil {
				inputRes := make([]solver.Result, len(inputs))
				for i, input := range inputs {
					inputRes[i] = worker.NewWorkerRefResult(input.(cache.ImmutableRef), s.w)
				}

				outputRes := make([]solver.Result, len(actions))

				// Commit the mutable for the primary input of the failed action.
				if !inpMount.Readonly() {
					ref, cerr := s.r.Commit(ctx, inpMount)
					if cerr == nil {
						outputRes[idx-len(inputs)] = worker.NewWorkerRefResult(ref.(cache.ImmutableRef), s.w)
					}
				}

				// If the action has a secondary input, commit it and set the ref on
				// the output results.
				if inpMountSecondary != nil && !inpMountSecondary.Readonly() {
					ref2, cerr := s.r.Commit(ctx, inpMountSecondary)
					if cerr == nil {
						outputRes[int(action.SecondaryInput)-len(inputs)] = worker.NewWorkerRefResult(ref2.(cache.ImmutableRef), s.w)
					}
				}

				err = errdefs.WithExecErrorWithContext(ctx, err, inputRes, outputRes)
			}
			for _, m := range toRelease {
				m.Release(context.TODO())
			}
		}()

		loadInput := func(ctx context.Context) func() error {
			return func() error {
				inp, err := s.getInput(ctx, int(action.Input), inputs, actions, g)
				if err != nil {
					return err
				}
				if inp.ref != nil {
					m, err := s.r.Prepare(ctx, inp.ref, false, g)
					if err != nil {
						return err
					}
					inpMount = m
					return nil
				}
				inpMount = inp.mount
				return nil
			}
		}

		loadSecondaryInput := func(ctx context.Context) func() error {
			return func() error {
				inp, err := s.getInput(ctx, int(action.SecondaryInput), inputs, actions, g)
				if err != nil {
					return err
				}
				if inp.ref != nil {
					m, err := s.r.Prepare(ctx, inp.ref, true, g)
					if err != nil {
						return err
					}
					inpMountSecondary = m
					toRelease = append(toRelease, m)
					return nil
				}
				inpMountSecondary = inp.mount
				return nil
			}
		}

		loadUser := func(ctx context.Context, uopt *pb.UserOpt) (fileoptypes.Mount, error) {
			if uopt == nil {
				return nil, nil
			}
			switch u := uopt.User.(type) {
			case *pb.UserOpt_ByName:
				var m fileoptypes.Mount
				if u.ByName.Input < 0 {
					return nil, errors.Errorf("invalid user index: %d", u.ByName.Input)
				}
				inp, err := s.getInput(ctx, int(u.ByName.Input), inputs, actions, g)
				if err != nil {
					return nil, err
				}
				if inp.ref != nil {
					mm, err := s.r.Prepare(ctx, inp.ref, true, g)
					if err != nil {
						return nil, err
					}
					toRelease = append(toRelease, mm)
					m = mm
				} else {
					m = inp.mount
				}
				return m, nil
			default:
				return nil, nil
			}
		}

		loadOwner := func(ctx context.Context, chopt *pb.ChownOpt) (fileoptypes.Mount, fileoptypes.Mount, error) {
			if chopt == nil {
				return nil, nil, nil
			}
			um, err := loadUser(ctx, chopt.User)
			if err != nil {
				return nil, nil, err
			}
			gm, err := loadUser(ctx, chopt.Group)
			if err != nil {
				return nil, nil, err
			}
			return um, gm, nil
		}

		if action.Input != -1 && action.SecondaryInput != -1 {
			eg, ctx := errgroup.WithContext(ctx)
			eg.Go(loadInput(ctx))
			eg.Go(loadSecondaryInput(ctx))
			if err := eg.Wait(); err != nil {
				return nil, err
			}
		} else {
			if action.Input != -1 {
				if err := loadInput(ctx)(); err != nil {
					return nil, err
				}
			}
			if action.SecondaryInput != -1 {
				if err := loadSecondaryInput(ctx)(); err != nil {
					return nil, err
				}
			}
		}

		if inpMount == nil {
			m, err := s.r.Prepare(ctx, nil, false, g)
			if err != nil {
				return nil, err
			}
			inpMount = m
		}

		switch a := action.Action.(type) {
		case *pb.FileAction_Mkdir:
			user, group, err := loadOwner(ctx, a.Mkdir.Owner)
			if err != nil {
				return nil, err
			}
			if err := s.b.Mkdir(ctx, inpMount, user, group, *a.Mkdir); err != nil {
				return nil, err
			}
		case *pb.FileAction_Mkfile:
			user, group, err := loadOwner(ctx, a.Mkfile.Owner)
			if err != nil {
				return nil, err
			}
			if err := s.b.Mkfile(ctx, inpMount, user, group, *a.Mkfile); err != nil {
				return nil, err
			}
		case *pb.FileAction_Rm:
			if err := s.b.Rm(ctx, inpMount, *a.Rm); err != nil {
				return nil, err
			}
		case *pb.FileAction_Copy:
			if inpMountSecondary == nil {
				m, err := s.r.Prepare(ctx, nil, true, g)
				if err != nil {
					return nil, err
				}
				inpMountSecondary = m
			}
			user, group, err := loadOwner(ctx, a.Copy.Owner)
			if err != nil {
				return nil, err
			}
			if err := s.b.Copy(ctx, inpMountSecondary, inpMount, user, group, *a.Copy); err != nil {
				return nil, err
			}
		default:
			return nil, errors.Errorf("invalid action type %T", action.Action)
		}

		if inp.requiresCommit {
			ref, err := s.r.Commit(ctx, inpMount)
			if err != nil {
				return nil, err
			}
			inp.ref = ref
		} else {
			inp.mount = inpMount
		}
		s.mu.Lock()
		s.ins[idx] = inp
		s.mu.Unlock()
		return inp, nil
	})
	if err != nil {
		return input{}, err
	}
	return inp.(input), err
}

func isDefaultIndexes(idxs [][]int) bool {
	// Older version of checksum did not contain indexes for actions resulting in possibility for a wrong cache match.
	// We detect the most common pattern for indexes and maintain old checksum for that case to minimize cache misses on upgrade.
	// If a future change causes braking changes in instruction cache consider removing this exception.
	if len(idxs) == 0 {
		return false
	}

	for i, idx := range idxs {
		if len(idx) != 3 {
			return false
		}
		// input for first action is first input
		if i == 0 && idx[0] != 0 {
			return false
		}
		// input for other actions is previous action
		if i != 0 && idx[0] != len(idxs)+(i-1) {
			return false
		}
		// secondary input is second input or -1
		if idx[1] != -1 && idx[1] != 1 {
			return false
		}
		// last action creates output
		if i == len(idxs)-1 && idx[2] != 0 {
			return false
		}
		// other actions do not create an output
		if i != len(idxs)-1 && idx[2] != -1 {
			return false
		}
	}
	return true
}
