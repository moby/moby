package networkdb

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	"pgregory.net/rapid"
)

// convergencePlan names a file holding a scenario to replay.
var convergencePlan = flag.String("networkdb.convergence-plan", "",
	"file holding a scenario printed by TestNetworkDBAlwaysConverges, to be replayed by TestNetworkDBReplayScenario")

// TestNetworkDBReplayScenario executes the scenario in the file named by
// -networkdb.convergence-plan, and is skipped without one.
//
// It exists to get a failure out of rapid's hands. A plan printed by
// TestNetworkDBAlwaysConverges can be replayed here as many times as it takes,
// and cut down by hand between runs, with no draw buffer to keep in step and
// nothing that has to agree with a recorded seed. See that test's documentation
// for how the modes compare.
func TestNetworkDBReplayScenario(t *testing.T) {
	if *convergencePlan == "" {
		t.Skip("no -networkdb.convergence-plan given")
	}
	requireSynctest(t)

	p := loadScenario(t, *convergencePlan)
	runScenario(t, p)
}

// loadScenario parses the scenario in path.
func loadScenario(t *testing.T, path string) *plan {
	t.Helper()

	b, err := os.ReadFile(path)
	assert.NilError(t, err)
	p, err := parsePlan(string(b))
	assert.NilError(t, err)
	return p
}

// runScenario executes p as many times as -networkdb.convergence-attempts
// asks for, failing on the first execution which does not converge.
func runScenario(t *testing.T, p *plan) {
	t.Helper()

	attempts := max(*convergenceAttempts, 1)
	t.Logf("Replaying %d time(s):\n%s", attempts, p)
	for attempt := range attempts {
		// One bubble per attempt; see testConvergence. A failure inside a bubble
		// becomes FailNow on the parent, so the first failing attempt ends the
		// test and the ones after it never run.
		synctest.Test(t, func(t *testing.T) {
			executePlan(t, p, attempt, attempts)
		})
	}
}

// delayColumn is how much room [action.String] gives the wait before an action,
// so that the actions all start at one column and the waits line up under each
// other.
//
// Fixed rather than sized to the plan in hand, which would be tidier per file and
// worse everywhere it matters: reducing a scenario drops actions, and dropping
// the one with the longest wait would reflow every remaining line, burying the
// actual change in whitespace. Six characters holds "+500ms", the longest the
// state machine draws. A hand-written scenario may wait longer than that, and
// such a line simply overhangs the column, pushing its own action right and
// leaving the rest aligned.
const delayColumn = 6

func (p *plan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "scenario nodes=%d seed=%#x", p.nodes, p.seed)
	if p.stagger > 0 {
		fmt.Fprintf(&b, " stagger=%v", p.stagger)
	}
	b.WriteString("\n")

	for _, a := range p.actions {
		fmt.Fprintf(&b, "  %s\n", a)
	}
	return b.String()
}

var (
	planHeaderRE = regexp.MustCompile(`^scenario nodes=(\d+) seed=(0x[0-9a-fA-F]+)(?: stagger=([0-9hmsun\xb5.]+))?$`)
	planActionRE = regexp.MustCompile(`^#(\d+):(\w+)\((.*)\)$`)
	// An action may be preceded by how long to wait before it, as "+250ms".
	planDelayRE = regexp.MustCompile(`^\+(\S+)\s+`)
)

// parsePlan reads a scenario in the form [plan.String] prints. Blank lines,
// "# " comments, indentation, go test's file:line log prefix and any preamble
// ahead of the header are all tolerated, so that a plan can be pasted straight
// out of a failure. Past the header, a line which does not parse is an error
// naming it.
func parsePlan(s string) (*plan, error) {
	p := &plan{nodes: -1}
	var lineno int
	for line := range strings.Lines(s) {
		lineno++
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if m := planHeaderRE.FindStringSubmatch(line); m != nil {
			if p.nodes >= 0 {
				return nil, fmt.Errorf("line %d: a second scenario header", lineno)
			}
			nodes, err := strconv.Atoi(m[1])
			if err != nil || nodes < 2 {
				return nil, fmt.Errorf("line %d: nodes=%s: want 2 or more", lineno, m[1])
			}
			seed, err := strconv.ParseUint(m[2], 0, 64)
			if err != nil {
				return nil, fmt.Errorf("line %d: seed=%s: %w", lineno, m[2], err)
			}
			p.nodes, p.seed = nodes, seed
			if m[3] != "" {
				stagger, err := time.ParseDuration(m[3])
				if err != nil || stagger < 0 {
					return nil, fmt.Errorf("line %d: stagger=%s: want a non-negative duration", lineno, m[3])
				}
				p.stagger = stagger
			}
			continue
		}
		if p.nodes < 0 {
			// Anything ahead of the header is preamble -- the label go test
			// prints above the plan, a shell prompt, a copied command -- so skip
			// it. Past the header every line has to parse: a plan quietly read as
			// something other than what it says is worse than no plan at all.
			continue
		}

		a, err := parseAction(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineno, err)
		}
		p.actions = append(p.actions, a)
	}
	if p.nodes < 0 {
		return nil, fmt.Errorf("no %q header found", "scenario nodes=N seed=0xN")
	}
	// Fold the plan now so that an inconsistent one is reported before a cluster
	// is ever started, rather than as a puzzling non-convergence.
	if _, err := p.model(); err != nil {
		return nil, err
	}
	return p, nil
}

// TestPlanRoundTrip checks that parsePlan reads back what [plan.String] prints,
// so that the plan quoted in a failure is one TestNetworkDBReplayScenario can
// actually execute. It draws scenarios with the same state machine as
// TestNetworkDBAlwaysConverges but never starts a cluster, so it costs nothing.
func TestPlanRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		numNodes := rapid.IntRange(2, 25).Draw(t, "numNodes")
		numNetworks := rapid.IntRange(1, 5).Draw(t, "numNetworks")
		seed := rapid.Uint64().Draw(t, "rngSeed")
		// Nanoseconds, not the whole milliseconds the search draws: a duration
		// prints to whatever precision it holds, and the awkward values are the
		// ones which would catch a lossy round trip.
		stagger := time.Duration(rapid.IntRange(0, 200_000_000).Draw(t, "stagger"))
		fsm := newNetworkDBFSM(numNodes, numNetworks, rapid.IntRange(0, 500).Draw(t, "maxDelay"))
		t.Repeat(rapid.StateMachineActions(fsm))
		want := &plan{nodes: numNodes, seed: seed, stagger: stagger, actions: fsm.plan}

		got, err := parsePlan(want.String())
		if err != nil {
			t.Fatalf("parsing back\n%s%v", want, err)
		}
		opts := cmp.Options{cmp.AllowUnexported(plan{}, action{}), cmpopts.EquateEmpty()}
		if diff := cmp.Diff(want, got, opts); diff != "" {
			t.Errorf("round trip differs:\n%s", diff)
		}
	})
}

// TestParsePlanRejects covers what a hand-edited scenario is likeliest to get
// wrong. TestPlanRoundTrip cannot: it only ever offers parsePlan what
// [plan.String] prints, and none of these are things it prints.
func TestParsePlanRejects(t *testing.T) {
	const header = "scenario nodes=2 seed=0x1\n"
	for _, tt := range []struct{ name, in string }{
		{"no header", "  #0:JoinNetwork(nw0)\n"},
		{"a second header", header + header},
		{"too few nodes", "scenario nodes=1 seed=0x1\n"},
		{"negative stagger", "scenario nodes=2 seed=0x1 stagger=-1ms\n"},
		{"negative delay", header + "  +-1ms #0:JoinNetwork(nw0)\n"},
		{"unknown action", header + "  #0:Detonate(nw0)\n"},
		{"node out of range", header + "  #9:JoinNetwork(nw0)\n"},
		{"entry without a join", header + "  #0:CreateEntry(nw0, k0=v0)\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if p, err := parsePlan(tt.in); err == nil {
				t.Errorf("parsePlan accepted %q as:\n%s", tt.in, p)
			}
		})
	}
}

func parseAction(line string) (action, error) {
	var delay time.Duration
	if m := planDelayRE.FindStringSubmatch(line); m != nil {
		d, err := time.ParseDuration(m[1])
		if err != nil || d < 0 {
			return action{}, fmt.Errorf("+%s: want a non-negative duration", m[1])
		}
		delay, line = d, strings.TrimSpace(line[len(m[0]):])
	}

	m := planActionRE.FindStringSubmatch(line)
	if m == nil {
		return action{}, fmt.Errorf("cannot parse %q as an action", line)
	}
	nodeidx, err := strconv.Atoi(m[1])
	if err != nil {
		return action{}, fmt.Errorf("node #%s: %w", m[1], err)
	}
	kind, ok := parseActionKind(m[2])
	if !ok {
		return action{}, fmt.Errorf("unknown action %q", m[2])
	}
	args := strings.Split(m[3], ", ")
	a := action{kind: kind, nodeidx: nodeidx, delay: delay}
	switch kind {
	case actionJoinNetwork, actionLeaveNetwork:
		if len(args) != 1 {
			return action{}, fmt.Errorf("%v takes one argument, a network, got %q", kind, m[3])
		}
		a.network = args[0]
	case actionCreateEntry, actionUpdateEntry:
		if len(args) != 2 {
			return action{}, fmt.Errorf("%v takes a network and key=value, got %q", kind, m[3])
		}
		key, value, ok := strings.Cut(args[1], "=")
		if !ok {
			return action{}, fmt.Errorf("%v: want key=value, got %q", kind, args[1])
		}
		a.network, a.key, a.value = args[0], key, value
	case actionDeleteEntry:
		if len(args) != 2 {
			return action{}, fmt.Errorf("%v takes a network and a key, got %q", kind, m[3])
		}
		a.network, a.key = args[0], args[1]
	}
	return a, nil
}

func parseActionKind(s string) (actionKind, bool) {
	if i := slices.Index(actionKindNames[:], s); i >= 0 {
		return actionKind(i), true
	}
	return 0, false
}
