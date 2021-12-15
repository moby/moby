package unused

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"
	"sync"

	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/lint"
	"honnef.co/go/tools/lint/lintdsl"
	"honnef.co/go/tools/ssa"
)

// TODO(dh): conversions between structs mark fields as used, but the
// conversion itself isn't part of that subgraph. even if the function
// containing the conversion is unused, the fields will be marked as
// used.

// TODO(dh): we cannot observe function calls in assembly files.

/*

- packages use:
  - (1.1) exported named types (unless in package main)
  - (1.2) exported functions (unless in package main)
  - (1.3) exported variables (unless in package main)
  - (1.4) exported constants (unless in package main)
  - (1.5) init functions
  - (1.6) functions exported to cgo
  - (1.7) the main function iff in the main package
  - (1.8) symbols linked via go:linkname

- named types use:
  - (2.1) exported methods
  - (2.2) the type they're based on
  - (2.3) all their aliases. we can't easily track uses of aliases
    because go/types turns them into uses of the aliased types. assume
    that if a type is used, so are all of its aliases.

- variables and constants use:
  - their types

- functions use:
  - (4.1) all their arguments, return parameters and receivers
  - (4.2) anonymous functions defined beneath them
  - (4.3) closures and bound methods.
    this implements a simplified model where a function is used merely by being referenced, even if it is never called.
    that way we don't have to keep track of closures escaping functions.
  - (4.4) functions they return. we assume that someone else will call the returned function
  - (4.5) functions/interface methods they call
  - types they instantiate or convert to
  - (4.7) fields they access
  - (4.8) types of all instructions
  - (4.9) package-level variables they assign to iff in tests (sinks for benchmarks)

- conversions use:
  - (5.1) when converting between two equivalent structs, the fields in
    either struct use each other. the fields are relevant for the
    conversion, but only if the fields are also accessed outside the
    conversion.
  - (5.2) when converting to or from unsafe.Pointer, mark all fields as used.

- structs use:
  - (6.1) fields of type NoCopy sentinel
  - (6.2) exported fields
  - (6.3) embedded fields that help implement interfaces (either fully implements it, or contributes required methods) (recursively)
  - (6.4) embedded fields that have exported methods (recursively)
  - (6.5) embedded structs that have exported fields (recursively)

- (7.1) field accesses use fields
- (7.2) fields use their types

- (8.0) How we handle interfaces:
  - (8.1) We do not technically care about interfaces that only consist of
    exported methods. Exported methods on concrete types are always
    marked as used.
  - Any concrete type implements all known interfaces. Even if it isn't
    assigned to any interfaces in our code, the user may receive a value
    of the type and expect to pass it back to us through an interface.

    Concrete types use their methods that implement interfaces. If the
    type is used, it uses those methods. Otherwise, it doesn't. This
    way, types aren't incorrectly marked reachable through the edge
    from method to type.

  - (8.3) All interface methods are marked as used, even if they never get
    called. This is to accomodate sum types (unexported interface
    method that must exist but never gets called.)

  - (8.4) All embedded interfaces are marked as used. This is an
    extension of 8.3, but we have to explicitly track embedded
    interfaces because in a chain C->B->A, B wouldn't be marked as
    used by 8.3 just because it contributes A's methods to C.

- Inherent uses:
  - thunks and other generated wrappers call the real function
  - (9.2) variables use their types
  - (9.3) types use their underlying and element types
  - (9.4) conversions use the type they convert to
  - (9.5) instructions use their operands
  - (9.6) instructions use their operands' types
  - (9.7) variable _reads_ use variables, writes do not, except in tests
  - (9.8) runtime functions that may be called from user code via the compiler


- const groups:
  (10.1) if one constant out of a block of constants is used, mark all
  of them used. a lot of the time, unused constants exist for the sake
  of completeness. See also
  https://github.com/dominikh/go-tools/issues/365



- Differences in whole program mode:
  - (e1) all packages share a single graph
  - (e2) types aim to implement all exported interfaces from all packages
  - (e3) exported identifiers aren't automatically used. for fields and
    methods this poses extra issues due to reflection. We assume
    that all exported fields are used. We also maintain a list of
    known reflection-based method callers.

*/

func assert(b bool) {
	if !b {
		panic("failed assertion")
	}
}

type Checker struct {
	WholeProgram bool
	Debug        io.Writer

	interfaces      []*types.Interface
	initialPackages []*lint.Pkg
	scopes          map[*types.Scope]*ssa.Function

	seenMu sync.Mutex
	seen   map[token.Position]struct{}
	out    []types.Object
}

func (*Checker) Name() string   { return "unused" }
func (*Checker) Prefix() string { return "U" }

func (l *Checker) Checks() []lint.Check {
	return []lint.Check{
		{ID: "U1000", FilterGenerated: true, Fn: l.Lint},
	}
}

func typString(obj types.Object) string {
	switch obj := obj.(type) {
	case *types.Func:
		return "func"
	case *types.Var:
		if obj.IsField() {
			return "field"
		}
		return "var"
	case *types.Const:
		return "const"
	case *types.TypeName:
		return "type"
	default:
		return "identifier"
	}
}

// /usr/lib/go/src/runtime/proc.go:433:6: func badmorestackg0 is unused (U1000)

// Functions defined in the Go runtime that may be called through
// compiler magic or via assembly.
var runtimeFuncs = map[string]bool{
	// The first part of the list is copied from
	// cmd/compile/internal/gc/builtin.go, var runtimeDecls
	"newobject":            true,
	"panicindex":           true,
	"panicslice":           true,
	"panicdivide":          true,
	"panicmakeslicelen":    true,
	"throwinit":            true,
	"panicwrap":            true,
	"gopanic":              true,
	"gorecover":            true,
	"goschedguarded":       true,
	"printbool":            true,
	"printfloat":           true,
	"printint":             true,
	"printhex":             true,
	"printuint":            true,
	"printcomplex":         true,
	"printstring":          true,
	"printpointer":         true,
	"printiface":           true,
	"printeface":           true,
	"printslice":           true,
	"printnl":              true,
	"printsp":              true,
	"printlock":            true,
	"printunlock":          true,
	"concatstring2":        true,
	"concatstring3":        true,
	"concatstring4":        true,
	"concatstring5":        true,
	"concatstrings":        true,
	"cmpstring":            true,
	"intstring":            true,
	"slicebytetostring":    true,
	"slicebytetostringtmp": true,
	"slicerunetostring":    true,
	"stringtoslicebyte":    true,
	"stringtoslicerune":    true,
	"slicecopy":            true,
	"slicestringcopy":      true,
	"decoderune":           true,
	"countrunes":           true,
	"convI2I":              true,
	"convT16":              true,
	"convT32":              true,
	"convT64":              true,
	"convTstring":          true,
	"convTslice":           true,
	"convT2E":              true,
	"convT2Enoptr":         true,
	"convT2I":              true,
	"convT2Inoptr":         true,
	"assertE2I":            true,
	"assertE2I2":           true,
	"assertI2I":            true,
	"assertI2I2":           true,
	"panicdottypeE":        true,
	"panicdottypeI":        true,
	"panicnildottype":      true,
	"ifaceeq":              true,
	"efaceeq":              true,
	"fastrand":             true,
	"makemap64":            true,
	"makemap":              true,
	"makemap_small":        true,
	"mapaccess1":           true,
	"mapaccess1_fast32":    true,
	"mapaccess1_fast64":    true,
	"mapaccess1_faststr":   true,
	"mapaccess1_fat":       true,
	"mapaccess2":           true,
	"mapaccess2_fast32":    true,
	"mapaccess2_fast64":    true,
	"mapaccess2_faststr":   true,
	"mapaccess2_fat":       true,
	"mapassign":            true,
	"mapassign_fast32":     true,
	"mapassign_fast32ptr":  true,
	"mapassign_fast64":     true,
	"mapassign_fast64ptr":  true,
	"mapassign_faststr":    true,
	"mapiterinit":          true,
	"mapdelete":            true,
	"mapdelete_fast32":     true,
	"mapdelete_fast64":     true,
	"mapdelete_faststr":    true,
	"mapiternext":          true,
	"mapclear":             true,
	"makechan64":           true,
	"makechan":             true,
	"chanrecv1":            true,
	"chanrecv2":            true,
	"chansend1":            true,
	"closechan":            true,
	"writeBarrier":         true,
	"typedmemmove":         true,
	"typedmemclr":          true,
	"typedslicecopy":       true,
	"selectnbsend":         true,
	"selectnbrecv":         true,
	"selectnbrecv2":        true,
	"selectsetpc":          true,
	"selectgo":             true,
	"block":                true,
	"makeslice":            true,
	"makeslice64":          true,
	"growslice":            true,
	"memmove":              true,
	"memclrNoHeapPointers": true,
	"memclrHasPointers":    true,
	"memequal":             true,
	"memequal8":            true,
	"memequal16":           true,
	"memequal32":           true,
	"memequal64":           true,
	"memequal128":          true,
	"int64div":             true,
	"uint64div":            true,
	"int64mod":             true,
	"uint64mod":            true,
	"float64toint64":       true,
	"float64touint64":      true,
	"float64touint32":      true,
	"int64tofloat64":       true,
	"uint64tofloat64":      true,
	"uint32tofloat64":      true,
	"complex128div":        true,
	"racefuncenter":        true,
	"racefuncenterfp":      true,
	"racefuncexit":         true,
	"raceread":             true,
	"racewrite":            true,
	"racereadrange":        true,
	"racewriterange":       true,
	"msanread":             true,
	"msanwrite":            true,
	"x86HasPOPCNT":         true,
	"x86HasSSE41":          true,
	"arm64HasATOMICS":      true,

	// The second part of the list is extracted from assembly code in
	// the standard library, with the exception of the runtime package itself
	"abort":                 true,
	"aeshashbody":           true,
	"args":                  true,
	"asminit":               true,
	"badctxt":               true,
	"badmcall2":             true,
	"badmcall":              true,
	"badmorestackg0":        true,
	"badmorestackgsignal":   true,
	"badsignal2":            true,
	"callbackasm1":          true,
	"callCfunction":         true,
	"cgocallback_gofunc":    true,
	"cgocallbackg":          true,
	"checkgoarm":            true,
	"check":                 true,
	"debugCallCheck":        true,
	"debugCallWrap":         true,
	"emptyfunc":             true,
	"entersyscall":          true,
	"exit":                  true,
	"exits":                 true,
	"exitsyscall":           true,
	"externalthreadhandler": true,
	"findnull":              true,
	"goexit1":               true,
	"gostring":              true,
	"i386_set_ldt":          true,
	"_initcgo":              true,
	"init_thread_tls":       true,
	"ldt0setup":             true,
	"libpreinit":            true,
	"load_g":                true,
	"morestack":             true,
	"mstart":                true,
	"nacl_sysinfo":          true,
	"nanotimeQPC":           true,
	"nanotime":              true,
	"newosproc0":            true,
	"newproc":               true,
	"newstack":              true,
	"noted":                 true,
	"nowQPC":                true,
	"osinit":                true,
	"printf":                true,
	"racecallback":          true,
	"reflectcallmove":       true,
	"reginit":               true,
	"rt0_go":                true,
	"save_g":                true,
	"schedinit":             true,
	"setldt":                true,
	"settls":                true,
	"sighandler":            true,
	"sigprofNonGo":          true,
	"sigtrampgo":            true,
	"_sigtramp":             true,
	"sigtramp":              true,
	"stackcheck":            true,
	"syscall_chdir":         true,
	"syscall_chroot":        true,
	"syscall_close":         true,
	"syscall_dup2":          true,
	"syscall_execve":        true,
	"syscall_exit":          true,
	"syscall_fcntl":         true,
	"syscall_forkx":         true,
	"syscall_gethostname":   true,
	"syscall_getpid":        true,
	"syscall_ioctl":         true,
	"syscall_pipe":          true,
	"syscall_rawsyscall6":   true,
	"syscall_rawSyscall6":   true,
	"syscall_rawsyscall":    true,
	"syscall_RawSyscall":    true,
	"syscall_rawsysvicall6": true,
	"syscall_setgid":        true,
	"syscall_setgroups":     true,
	"syscall_setpgid":       true,
	"syscall_setsid":        true,
	"syscall_setuid":        true,
	"syscall_syscall6":      true,
	"syscall_syscall":       true,
	"syscall_Syscall":       true,
	"syscall_sysvicall6":    true,
	"syscall_wait4":         true,
	"syscall_write":         true,
	"traceback":             true,
	"tstart":                true,
	"usplitR0":              true,
	"wbBufFlush":            true,
	"write":                 true,
}

func (c *Checker) Init(prog *lint.Program) {
	for _, pkg := range prog.AllPackages {
		c.interfaces = append(c.interfaces, interfacesFromExportData(pkg.Types)...)
	}
	c.initialPackages = prog.InitialPackages
	c.seen = map[token.Position]struct{}{}

	c.scopes = map[*types.Scope]*ssa.Function{}
	for _, pkg := range prog.InitialPackages {
		for _, fn := range pkg.InitialFunctions {
			if fn.Object() != nil {
				scope := fn.Object().(*types.Func).Scope()
				c.scopes[scope] = fn
			}
		}
	}

	// This is a hack to work in the confines of "one package per
	// job". We do all the actual work in the Init function, and only
	// report results in the actual checker function.
	var out []types.Object
	if c.WholeProgram {
		// (e1) all packages share a single graph
		out = c.processPkgs(prog.InitialPackages...)
	} else {
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, pkg := range prog.InitialPackages {
			pkg := pkg
			wg.Add(1)
			go func() {
				res := c.processPkgs(pkg)
				mu.Lock()
				out = append(out, res...)
				mu.Unlock()
				wg.Done()
			}()
		}
		wg.Wait()
	}
	out2 := make([]types.Object, 0, len(out))
	for _, v := range out {
		if _, ok := c.seen[prog.Fset().Position(v.Pos())]; !ok {
			out2 = append(out2, v)
		}
	}
	c.out = out2
}

func (c *Checker) Lint(j *lint.Job) {
	// The actual work is being done in Init. We only report existing
	// results here.
	unused := c.out
	for _, u := range unused {
		if u.Pkg() != j.Pkg.Types {
			continue
		}
		name := u.Name()
		if sig, ok := u.Type().(*types.Signature); ok && sig.Recv() != nil {
			switch sig.Recv().Type().(type) {
			case *types.Named, *types.Pointer:
				typ := types.TypeString(sig.Recv().Type(), func(*types.Package) string { return "" })
				if len(typ) > 0 && typ[0] == '*' {
					name = fmt.Sprintf("(%s).%s", typ, u.Name())
				} else if len(typ) > 0 {
					name = fmt.Sprintf("%s.%s", typ, u.Name())
				}
			}
		}
		j.Errorf(u, "%s %s is unused", typString(u), name)
	}
}

func (c *Checker) debugf(f string, v ...interface{}) {
	if c.Debug != nil {
		fmt.Fprintf(c.Debug, f, v...)
	}
}

func (graph *Graph) quieten(node *Node) {
	if node.seen {
		return
	}
	switch obj := node.obj.(type) {
	case *ssa.Function:
		sig := obj.Type().(*types.Signature)
		if sig.Recv() != nil {
			if node, ok := graph.nodeMaybe(sig.Recv()); ok {
				node.quiet = true
			}
		}
		for i := 0; i < sig.Params().Len(); i++ {
			if node, ok := graph.nodeMaybe(sig.Params().At(i)); ok {
				node.quiet = true
			}
		}
		for i := 0; i < sig.Results().Len(); i++ {
			if node, ok := graph.nodeMaybe(sig.Results().At(i)); ok {
				node.quiet = true
			}
		}
	case *types.Named:
		for i := 0; i < obj.NumMethods(); i++ {
			m := graph.pkg.Prog.FuncValue(obj.Method(i))
			if node, ok := graph.nodeMaybe(m); ok {
				node.quiet = true
			}
		}
	case *types.Struct:
		for i := 0; i < obj.NumFields(); i++ {
			if node, ok := graph.nodeMaybe(obj.Field(i)); ok {
				node.quiet = true
			}
		}
	case *types.Interface:
		for i := 0; i < obj.NumExplicitMethods(); i++ {
			m := obj.ExplicitMethod(i)
			if node, ok := graph.nodeMaybe(m); ok {
				node.quiet = true
			}
		}
	}
}

func (c *Checker) processPkgs(pkgs ...*lint.Pkg) []types.Object {
	graph := NewGraph()
	graph.wholeProgram = c.WholeProgram
	graph.scopes = c.scopes
	graph.initialPackages = c.initialPackages

	var out []types.Object

	for _, pkg := range pkgs {
		if pkg.PkgPath == "unsafe" {
			continue
		}
		graph.entry(pkg)
	}

	if c.WholeProgram {
		var ifaces []*types.Interface
		var notIfaces []types.Type

		// implement as many interfaces as possible
		graph.seenTypes.Iterate(func(t types.Type, _ interface{}) {
			switch t := t.(type) {
			case *types.Interface:
				ifaces = append(ifaces, t)
			default:
				if _, ok := t.Underlying().(*types.Interface); !ok {
					notIfaces = append(notIfaces, t)
				}
			}
		})

		// OPT(dh): this is not terribly efficient
		ifaces = append(ifaces, c.interfaces...)

		// (8.0) handle interfaces
		// (e2) types aim to implement all exported interfaces from all packages
		for _, t := range notIfaces {
			ms := graph.msCache.MethodSet(t)
			for _, iface := range ifaces {
				if sels, ok := graph.implements(t, iface, ms); ok {
					for _, sel := range sels {
						graph.useMethod(t, sel, t, "implements")
					}
				}
			}
		}
	}

	if c.Debug != nil {
		debugNode := func(node *Node) {
			if node.obj == nil {
				c.debugf("n%d [label=\"Root\"];\n", node.id)
			} else {
				c.debugf("n%d [label=%q];\n", node.id, node.obj)
			}
			for used, reasons := range node.used {
				for _, reason := range reasons {
					c.debugf("n%d -> n%d [label=%q];\n", node.id, used.id, reason)
				}
			}
		}

		c.debugf("digraph{\n")
		debugNode(graph.Root)
		for _, node := range graph.Nodes {
			debugNode(node)
		}
		graph.TypeNodes.Iterate(func(key types.Type, value interface{}) {
			debugNode(value.(*Node))
		})
		c.debugf("}\n")
	}

	graph.color(graph.Root)
	// if a node is unused, don't report any of the node's
	// children as unused. for example, if a function is unused,
	// don't flag its receiver. if a named type is unused, don't
	// flag its methods.

	for _, node := range graph.Nodes {
		graph.quieten(node)
	}
	graph.TypeNodes.Iterate(func(_ types.Type, value interface{}) {
		graph.quieten(value.(*Node))
	})

	report := func(node *Node) {
		if node.seen {
			var pos token.Pos
			switch obj := node.obj.(type) {
			case types.Object:
				pos = obj.Pos()
			case *ssa.Function:
				pos = obj.Pos()
			}

			if pos != 0 {
				c.seenMu.Lock()
				c.seen[pkgs[0].Fset.Position(pos)] = struct{}{}
				c.seenMu.Unlock()
			}
			return
		}
		if node.quiet {
			c.debugf("n%d [color=purple];\n", node.id)
			return
		}

		type packager1 interface {
			Pkg() *types.Package
		}
		type packager2 interface {
			Package() *ssa.Package
		}

		// do not report objects from packages we aren't checking.
	checkPkg:
		switch obj := node.obj.(type) {
		case packager1:
			for _, pkg := range pkgs {
				if pkg.Types == obj.Pkg() {
					break checkPkg
				}
			}
			c.debugf("n%d [color=yellow];\n", node.id)
			return
		case packager2:
			// This happens to filter $bound and $thunk, which
			// should be fine, since we wouldn't want to report
			// them, anyway. Remember that this filtering is only
			// for the output, it doesn't affect the reachability
			// of nodes in the graph.
			for _, pkg := range pkgs {
				if pkg.SSA == obj.Package() {
					break checkPkg
				}
			}
			c.debugf("n%d [color=yellow];\n", node.id)
			return
		}

		c.debugf("n%d [color=red];\n", node.id)
		switch obj := node.obj.(type) {
		case *types.Var:
			// don't report unnamed variables (receivers, interface embedding)
			if obj.Name() != "" || obj.IsField() {
				out = append(out, obj)
			}
		case types.Object:
			if obj.Name() != "_" {
				out = append(out, obj)
			}
		case *ssa.Function:
			if obj == nil {
				// TODO(dh): how does this happen?
				return
			}
			if obj.Object() == nil {
				// Closures
				return
			}
			out = append(out, obj.Object())
		default:
			c.debugf("n%d [color=gray];\n", node.id)
		}
	}
	for _, node := range graph.Nodes {
		report(node)
	}
	graph.TypeNodes.Iterate(func(_ types.Type, value interface{}) {
		report(value.(*Node))
	})

	return out
}

type Graph struct {
	pkg     *ssa.Package
	msCache typeutil.MethodSetCache
	scopes  map[*types.Scope]*ssa.Function

	wholeProgram bool

	nodeCounter int

	Root      *Node
	TypeNodes typeutil.Map
	Nodes     map[interface{}]*Node

	seenTypes typeutil.Map
	seenFns   map[*ssa.Function]struct{}

	initialPackages []*lint.Pkg
}

func NewGraph() *Graph {
	g := &Graph{
		Nodes:   map[interface{}]*Node{},
		seenFns: map[*ssa.Function]struct{}{},
	}
	g.Root = g.newNode(nil)
	return g
}

func (g *Graph) color(root *Node) {
	if root.seen {
		return
	}
	root.seen = true
	for other := range root.used {
		g.color(other)
	}
}

type ConstGroup struct {
	// give the struct a size to get unique pointers
	_ byte
}

func (ConstGroup) String() string { return "const group" }

type Node struct {
	obj  interface{}
	id   int
	used map[*Node][]string

	seen  bool
	quiet bool
}

func (g *Graph) nodeMaybe(obj interface{}) (*Node, bool) {
	if t, ok := obj.(types.Type); ok {
		if v := g.TypeNodes.At(t); v != nil {
			return v.(*Node), true
		}
		return nil, false
	}
	if node, ok := g.Nodes[obj]; ok {
		return node, true
	}
	return nil, false
}

func (g *Graph) node(obj interface{}) (node *Node, new bool) {
	if t, ok := obj.(types.Type); ok {
		if v := g.TypeNodes.At(t); v != nil {
			return v.(*Node), false
		}
		node := g.newNode(obj)
		g.TypeNodes.Set(t, node)
		return node, true
	}
	if node, ok := g.Nodes[obj]; ok {
		return node, false
	}
	node = g.newNode(obj)
	g.Nodes[obj] = node
	return node, true
}

func (g *Graph) newNode(obj interface{}) *Node {
	g.nodeCounter++
	return &Node{
		obj:  obj,
		id:   g.nodeCounter,
		used: map[*Node][]string{},
	}
}

func (n *Node) use(node *Node, reason string) {
	assert(node != nil)
	n.used[node] = append(n.used[node], reason)
}

// isIrrelevant reports whether an object's presence in the graph is
// of any relevance. A lot of objects will never have outgoing edges,
// nor meaningful incoming ones. Examples are basic types and empty
// signatures, among many others.
//
// Dropping these objects should have no effect on correctness, but
// may improve performance. It also helps with debugging, as it
// greatly reduces the size of the graph.
func isIrrelevant(obj interface{}) bool {
	if obj, ok := obj.(types.Object); ok {
		switch obj := obj.(type) {
		case *types.Var:
			if obj.IsField() {
				// We need to track package fields
				return false
			}
			if obj.Pkg() != nil && obj.Parent() == obj.Pkg().Scope() {
				// We need to track package-level variables
				return false
			}
			return isIrrelevant(obj.Type())
		default:
			return false
		}
	}
	if T, ok := obj.(types.Type); ok {
		switch T := T.(type) {
		case *types.Array:
			return isIrrelevant(T.Elem())
		case *types.Slice:
			return isIrrelevant(T.Elem())
		case *types.Basic:
			return true
		case *types.Tuple:
			for i := 0; i < T.Len(); i++ {
				if !isIrrelevant(T.At(i).Type()) {
					return false
				}
			}
			return true
		case *types.Signature:
			if T.Recv() != nil {
				return false
			}
			for i := 0; i < T.Params().Len(); i++ {
				if !isIrrelevant(T.Params().At(i)) {
					return false
				}
			}
			for i := 0; i < T.Results().Len(); i++ {
				if !isIrrelevant(T.Results().At(i)) {
					return false
				}
			}
			return true
		case *types.Interface:
			return T.NumMethods() == 0
		default:
			return false
		}
	}
	return false
}

func (g *Graph) isInterestingPackage(pkg *types.Package) bool {
	if g.wholeProgram {
		for _, opkg := range g.initialPackages {
			if opkg.Types == pkg {
				return true
			}
		}
		return false
	}
	return pkg == g.pkg.Pkg
}

func (g *Graph) see(obj interface{}) {
	if isIrrelevant(obj) {
		return
	}

	assert(obj != nil)
	if obj, ok := obj.(types.Object); ok && obj.Pkg() != nil {
		if !g.isInterestingPackage(obj.Pkg()) {
			return
		}
	}

	// add new node to graph
	g.node(obj)
}

func (g *Graph) use(used, by interface{}, reason string) {
	if isIrrelevant(used) {
		return
	}

	assert(used != nil)
	if _, ok := used.(*types.Func); ok {
		assert(g.pkg.Prog.FuncValue(used.(*types.Func)) == nil)
	}
	if _, ok := by.(*types.Func); ok {
		assert(g.pkg.Prog.FuncValue(by.(*types.Func)) == nil)
	}
	if obj, ok := used.(types.Object); ok && obj.Pkg() != nil {
		if !g.isInterestingPackage(obj.Pkg()) {
			return
		}
	}
	if obj, ok := by.(types.Object); ok && obj.Pkg() != nil {
		if !g.isInterestingPackage(obj.Pkg()) {
			return
		}
	}
	usedNode, new := g.node(used)
	assert(!new)
	if by == nil {
		g.Root.use(usedNode, reason)
	} else {
		byNode, new := g.node(by)
		assert(!new)
		byNode.use(usedNode, reason)
	}
}

func (g *Graph) seeAndUse(used, by interface{}, reason string) {
	g.see(used)
	g.use(used, by, reason)
}

func (g *Graph) trackExportedIdentifier(obj types.Object) bool {
	if !obj.Exported() {
		// object isn't exported, the question is moot
		return false
	}
	if g.wholeProgram {
		// whole program mode tracks exported identifiers accurately
		return false
	}

	path := g.pkg.Prog.Fset.Position(obj.Pos()).Filename
	if g.pkg.Pkg.Name() == "main" && !strings.HasSuffix(path, "_test.go") {
		// exported identifiers in package main can't be imported.
		// However, test functions can be called, and xtest packages
		// even have access to exported identifiers.
		return false
	}

	// at one point we only considered exported identifiers in
	// *_test.go files if they were Benchmark, Example or Test
	// functions. However, this doesn't work when we look at one
	// package at a time, because objects exported in a test variant
	// of a package may be used by the xtest package. The only
	// solution would be to look at multiple packages at once
	return true
}

func (g *Graph) entry(pkg *lint.Pkg) {
	// TODO rename Entry
	g.pkg = pkg.SSA

	for _, f := range pkg.Syntax {
		for _, cg := range f.Comments {
			for _, c := range cg.List {
				if strings.HasPrefix(c.Text, "//go:linkname ") {
					// FIXME(dh): we're looking at all comments. The
					// compiler only looks at comments in the
					// left-most column. The intention probably is to
					// only look at top-level comments.

					// (1.8) packages use symbols linked via go:linkname
					fields := strings.Fields(c.Text)
					if len(fields) == 3 {
						if m, ok := pkg.SSA.Members[fields[1]]; ok {
							var obj interface{}
							switch m := m.(type) {
							case *ssa.Global:
								obj = m.Object()
							case *ssa.Function:
								obj = m
							default:
								panic(fmt.Sprintf("unhandled type: %T", m))
							}
							assert(obj != nil)
							g.seeAndUse(obj, nil, "go:linkname")
						}
					}
				}
			}
		}
	}

	surroundingFunc := func(obj types.Object) *ssa.Function {
		scope := obj.Parent()
		for scope != nil {
			if fn := g.scopes[scope]; fn != nil {
				return fn
			}
			scope = scope.Parent()
		}
		return nil
	}

	// SSA form won't tell us about locally scoped types that aren't
	// being used. Walk the list of Defs to get all named types.
	//
	// SSA form also won't tell us about constants; use Defs and Uses
	// to determine which constants exist and which are being used.
	for _, obj := range pkg.TypesInfo.Defs {
		switch obj := obj.(type) {
		case *types.TypeName:
			// types are being handled by walking the AST
		case *types.Const:
			g.see(obj)
			fn := surroundingFunc(obj)
			if fn == nil && g.trackExportedIdentifier(obj) {
				// (1.4) packages use exported constants (unless in package main)
				g.use(obj, nil, "exported constant")
			}
			g.typ(obj.Type())
			g.seeAndUse(obj.Type(), obj, "constant type")
		}
	}

	// Find constants being used inside functions, find sinks in tests
	handledConsts := map[*ast.Ident]struct{}{}
	for _, fn := range pkg.InitialFunctions {
		g.see(fn)
		node := fn.Syntax()
		if node == nil {
			continue
		}
		ast.Inspect(node, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.Ident:
				obj, ok := pkg.TypesInfo.Uses[node]
				if !ok {
					return true
				}
				switch obj := obj.(type) {
				case *types.Const:
					g.seeAndUse(obj, fn, "used constant")
				}
			case *ast.AssignStmt:
				for _, expr := range node.Lhs {
					ident, ok := expr.(*ast.Ident)
					if !ok {
						continue
					}
					obj := pkg.TypesInfo.ObjectOf(ident)
					if obj == nil {
						continue
					}
					path := g.pkg.Prog.Fset.File(obj.Pos()).Name()
					if strings.HasSuffix(path, "_test.go") {
						if obj.Parent() != nil && obj.Parent().Parent() != nil && obj.Parent().Parent().Parent() == nil {
							// object's scope is the package, whose
							// parent is the file, whose parent is nil

							// (4.9) functions use package-level variables they assign to iff in tests (sinks for benchmarks)
							// (9.7) variable _reads_ use variables, writes do not, except in tests
							g.seeAndUse(obj, fn, "test sink")
						}
					}
				}
			}

			return true
		})
	}
	// Find constants being used in non-function contexts
	for ident, obj := range pkg.TypesInfo.Uses {
		_, ok := obj.(*types.Const)
		if !ok {
			continue
		}
		if _, ok := handledConsts[ident]; ok {
			continue
		}
		g.seeAndUse(obj, nil, "used constant")
	}

	var fn *ssa.Function
	pkg.Inspector.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.GenDecl)(nil)}, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.FuncDecl:
			fn = pkg.SSA.Prog.FuncValue(pkg.TypesInfo.ObjectOf(n.Name).(*types.Func))
			if fn != nil {
				g.see(fn)
			}
		case *ast.GenDecl:
			switch n.Tok {
			case token.CONST:
				groups := lintdsl.GroupSpecs(pkg.Fset, n.Specs)
				for _, specs := range groups {
					if len(specs) > 1 {
						cg := &ConstGroup{}
						g.see(cg)
						for _, spec := range specs {
							for _, name := range spec.(*ast.ValueSpec).Names {
								obj := pkg.TypesInfo.ObjectOf(name)
								// (10.1) const groups
								g.seeAndUse(obj, cg, "const group")
								g.use(cg, obj, "const group")
							}
						}
					}
				}
			case token.VAR:
				for _, spec := range n.Specs {
					v := spec.(*ast.ValueSpec)
					for _, name := range v.Names {
						T := pkg.TypesInfo.TypeOf(name)
						if fn != nil {
							g.seeAndUse(T, fn, "var decl")
						} else {
							g.seeAndUse(T, nil, "var decl")
						}
						g.typ(T)
					}
				}
			case token.TYPE:
				for _, spec := range n.Specs {
					// go/types doesn't provide a way to go from a
					// types.Named to the named type it was based on
					// (the t1 in type t2 t1). Therefore we walk the
					// AST and process GenDecls.
					//
					// (2.2) named types use the type they're based on
					v := spec.(*ast.TypeSpec)
					T := pkg.TypesInfo.TypeOf(v.Type)
					obj := pkg.TypesInfo.ObjectOf(v.Name)
					g.see(obj)
					g.see(T)
					g.use(T, obj, "type")
					g.typ(obj.Type())
					g.typ(T)

					if v.Assign != 0 {
						aliasFor := obj.(*types.TypeName).Type()
						// (2.3) named types use all their aliases. we can't easily track uses of aliases
						if isIrrelevant(aliasFor) {
							// We do not track the type this is an
							// alias for (for example builtins), so
							// just mark the alias used.
							//
							// FIXME(dh): what about aliases declared inside functions?
							g.use(obj, nil, "alias")
						} else {
							g.see(aliasFor)
							g.seeAndUse(obj, aliasFor, "alias")
						}
					}
				}
			}
		default:
			panic(fmt.Sprintf("unreachable: %T", n))
		}
	})

	for _, m := range g.pkg.Members {
		switch m := m.(type) {
		case *ssa.NamedConst:
			// nothing to do, we collect all constants from Defs
		case *ssa.Global:
			if m.Object() != nil {
				g.see(m.Object())
				if g.trackExportedIdentifier(m.Object()) {
					// (1.3) packages use exported variables (unless in package main)
					g.use(m.Object(), nil, "exported top-level variable")
				}
			}
		case *ssa.Function:
			g.see(m)
			if m.Name() == "init" {
				// (1.5) packages use init functions
				g.use(m, nil, "init function")
			}
			// This branch catches top-level functions, not methods.
			if m.Object() != nil && g.trackExportedIdentifier(m.Object()) {
				// (1.2) packages use exported functions (unless in package main)
				g.use(m, nil, "exported top-level function")
			}
			if m.Name() == "main" && g.pkg.Pkg.Name() == "main" {
				// (1.7) packages use the main function iff in the main package
				g.use(m, nil, "main function")
			}
			if g.pkg.Pkg.Path() == "runtime" && runtimeFuncs[m.Name()] {
				// (9.8) runtime functions that may be called from user code via the compiler
				g.use(m, nil, "runtime function")
			}
			if m.Syntax() != nil {
				doc := m.Syntax().(*ast.FuncDecl).Doc
				if doc != nil {
					for _, cmt := range doc.List {
						if strings.HasPrefix(cmt.Text, "//go:cgo_export_") {
							// (1.6) packages use functions exported to cgo
							g.use(m, nil, "cgo exported")
						}
					}
				}
			}
			g.function(m)
		case *ssa.Type:
			if m.Object() != nil {
				g.see(m.Object())
				if g.trackExportedIdentifier(m.Object()) {
					// (1.1) packages use exported named types (unless in package main)
					g.use(m.Object(), nil, "exported top-level type")
				}
			}
			g.typ(m.Type())
		default:
			panic(fmt.Sprintf("unreachable: %T", m))
		}
	}

	if !g.wholeProgram {
		// When not in whole program mode we process one package per
		// graph, which means g.seenTypes only contains types of
		// interest to us. In whole program mode, we're better off
		// processing all interfaces at once, globally, both for
		// performance reasons and because in whole program mode we
		// actually care about all interfaces, not just the subset
		// that has unexported methods.

		var ifaces []*types.Interface
		var notIfaces []types.Type

		g.seenTypes.Iterate(func(t types.Type, _ interface{}) {
			switch t := t.(type) {
			case *types.Interface:
				// OPT(dh): (8.1) we only need interfaces that have unexported methods
				ifaces = append(ifaces, t)
			default:
				if _, ok := t.Underlying().(*types.Interface); !ok {
					notIfaces = append(notIfaces, t)
				}
			}
		})

		// (8.0) handle interfaces
		for _, t := range notIfaces {
			ms := g.msCache.MethodSet(t)
			for _, iface := range ifaces {
				if sels, ok := g.implements(t, iface, ms); ok {
					for _, sel := range sels {
						g.useMethod(t, sel, t, "implements")
					}
				}
			}
		}
	}
}

func (g *Graph) useMethod(t types.Type, sel *types.Selection, by interface{}, reason string) {
	obj := sel.Obj()
	path := sel.Index()
	assert(obj != nil)
	if len(path) > 1 {
		base := lintdsl.Dereference(t).Underlying().(*types.Struct)
		for _, idx := range path[:len(path)-1] {
			next := base.Field(idx)
			// (6.3) structs use embedded fields that help implement interfaces
			g.seeAndUse(next, base, "provides method")
			base, _ = lintdsl.Dereference(next.Type()).Underlying().(*types.Struct)
		}
	}
	if fn := g.pkg.Prog.FuncValue(obj.(*types.Func)); fn != nil {
		// actual function
		g.seeAndUse(fn, by, reason)
	} else {
		// interface method
		g.seeAndUse(obj, by, reason)
	}
}

func (g *Graph) function(fn *ssa.Function) {
	if fn.Package() != nil && fn.Package() != g.pkg {
		return
	}
	if _, ok := g.seenFns[fn]; ok {
		return
	}
	g.seenFns[fn] = struct{}{}

	// (4.1) functions use all their arguments, return parameters and receivers
	g.seeAndUse(fn.Signature, fn, "function signature")
	g.signature(fn.Signature)
	g.instructions(fn)
	for _, anon := range fn.AnonFuncs {
		// (4.2) functions use anonymous functions defined beneath them
		g.seeAndUse(anon, fn, "anonymous function")
		g.function(anon)
	}
}

func (g *Graph) typ(t types.Type) {
	if g.seenTypes.At(t) != nil {
		return
	}
	if t, ok := t.(*types.Named); ok && t.Obj().Pkg() != nil {
		if t.Obj().Pkg() != g.pkg.Pkg {
			return
		}
	}
	g.seenTypes.Set(t, struct{}{})
	if isIrrelevant(t) {
		return
	}

	g.see(t)
	switch t := t.(type) {
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			g.see(t.Field(i))
			if t.Field(i).Exported() {
				// (6.2) structs use exported fields
				g.use(t.Field(i), t, "exported struct field")
			} else if t.Field(i).Name() == "_" {
				g.use(t.Field(i), t, "blank field")
			} else if isNoCopyType(t.Field(i).Type()) {
				// (6.1) structs use fields of type NoCopy sentinel
				g.use(t.Field(i), t, "NoCopy sentinel")
			}
			if t.Field(i).Anonymous() {
				// (e3) exported identifiers aren't automatically used.
				if !g.wholeProgram {
					// does the embedded field contribute exported methods to the method set?
					T := t.Field(i).Type()
					if _, ok := T.Underlying().(*types.Pointer); !ok {
						// An embedded field is addressable, so check
						// the pointer type to get the full method set
						T = types.NewPointer(T)
					}
					ms := g.msCache.MethodSet(T)
					for j := 0; j < ms.Len(); j++ {
						if ms.At(j).Obj().Exported() {
							// (6.4) structs use embedded fields that have exported methods (recursively)
							g.use(t.Field(i), t, "extends exported method set")
							break
						}
					}
				}

				seen := map[*types.Struct]struct{}{}
				var hasExportedField func(t types.Type) bool
				hasExportedField = func(T types.Type) bool {
					t, ok := lintdsl.Dereference(T).Underlying().(*types.Struct)
					if !ok {
						return false
					}
					if _, ok := seen[t]; ok {
						return false
					}
					seen[t] = struct{}{}
					for i := 0; i < t.NumFields(); i++ {
						field := t.Field(i)
						if field.Exported() {
							return true
						}
						if field.Embedded() && hasExportedField(field.Type()) {
							return true
						}
					}
					return false
				}
				// does the embedded field contribute exported fields?
				if hasExportedField(t.Field(i).Type()) {
					// (6.5) structs use embedded structs that have exported fields (recursively)
					g.use(t.Field(i), t, "extends exported fields")
				}

			}
			g.variable(t.Field(i))
		}
	case *types.Basic:
		// Nothing to do
	case *types.Named:
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Underlying(), t, "underlying type")
		g.seeAndUse(t.Obj(), t, "type name")
		g.seeAndUse(t, t.Obj(), "named type")

		for i := 0; i < t.NumMethods(); i++ {
			meth := g.pkg.Prog.FuncValue(t.Method(i))
			g.see(meth)
			// don't use trackExportedIdentifier here, we care about
			// all exported methods, even in package main or in tests.
			if meth.Object() != nil && meth.Object().Exported() && !g.wholeProgram {
				// (2.1) named types use exported methods
				g.use(meth, t, "exported method")
			}
			g.function(meth)
		}

		g.typ(t.Underlying())
	case *types.Slice:
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Elem(), t, "element type")
		g.typ(t.Elem())
	case *types.Map:
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Elem(), t, "element type")
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Key(), t, "key type")
		g.typ(t.Elem())
		g.typ(t.Key())
	case *types.Signature:
		g.signature(t)
	case *types.Interface:
		for i := 0; i < t.NumMethods(); i++ {
			m := t.Method(i)
			// (8.3) All interface methods are marked as used
			g.seeAndUse(m, t, "interface method")
			g.seeAndUse(m.Type().(*types.Signature), m, "signature")
			g.signature(m.Type().(*types.Signature))
		}
		for i := 0; i < t.NumEmbeddeds(); i++ {
			tt := t.EmbeddedType(i)
			// (8.4) All embedded interfaces are marked as used
			g.seeAndUse(tt, t, "embedded interface")
		}
	case *types.Array:
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Elem(), t, "element type")
		g.typ(t.Elem())
	case *types.Pointer:
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Elem(), t, "element type")
		g.typ(t.Elem())
	case *types.Chan:
		// (9.3) types use their underlying and element types
		g.seeAndUse(t.Elem(), t, "element type")
		g.typ(t.Elem())
	case *types.Tuple:
		for i := 0; i < t.Len(); i++ {
			// (9.3) types use their underlying and element types
			g.seeAndUse(t.At(i), t, "tuple element")
			g.variable(t.At(i))
		}
	default:
		panic(fmt.Sprintf("unreachable: %T", t))
	}
}

func (g *Graph) variable(v *types.Var) {
	// (9.2) variables use their types
	g.seeAndUse(v.Type(), v, "variable type")
	g.typ(v.Type())
}

func (g *Graph) signature(sig *types.Signature) {
	if sig.Recv() != nil {
		g.seeAndUse(sig.Recv(), sig, "receiver")
		g.variable(sig.Recv())
	}
	for i := 0; i < sig.Params().Len(); i++ {
		param := sig.Params().At(i)
		g.seeAndUse(param, sig, "function argument")
		g.variable(param)
	}
	for i := 0; i < sig.Results().Len(); i++ {
		param := sig.Results().At(i)
		g.seeAndUse(param, sig, "function result")
		g.variable(param)
	}
}

func (g *Graph) instructions(fn *ssa.Function) {
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			ops := instr.Operands(nil)
			switch instr.(type) {
			case *ssa.Store:
				// (9.7) variable _reads_ use variables, writes do not
				ops = ops[1:]
			case *ssa.DebugRef:
				ops = nil
			}
			for _, arg := range ops {
				walkPhi(*arg, func(v ssa.Value) {
					switch v := v.(type) {
					case *ssa.Function:
						// (4.3) functions use closures and bound methods.
						// (4.5) functions use functions they call
						// (9.5) instructions use their operands
						// (4.4) functions use functions they return. we assume that someone else will call the returned function
						g.seeAndUse(v, fn, "instruction operand")
						g.function(v)
					case *ssa.Const:
						// (9.6) instructions use their operands' types
						g.seeAndUse(v.Type(), fn, "constant's type")
						g.typ(v.Type())
					case *ssa.Global:
						if v.Object() != nil {
							// (9.5) instructions use their operands
							g.seeAndUse(v.Object(), fn, "instruction operand")
						}
					}
				})
			}
			if v, ok := instr.(ssa.Value); ok {
				if _, ok := v.(*ssa.Range); !ok {
					// See https://github.com/golang/go/issues/19670

					// (4.8) instructions use their types
					// (9.4) conversions use the type they convert to
					g.seeAndUse(v.Type(), fn, "instruction")
					g.typ(v.Type())
				}
			}
			switch instr := instr.(type) {
			case *ssa.Field:
				st := instr.X.Type().Underlying().(*types.Struct)
				field := st.Field(instr.Field)
				// (4.7) functions use fields they access
				g.seeAndUse(field, fn, "field access")
			case *ssa.FieldAddr:
				st := lintdsl.Dereference(instr.X.Type()).Underlying().(*types.Struct)
				field := st.Field(instr.Field)
				// (4.7) functions use fields they access
				g.seeAndUse(field, fn, "field access")
			case *ssa.Store:
				// nothing to do, handled generically by operands
			case *ssa.Call:
				c := instr.Common()
				if !c.IsInvoke() {
					// handled generically as an instruction operand

					if g.wholeProgram {
						// (e3) special case known reflection-based method callers
						switch lintdsl.CallName(c) {
						case "net/rpc.Register", "net/rpc.RegisterName", "(*net/rpc.Server).Register", "(*net/rpc.Server).RegisterName":
							var arg ssa.Value
							switch lintdsl.CallName(c) {
							case "net/rpc.Register":
								arg = c.Args[0]
							case "net/rpc.RegisterName":
								arg = c.Args[1]
							case "(*net/rpc.Server).Register":
								arg = c.Args[1]
							case "(*net/rpc.Server).RegisterName":
								arg = c.Args[2]
							}
							walkPhi(arg, func(v ssa.Value) {
								if v, ok := v.(*ssa.MakeInterface); ok {
									walkPhi(v.X, func(vv ssa.Value) {
										ms := g.msCache.MethodSet(vv.Type())
										for i := 0; i < ms.Len(); i++ {
											if ms.At(i).Obj().Exported() {
												g.useMethod(vv.Type(), ms.At(i), fn, "net/rpc.Register")
											}
										}
									})
								}
							})
						}
					}
				} else {
					// (4.5) functions use functions/interface methods they call
					g.seeAndUse(c.Method, fn, "interface call")
				}
			case *ssa.Return:
				// nothing to do, handled generically by operands
			case *ssa.ChangeType:
				// conversion type handled generically

				s1, ok1 := lintdsl.Dereference(instr.Type()).Underlying().(*types.Struct)
				s2, ok2 := lintdsl.Dereference(instr.X.Type()).Underlying().(*types.Struct)
				if ok1 && ok2 {
					// Converting between two structs. The fields are
					// relevant for the conversion, but only if the
					// fields are also used outside of the conversion.
					// Mark fields as used by each other.

					assert(s1.NumFields() == s2.NumFields())
					for i := 0; i < s1.NumFields(); i++ {
						g.see(s1.Field(i))
						g.see(s2.Field(i))
						// (5.1) when converting between two equivalent structs, the fields in
						// either struct use each other. the fields are relevant for the
						// conversion, but only if the fields are also accessed outside the
						// conversion.
						g.seeAndUse(s1.Field(i), s2.Field(i), "struct conversion")
						g.seeAndUse(s2.Field(i), s1.Field(i), "struct conversion")
					}
				}
			case *ssa.MakeInterface:
				// nothing to do, handled generically by operands
			case *ssa.Slice:
				// nothing to do, handled generically by operands
			case *ssa.RunDefers:
				// nothing to do, the deferred functions are already marked use by defering them.
			case *ssa.Convert:
				// to unsafe.Pointer
				if typ, ok := instr.Type().(*types.Basic); ok && typ.Kind() == types.UnsafePointer {
					if ptr, ok := instr.X.Type().Underlying().(*types.Pointer); ok {
						if st, ok := ptr.Elem().Underlying().(*types.Struct); ok {
							for i := 0; i < st.NumFields(); i++ {
								// (5.2) when converting to or from unsafe.Pointer, mark all fields as used.
								g.seeAndUse(st.Field(i), fn, "unsafe conversion")
							}
						}
					}
				}
				// from unsafe.Pointer
				if typ, ok := instr.X.Type().(*types.Basic); ok && typ.Kind() == types.UnsafePointer {
					if ptr, ok := instr.Type().Underlying().(*types.Pointer); ok {
						if st, ok := ptr.Elem().Underlying().(*types.Struct); ok {
							for i := 0; i < st.NumFields(); i++ {
								// (5.2) when converting to or from unsafe.Pointer, mark all fields as used.
								g.seeAndUse(st.Field(i), fn, "unsafe conversion")
							}
						}
					}
				}
			case *ssa.TypeAssert:
				// nothing to do, handled generically by instruction
				// type (possibly a tuple, which contains the asserted
				// to type). redundantly handled by the type of
				// ssa.Extract, too
			case *ssa.MakeClosure:
				// nothing to do, handled generically by operands
			case *ssa.Alloc:
				// nothing to do
			case *ssa.UnOp:
				// nothing to do
			case *ssa.BinOp:
				// nothing to do
			case *ssa.If:
				// nothing to do
			case *ssa.Jump:
				// nothing to do
			case *ssa.IndexAddr:
				// nothing to do
			case *ssa.Extract:
				// nothing to do
			case *ssa.Panic:
				// nothing to do
			case *ssa.DebugRef:
				// nothing to do
			case *ssa.BlankStore:
				// nothing to do
			case *ssa.Phi:
				// nothing to do
			case *ssa.MakeMap:
				// nothing to do
			case *ssa.MapUpdate:
				// nothing to do
			case *ssa.Lookup:
				// nothing to do
			case *ssa.MakeSlice:
				// nothing to do
			case *ssa.Send:
				// nothing to do
			case *ssa.MakeChan:
				// nothing to do
			case *ssa.Range:
				// nothing to do
			case *ssa.Next:
				// nothing to do
			case *ssa.Index:
				// nothing to do
			case *ssa.Select:
				// nothing to do
			case *ssa.ChangeInterface:
				// nothing to do
			case *ssa.Go:
				// nothing to do, handled generically by operands
			case *ssa.Defer:
				// nothing to do, handled generically by operands
			default:
				panic(fmt.Sprintf("unreachable: %T", instr))
			}
		}
	}
}

// isNoCopyType reports whether a type represents the NoCopy sentinel
// type. The NoCopy type is a named struct with no fields and exactly
// one method `func Lock()` that is empty.
//
// FIXME(dh): currently we're not checking that the function body is
// empty.
func isNoCopyType(typ types.Type) bool {
	st, ok := typ.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	if st.NumFields() != 0 {
		return false
	}

	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}
	if named.NumMethods() != 1 {
		return false
	}
	meth := named.Method(0)
	if meth.Name() != "Lock" {
		return false
	}
	sig := meth.Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
		return false
	}
	return true
}

func walkPhi(v ssa.Value, fn func(v ssa.Value)) {
	phi, ok := v.(*ssa.Phi)
	if !ok {
		fn(v)
		return
	}

	seen := map[ssa.Value]struct{}{}
	var impl func(v *ssa.Phi)
	impl = func(v *ssa.Phi) {
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		for _, e := range v.Edges {
			if ev, ok := e.(*ssa.Phi); ok {
				impl(ev)
			} else {
				fn(e)
			}
		}
	}
	impl(phi)
}

func interfacesFromExportData(pkg *types.Package) []*types.Interface {
	var out []*types.Interface
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		out = append(out, interfacesFromObject(obj)...)
	}
	return out
}

func interfacesFromObject(obj types.Object) []*types.Interface {
	var out []*types.Interface
	switch obj := obj.(type) {
	case *types.Func:
		sig := obj.Type().(*types.Signature)
		for i := 0; i < sig.Results().Len(); i++ {
			out = append(out, interfacesFromObject(sig.Results().At(i))...)
		}
		for i := 0; i < sig.Params().Len(); i++ {
			out = append(out, interfacesFromObject(sig.Params().At(i))...)
		}
	case *types.TypeName:
		if named, ok := obj.Type().(*types.Named); ok {
			for i := 0; i < named.NumMethods(); i++ {
				out = append(out, interfacesFromObject(named.Method(i))...)
			}

			if iface, ok := named.Underlying().(*types.Interface); ok {
				out = append(out, iface)
			}
		}
	case *types.Var:
		// No call to Underlying here. We want unnamed interfaces
		// only. Named interfaces are gotten directly from the
		// package's scope.
		if iface, ok := obj.Type().(*types.Interface); ok {
			out = append(out, iface)
		}
	case *types.Const:
	case *types.Builtin:
	default:
		panic(fmt.Sprintf("unhandled type: %T", obj))
	}
	return out
}
