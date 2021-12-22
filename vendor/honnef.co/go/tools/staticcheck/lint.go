// Package staticcheck contains a linter for Go source code.
package staticcheck // import "honnef.co/go/tools/staticcheck"

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	htmltemplate "html/template"
	"net/http"
	"reflect"
	"regexp"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"sync"
	texttemplate "text/template"
	"unicode"

	. "honnef.co/go/tools/arg"
	"honnef.co/go/tools/deprecated"
	"honnef.co/go/tools/functions"
	"honnef.co/go/tools/internal/sharedcheck"
	"honnef.co/go/tools/lint"
	. "honnef.co/go/tools/lint/lintdsl"
	"honnef.co/go/tools/printf"
	"honnef.co/go/tools/ssa"
	"honnef.co/go/tools/ssautil"
	"honnef.co/go/tools/staticcheck/vrp"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

func validRegexp(call *Call) {
	arg := call.Args[0]
	err := ValidateRegexp(arg.Value)
	if err != nil {
		arg.Invalid(err.Error())
	}
}

type runeSlice []rune

func (rs runeSlice) Len() int               { return len(rs) }
func (rs runeSlice) Less(i int, j int) bool { return rs[i] < rs[j] }
func (rs runeSlice) Swap(i int, j int)      { rs[i], rs[j] = rs[j], rs[i] }

func utf8Cutset(call *Call) {
	arg := call.Args[1]
	if InvalidUTF8(arg.Value) {
		arg.Invalid(MsgInvalidUTF8)
	}
}

func uniqueCutset(call *Call) {
	arg := call.Args[1]
	if !UniqueStringCutset(arg.Value) {
		arg.Invalid(MsgNonUniqueCutset)
	}
}

func unmarshalPointer(name string, arg int) CallCheck {
	return func(call *Call) {
		if !Pointer(call.Args[arg].Value) {
			call.Args[arg].Invalid(fmt.Sprintf("%s expects to unmarshal into a pointer, but the provided value is not a pointer", name))
		}
	}
}

func pointlessIntMath(call *Call) {
	if ConvertedFromInt(call.Args[0].Value) {
		call.Invalid(fmt.Sprintf("calling %s on a converted integer is pointless", CallName(call.Instr.Common())))
	}
}

func checkValidHostPort(arg int) CallCheck {
	return func(call *Call) {
		if !ValidHostPort(call.Args[arg].Value) {
			call.Args[arg].Invalid(MsgInvalidHostPort)
		}
	}
}

var (
	checkRegexpRules = map[string]CallCheck{
		"regexp.MustCompile": validRegexp,
		"regexp.Compile":     validRegexp,
		"regexp.Match":       validRegexp,
		"regexp.MatchReader": validRegexp,
		"regexp.MatchString": validRegexp,
	}

	checkTimeParseRules = map[string]CallCheck{
		"time.Parse": func(call *Call) {
			arg := call.Args[Arg("time.Parse.layout")]
			err := ValidateTimeLayout(arg.Value)
			if err != nil {
				arg.Invalid(err.Error())
			}
		},
	}

	checkEncodingBinaryRules = map[string]CallCheck{
		"encoding/binary.Write": func(call *Call) {
			arg := call.Args[Arg("encoding/binary.Write.data")]
			if !CanBinaryMarshal(call.Job, arg.Value) {
				arg.Invalid(fmt.Sprintf("value of type %s cannot be used with binary.Write", arg.Value.Value.Type()))
			}
		},
	}

	checkURLsRules = map[string]CallCheck{
		"net/url.Parse": func(call *Call) {
			arg := call.Args[Arg("net/url.Parse.rawurl")]
			err := ValidateURL(arg.Value)
			if err != nil {
				arg.Invalid(err.Error())
			}
		},
	}

	checkSyncPoolValueRules = map[string]CallCheck{
		"(*sync.Pool).Put": func(call *Call) {
			arg := call.Args[Arg("(*sync.Pool).Put.x")]
			typ := arg.Value.Value.Type()
			if !IsPointerLike(typ) {
				arg.Invalid("argument should be pointer-like to avoid allocations")
			}
		},
	}

	checkRegexpFindAllRules = map[string]CallCheck{
		"(*regexp.Regexp).FindAll":                    RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllIndex":               RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllString":              RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllStringIndex":         RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllStringSubmatch":      RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllStringSubmatchIndex": RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllSubmatch":            RepeatZeroTimes("a FindAll method", 1),
		"(*regexp.Regexp).FindAllSubmatchIndex":       RepeatZeroTimes("a FindAll method", 1),
	}

	checkUTF8CutsetRules = map[string]CallCheck{
		"strings.IndexAny":     utf8Cutset,
		"strings.LastIndexAny": utf8Cutset,
		"strings.ContainsAny":  utf8Cutset,
		"strings.Trim":         utf8Cutset,
		"strings.TrimLeft":     utf8Cutset,
		"strings.TrimRight":    utf8Cutset,
	}

	checkUniqueCutsetRules = map[string]CallCheck{
		"strings.Trim":      uniqueCutset,
		"strings.TrimLeft":  uniqueCutset,
		"strings.TrimRight": uniqueCutset,
	}

	checkUnmarshalPointerRules = map[string]CallCheck{
		"encoding/xml.Unmarshal":                unmarshalPointer("xml.Unmarshal", 1),
		"(*encoding/xml.Decoder).Decode":        unmarshalPointer("Decode", 0),
		"(*encoding/xml.Decoder).DecodeElement": unmarshalPointer("DecodeElement", 0),
		"encoding/json.Unmarshal":               unmarshalPointer("json.Unmarshal", 1),
		"(*encoding/json.Decoder).Decode":       unmarshalPointer("Decode", 0),
	}

	checkUnbufferedSignalChanRules = map[string]CallCheck{
		"os/signal.Notify": func(call *Call) {
			arg := call.Args[Arg("os/signal.Notify.c")]
			if UnbufferedChannel(arg.Value) {
				arg.Invalid("the channel used with signal.Notify should be buffered")
			}
		},
	}

	checkMathIntRules = map[string]CallCheck{
		"math.Ceil":  pointlessIntMath,
		"math.Floor": pointlessIntMath,
		"math.IsNaN": pointlessIntMath,
		"math.Trunc": pointlessIntMath,
		"math.IsInf": pointlessIntMath,
	}

	checkStringsReplaceZeroRules = map[string]CallCheck{
		"strings.Replace": RepeatZeroTimes("strings.Replace", 3),
		"bytes.Replace":   RepeatZeroTimes("bytes.Replace", 3),
	}

	checkListenAddressRules = map[string]CallCheck{
		"net/http.ListenAndServe":    checkValidHostPort(0),
		"net/http.ListenAndServeTLS": checkValidHostPort(0),
	}

	checkBytesEqualIPRules = map[string]CallCheck{
		"bytes.Equal": func(call *Call) {
			if ConvertedFrom(call.Args[Arg("bytes.Equal.a")].Value, "net.IP") &&
				ConvertedFrom(call.Args[Arg("bytes.Equal.b")].Value, "net.IP") {
				call.Invalid("use net.IP.Equal to compare net.IPs, not bytes.Equal")
			}
		},
	}

	checkRegexpMatchLoopRules = map[string]CallCheck{
		"regexp.Match":       loopedRegexp("regexp.Match"),
		"regexp.MatchReader": loopedRegexp("regexp.MatchReader"),
		"regexp.MatchString": loopedRegexp("regexp.MatchString"),
	}

	checkNoopMarshal = map[string]CallCheck{
		// TODO(dh): should we really flag XML? Even an empty struct
		// produces a non-zero amount of data, namely its type name.
		// Let's see if we encounter any false positives.
		//
		// Also, should we flag gob?
		"encoding/json.Marshal":           checkNoopMarshalImpl(Arg("json.Marshal.v"), "MarshalJSON", "MarshalText"),
		"encoding/xml.Marshal":            checkNoopMarshalImpl(Arg("xml.Marshal.v"), "MarshalXML", "MarshalText"),
		"(*encoding/json.Encoder).Encode": checkNoopMarshalImpl(Arg("(*encoding/json.Encoder).Encode.v"), "MarshalJSON", "MarshalText"),
		"(*encoding/xml.Encoder).Encode":  checkNoopMarshalImpl(Arg("(*encoding/xml.Encoder).Encode.v"), "MarshalXML", "MarshalText"),

		"encoding/json.Unmarshal":         checkNoopMarshalImpl(Arg("json.Unmarshal.v"), "UnmarshalJSON", "UnmarshalText"),
		"encoding/xml.Unmarshal":          checkNoopMarshalImpl(Arg("xml.Unmarshal.v"), "UnmarshalXML", "UnmarshalText"),
		"(*encoding/json.Decoder).Decode": checkNoopMarshalImpl(Arg("(*encoding/json.Decoder).Decode.v"), "UnmarshalJSON", "UnmarshalText"),
		"(*encoding/xml.Decoder).Decode":  checkNoopMarshalImpl(Arg("(*encoding/xml.Decoder).Decode.v"), "UnmarshalXML", "UnmarshalText"),
	}

	checkUnsupportedMarshal = map[string]CallCheck{
		"encoding/json.Marshal":           checkUnsupportedMarshalImpl(Arg("json.Marshal.v"), "json", "MarshalJSON", "MarshalText"),
		"encoding/xml.Marshal":            checkUnsupportedMarshalImpl(Arg("xml.Marshal.v"), "xml", "MarshalXML", "MarshalText"),
		"(*encoding/json.Encoder).Encode": checkUnsupportedMarshalImpl(Arg("(*encoding/json.Encoder).Encode.v"), "json", "MarshalJSON", "MarshalText"),
		"(*encoding/xml.Encoder).Encode":  checkUnsupportedMarshalImpl(Arg("(*encoding/xml.Encoder).Encode.v"), "xml", "MarshalXML", "MarshalText"),
	}

	checkAtomicAlignment = map[string]CallCheck{
		"sync/atomic.AddInt64":             checkAtomicAlignmentImpl,
		"sync/atomic.AddUint64":            checkAtomicAlignmentImpl,
		"sync/atomic.CompareAndSwapInt64":  checkAtomicAlignmentImpl,
		"sync/atomic.CompareAndSwapUint64": checkAtomicAlignmentImpl,
		"sync/atomic.LoadInt64":            checkAtomicAlignmentImpl,
		"sync/atomic.LoadUint64":           checkAtomicAlignmentImpl,
		"sync/atomic.StoreInt64":           checkAtomicAlignmentImpl,
		"sync/atomic.StoreUint64":          checkAtomicAlignmentImpl,
		"sync/atomic.SwapInt64":            checkAtomicAlignmentImpl,
		"sync/atomic.SwapUint64":           checkAtomicAlignmentImpl,
	}

	// TODO(dh): detect printf wrappers
	checkPrintfRules = map[string]CallCheck{
		"fmt.Errorf":  func(call *Call) { checkPrintfCall(call, 0, 1) },
		"fmt.Printf":  func(call *Call) { checkPrintfCall(call, 0, 1) },
		"fmt.Sprintf": func(call *Call) { checkPrintfCall(call, 0, 1) },
		"fmt.Fprintf": func(call *Call) { checkPrintfCall(call, 1, 2) },
	}
)

func checkPrintfCall(call *Call, fIdx, vIdx int) {
	f := call.Args[fIdx]
	var args []ssa.Value
	switch v := call.Args[vIdx].Value.Value.(type) {
	case *ssa.Slice:
		var ok bool
		args, ok = ssautil.Vararg(v)
		if !ok {
			// We don't know what the actual arguments to the function are
			return
		}
	case *ssa.Const:
		// nil, i.e. no arguments
	default:
		// We don't know what the actual arguments to the function are
		return
	}
	checkPrintfCallImpl(call, f.Value.Value, args)
}

type verbFlag int

const (
	isInt verbFlag = 1 << iota
	isBool
	isFP
	isString
	isPointer
	isPseudoPointer
	isSlice
	isAny
	noRecurse
)

var verbs = [...]verbFlag{
	'b': isPseudoPointer | isInt | isFP,
	'c': isInt,
	'd': isPseudoPointer | isInt,
	'e': isFP,
	'E': isFP,
	'f': isFP,
	'F': isFP,
	'g': isFP,
	'G': isFP,
	'o': isPseudoPointer | isInt,
	'p': isSlice | isPointer | noRecurse,
	'q': isInt | isString,
	's': isString,
	't': isBool,
	'T': isAny,
	'U': isInt,
	'v': isAny,
	'X': isPseudoPointer | isInt | isString,
	'x': isPseudoPointer | isInt | isString,
}

func checkPrintfCallImpl(call *Call, f ssa.Value, args []ssa.Value) {
	var elem func(T types.Type, verb rune) ([]types.Type, bool)
	elem = func(T types.Type, verb rune) ([]types.Type, bool) {
		if verbs[verb]&noRecurse != 0 {
			return []types.Type{T}, false
		}
		switch T := T.(type) {
		case *types.Slice:
			if verbs[verb]&isSlice != 0 {
				return []types.Type{T}, false
			}
			if verbs[verb]&isString != 0 && IsType(T.Elem().Underlying(), "byte") {
				return []types.Type{T}, false
			}
			return []types.Type{T.Elem()}, true
		case *types.Map:
			key := T.Key()
			val := T.Elem()
			return []types.Type{key, val}, true
		case *types.Struct:
			out := make([]types.Type, 0, T.NumFields())
			for i := 0; i < T.NumFields(); i++ {
				out = append(out, T.Field(i).Type())
			}
			return out, true
		case *types.Array:
			return []types.Type{T.Elem()}, true
		default:
			return []types.Type{T}, false
		}
	}
	isInfo := func(T types.Type, info types.BasicInfo) bool {
		basic, ok := T.Underlying().(*types.Basic)
		return ok && basic.Info()&info != 0
	}

	isStringer := func(T types.Type, ms *types.MethodSet) bool {
		sel := ms.Lookup(nil, "String")
		if sel == nil {
			return false
		}
		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			// should be unreachable
			return false
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 0 {
			return false
		}
		if sig.Results().Len() != 1 {
			return false
		}
		if !IsType(sig.Results().At(0).Type(), "string") {
			return false
		}
		return true
	}
	isError := func(T types.Type, ms *types.MethodSet) bool {
		sel := ms.Lookup(nil, "Error")
		if sel == nil {
			return false
		}
		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			// should be unreachable
			return false
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 0 {
			return false
		}
		if sig.Results().Len() != 1 {
			return false
		}
		if !IsType(sig.Results().At(0).Type(), "string") {
			return false
		}
		return true
	}

	isFormatter := func(T types.Type, ms *types.MethodSet) bool {
		sel := ms.Lookup(nil, "Format")
		if sel == nil {
			return false
		}
		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			// should be unreachable
			return false
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 2 {
			return false
		}
		// TODO(dh): check the types of the arguments for more
		// precision
		if sig.Results().Len() != 0 {
			return false
		}
		return true
	}

	seen := map[types.Type]bool{}
	var checkType func(verb rune, T types.Type, top bool) bool
	checkType = func(verb rune, T types.Type, top bool) bool {
		if top {
			for k := range seen {
				delete(seen, k)
			}
		}
		if seen[T] {
			return true
		}
		seen[T] = true
		if int(verb) >= len(verbs) {
			// Unknown verb
			return true
		}

		flags := verbs[verb]
		if flags == 0 {
			// Unknown verb
			return true
		}

		ms := types.NewMethodSet(T)
		if isFormatter(T, ms) {
			// the value is responsible for formatting itself
			return true
		}

		if flags&isString != 0 && (isStringer(T, ms) || isError(T, ms)) {
			// Check for stringer early because we're about to dereference
			return true
		}

		T = T.Underlying()
		if flags&(isPointer|isPseudoPointer) == 0 && top {
			T = Dereference(T)
		}
		if flags&isPseudoPointer != 0 && top {
			t := Dereference(T)
			if _, ok := t.Underlying().(*types.Struct); ok {
				T = t
			}
		}

		if _, ok := T.(*types.Interface); ok {
			// We don't know what's in the interface
			return true
		}

		var info types.BasicInfo
		if flags&isInt != 0 {
			info |= types.IsInteger
		}
		if flags&isBool != 0 {
			info |= types.IsBoolean
		}
		if flags&isFP != 0 {
			info |= types.IsFloat | types.IsComplex
		}
		if flags&isString != 0 {
			info |= types.IsString
		}

		if info != 0 && isInfo(T, info) {
			return true
		}

		if flags&isString != 0 && (IsType(T, "[]byte") || isStringer(T, ms) || isError(T, ms)) {
			return true
		}

		if flags&isPointer != 0 && IsPointerLike(T) {
			return true
		}
		if flags&isPseudoPointer != 0 {
			switch U := T.Underlying().(type) {
			case *types.Pointer:
				if !top {
					return true
				}

				if _, ok := U.Elem().Underlying().(*types.Struct); !ok {
					return true
				}
			case *types.Chan, *types.Signature:
				return true
			}
		}

		if flags&isSlice != 0 {
			if _, ok := T.(*types.Slice); ok {
				return true
			}
		}

		if flags&isAny != 0 {
			return true
		}

		elems, ok := elem(T.Underlying(), verb)
		if !ok {
			return false
		}
		for _, elem := range elems {
			if !checkType(verb, elem, false) {
				return false
			}
		}

		return true
	}

	k, ok := f.(*ssa.Const)
	if !ok {
		return
	}
	actions, err := printf.Parse(constant.StringVal(k.Value))
	if err != nil {
		call.Invalid("couldn't parse format string")
		return
	}

	ptr := 1
	hasExplicit := false

	checkStar := func(verb printf.Verb, star printf.Argument) bool {
		if star, ok := star.(printf.Star); ok {
			idx := 0
			if star.Index == -1 {
				idx = ptr
				ptr++
			} else {
				hasExplicit = true
				idx = star.Index
				ptr = star.Index + 1
			}
			if idx == 0 {
				call.Invalid(fmt.Sprintf("Printf format %s reads invalid arg 0; indices are 1-based", verb.Raw))
				return false
			}
			if idx > len(args) {
				call.Invalid(
					fmt.Sprintf("Printf format %s reads arg #%d, but call has only %d args",
						verb.Raw, idx, len(args)))
				return false
			}
			if arg, ok := args[idx-1].(*ssa.MakeInterface); ok {
				if !isInfo(arg.X.Type(), types.IsInteger) {
					call.Invalid(fmt.Sprintf("Printf format %s reads non-int arg #%d as argument of *", verb.Raw, idx))
				}
			}
		}
		return true
	}

	// We only report one problem per format string. Making a
	// mistake with an index tends to invalidate all future
	// implicit indices.
	for _, action := range actions {
		verb, ok := action.(printf.Verb)
		if !ok {
			continue
		}

		if !checkStar(verb, verb.Width) || !checkStar(verb, verb.Precision) {
			return
		}

		off := ptr
		if verb.Value != -1 {
			hasExplicit = true
			off = verb.Value
		}
		if off > len(args) {
			call.Invalid(
				fmt.Sprintf("Printf format %s reads arg #%d, but call has only %d args",
					verb.Raw, off, len(args)))
			return
		} else if verb.Value == 0 && verb.Letter != '%' {
			call.Invalid(fmt.Sprintf("Printf format %s reads invalid arg 0; indices are 1-based", verb.Raw))
			return
		} else if off != 0 {
			arg, ok := args[off-1].(*ssa.MakeInterface)
			if ok {
				if !checkType(verb.Letter, arg.X.Type(), true) {
					call.Invalid(fmt.Sprintf("Printf format %s has arg #%d of wrong type %s",
						verb.Raw, ptr, args[ptr-1].(*ssa.MakeInterface).X.Type()))
					return
				}
			}
		}

		switch verb.Value {
		case -1:
			// Consume next argument
			ptr++
		case 0:
			// Don't consume any arguments
		default:
			ptr = verb.Value + 1
		}
	}

	if !hasExplicit && ptr <= len(args) {
		call.Invalid(fmt.Sprintf("Printf call needs %d args but has %d args", ptr-1, len(args)))
	}
}

func checkAtomicAlignmentImpl(call *Call) {
	sizes := call.Job.Pkg.TypesSizes
	if sizes.Sizeof(types.Typ[types.Uintptr]) != 4 {
		// Not running on a 32-bit platform
		return
	}
	v, ok := call.Args[0].Value.Value.(*ssa.FieldAddr)
	if !ok {
		// TODO(dh): also check indexing into arrays and slices
		return
	}
	T := v.X.Type().Underlying().(*types.Pointer).Elem().Underlying().(*types.Struct)
	fields := make([]*types.Var, 0, T.NumFields())
	for i := 0; i < T.NumFields() && i <= v.Field; i++ {
		fields = append(fields, T.Field(i))
	}

	off := sizes.Offsetsof(fields)[v.Field]
	if off%8 != 0 {
		msg := fmt.Sprintf("address of non 64-bit aligned field %s passed to %s",
			T.Field(v.Field).Name(),
			CallName(call.Instr.Common()))
		call.Invalid(msg)
	}
}

func checkNoopMarshalImpl(argN int, meths ...string) CallCheck {
	return func(call *Call) {
		arg := call.Args[argN]
		T := arg.Value.Value.Type()
		Ts, ok := Dereference(T).Underlying().(*types.Struct)
		if !ok {
			return
		}
		if Ts.NumFields() == 0 {
			return
		}
		fields := FlattenFields(Ts)
		for _, field := range fields {
			if field.Var.Exported() {
				return
			}
		}
		// OPT(dh): we could use a method set cache here
		ms := types.NewMethodSet(T)
		// TODO(dh): we're not checking the signature, which can cause false negatives.
		// This isn't a huge problem, however, since vet complains about incorrect signatures.
		for _, meth := range meths {
			if ms.Lookup(nil, meth) != nil {
				return
			}
		}
		arg.Invalid("struct doesn't have any exported fields, nor custom marshaling")
	}
}

func checkUnsupportedMarshalImpl(argN int, tag string, meths ...string) CallCheck {
	// TODO(dh): flag slices and maps of unsupported types
	return func(call *Call) {
		arg := call.Args[argN]
		T := arg.Value.Value.Type()
		Ts, ok := Dereference(T).Underlying().(*types.Struct)
		if !ok {
			return
		}
		// OPT(dh): we could use a method set cache here
		ms := types.NewMethodSet(T)
		// TODO(dh): we're not checking the signature, which can cause false negatives.
		// This isn't a huge problem, however, since vet complains about incorrect signatures.
		for _, meth := range meths {
			if ms.Lookup(nil, meth) != nil {
				return
			}
		}
		fields := FlattenFields(Ts)
		for _, field := range fields {
			if !(field.Var.Exported()) {
				continue
			}
			if reflect.StructTag(field.Tag).Get(tag) == "-" {
				continue
			}
			// OPT(dh): we could use a method set cache here
			ms := types.NewMethodSet(field.Var.Type())
			// TODO(dh): we're not checking the signature, which can cause false negatives.
			// This isn't a huge problem, however, since vet complains about incorrect signatures.
			for _, meth := range meths {
				if ms.Lookup(nil, meth) != nil {
					return
				}
			}
			switch field.Var.Type().Underlying().(type) {
			case *types.Chan, *types.Signature:
				arg.Invalid(fmt.Sprintf("trying to marshal chan or func value, field %s", fieldPath(T, field.Path)))
			}
		}
	}
}

func fieldPath(start types.Type, indices []int) string {
	p := start.String()
	for _, idx := range indices {
		field := Dereference(start).Underlying().(*types.Struct).Field(idx)
		start = field.Type()
		p += "." + field.Name()
	}
	return p
}

type Checker struct {
	CheckGenerated bool
	funcDescs      *functions.Descriptions
	deprecatedPkgs map[*types.Package]string
	deprecatedObjs map[types.Object]string
}

func NewChecker() *Checker {
	return &Checker{}
}

func (*Checker) Name() string   { return "staticcheck" }
func (*Checker) Prefix() string { return "SA" }

func (c *Checker) Checks() []lint.Check {
	return []lint.Check{
		{ID: "SA1000", FilterGenerated: false, Fn: c.callChecker(checkRegexpRules), Doc: docSA1000},
		{ID: "SA1001", FilterGenerated: false, Fn: c.CheckTemplate, Doc: docSA1001},
		{ID: "SA1002", FilterGenerated: false, Fn: c.callChecker(checkTimeParseRules), Doc: docSA1002},
		{ID: "SA1003", FilterGenerated: false, Fn: c.callChecker(checkEncodingBinaryRules), Doc: docSA1003},
		{ID: "SA1004", FilterGenerated: false, Fn: c.CheckTimeSleepConstant, Doc: docSA1004},
		{ID: "SA1005", FilterGenerated: false, Fn: c.CheckExec, Doc: docSA1005},
		{ID: "SA1006", FilterGenerated: false, Fn: c.CheckUnsafePrintf, Doc: docSA1006},
		{ID: "SA1007", FilterGenerated: false, Fn: c.callChecker(checkURLsRules), Doc: docSA1007},
		{ID: "SA1008", FilterGenerated: false, Fn: c.CheckCanonicalHeaderKey, Doc: docSA1008},
		{ID: "SA1010", FilterGenerated: false, Fn: c.callChecker(checkRegexpFindAllRules), Doc: docSA1010},
		{ID: "SA1011", FilterGenerated: false, Fn: c.callChecker(checkUTF8CutsetRules), Doc: docSA1011},
		{ID: "SA1012", FilterGenerated: false, Fn: c.CheckNilContext, Doc: docSA1012},
		{ID: "SA1013", FilterGenerated: false, Fn: c.CheckSeeker, Doc: docSA1013},
		{ID: "SA1014", FilterGenerated: false, Fn: c.callChecker(checkUnmarshalPointerRules), Doc: docSA1014},
		{ID: "SA1015", FilterGenerated: false, Fn: c.CheckLeakyTimeTick, Doc: docSA1015},
		{ID: "SA1016", FilterGenerated: false, Fn: c.CheckUntrappableSignal, Doc: docSA1016},
		{ID: "SA1017", FilterGenerated: false, Fn: c.callChecker(checkUnbufferedSignalChanRules), Doc: docSA1017},
		{ID: "SA1018", FilterGenerated: false, Fn: c.callChecker(checkStringsReplaceZeroRules), Doc: docSA1018},
		{ID: "SA1019", FilterGenerated: false, Fn: c.CheckDeprecated, Doc: docSA1019},
		{ID: "SA1020", FilterGenerated: false, Fn: c.callChecker(checkListenAddressRules), Doc: docSA1020},
		{ID: "SA1021", FilterGenerated: false, Fn: c.callChecker(checkBytesEqualIPRules), Doc: docSA1021},
		{ID: "SA1023", FilterGenerated: false, Fn: c.CheckWriterBufferModified, Doc: docSA1023},
		{ID: "SA1024", FilterGenerated: false, Fn: c.callChecker(checkUniqueCutsetRules), Doc: docSA1024},
		{ID: "SA1025", FilterGenerated: false, Fn: c.CheckTimerResetReturnValue, Doc: docSA1025},
		{ID: "SA1026", FilterGenerated: false, Fn: c.callChecker(checkUnsupportedMarshal), Doc: docSA1026},
		{ID: "SA1027", FilterGenerated: false, Fn: c.callChecker(checkAtomicAlignment), Doc: docSA1027},

		{ID: "SA2000", FilterGenerated: false, Fn: c.CheckWaitgroupAdd, Doc: docSA2000},
		{ID: "SA2001", FilterGenerated: false, Fn: c.CheckEmptyCriticalSection, Doc: docSA2001},
		{ID: "SA2002", FilterGenerated: false, Fn: c.CheckConcurrentTesting, Doc: docSA2002},
		{ID: "SA2003", FilterGenerated: false, Fn: c.CheckDeferLock, Doc: docSA2003},

		{ID: "SA3000", FilterGenerated: false, Fn: c.CheckTestMainExit, Doc: docSA3000},
		{ID: "SA3001", FilterGenerated: false, Fn: c.CheckBenchmarkN, Doc: docSA3001},

		{ID: "SA4000", FilterGenerated: false, Fn: c.CheckLhsRhsIdentical, Doc: docSA4000},
		{ID: "SA4001", FilterGenerated: false, Fn: c.CheckIneffectiveCopy, Doc: docSA4001},
		{ID: "SA4002", FilterGenerated: false, Fn: c.CheckDiffSizeComparison, Doc: docSA4002},
		{ID: "SA4003", FilterGenerated: false, Fn: c.CheckExtremeComparison, Doc: docSA4003},
		{ID: "SA4004", FilterGenerated: false, Fn: c.CheckIneffectiveLoop, Doc: docSA4004},
		{ID: "SA4006", FilterGenerated: false, Fn: c.CheckUnreadVariableValues, Doc: docSA4006},
		{ID: "SA4008", FilterGenerated: false, Fn: c.CheckLoopCondition, Doc: docSA4008},
		{ID: "SA4009", FilterGenerated: false, Fn: c.CheckArgOverwritten, Doc: docSA4009},
		{ID: "SA4010", FilterGenerated: false, Fn: c.CheckIneffectiveAppend, Doc: docSA4010},
		{ID: "SA4011", FilterGenerated: false, Fn: c.CheckScopedBreak, Doc: docSA4011},
		{ID: "SA4012", FilterGenerated: false, Fn: c.CheckNaNComparison, Doc: docSA4012},
		{ID: "SA4013", FilterGenerated: false, Fn: c.CheckDoubleNegation, Doc: docSA4013},
		{ID: "SA4014", FilterGenerated: false, Fn: c.CheckRepeatedIfElse, Doc: docSA4014},
		{ID: "SA4015", FilterGenerated: false, Fn: c.callChecker(checkMathIntRules), Doc: docSA4015},
		{ID: "SA4016", FilterGenerated: false, Fn: c.CheckSillyBitwiseOps, Doc: docSA4016},
		{ID: "SA4017", FilterGenerated: false, Fn: c.CheckPureFunctions, Doc: docSA4017},
		{ID: "SA4018", FilterGenerated: true, Fn: c.CheckSelfAssignment, Doc: docSA4018},
		{ID: "SA4019", FilterGenerated: true, Fn: c.CheckDuplicateBuildConstraints, Doc: docSA4019},
		{ID: "SA4020", FilterGenerated: false, Fn: c.CheckUnreachableTypeCases, Doc: docSA4020},
		{ID: "SA4021", FilterGenerated: true, Fn: c.CheckSingleArgAppend, Doc: docSA4021},

		{ID: "SA5000", FilterGenerated: false, Fn: c.CheckNilMaps, Doc: docSA5000},
		{ID: "SA5001", FilterGenerated: false, Fn: c.CheckEarlyDefer, Doc: docSA5001},
		{ID: "SA5002", FilterGenerated: false, Fn: c.CheckInfiniteEmptyLoop, Doc: docSA5002},
		{ID: "SA5003", FilterGenerated: false, Fn: c.CheckDeferInInfiniteLoop, Doc: docSA5003},
		{ID: "SA5004", FilterGenerated: false, Fn: c.CheckLoopEmptyDefault, Doc: docSA5004},
		{ID: "SA5005", FilterGenerated: false, Fn: c.CheckCyclicFinalizer, Doc: docSA5005},
		{ID: "SA5007", FilterGenerated: false, Fn: c.CheckInfiniteRecursion, Doc: docSA5007},
		{ID: "SA5008", FilterGenerated: false, Fn: c.CheckStructTags, Doc: ``},
		{ID: "SA5009", FilterGenerated: false, Fn: c.callChecker(checkPrintfRules), Doc: ``},

		{ID: "SA6000", FilterGenerated: false, Fn: c.callChecker(checkRegexpMatchLoopRules), Doc: docSA6000},
		{ID: "SA6001", FilterGenerated: false, Fn: c.CheckMapBytesKey, Doc: docSA6001},
		{ID: "SA6002", FilterGenerated: false, Fn: c.callChecker(checkSyncPoolValueRules), Doc: docSA6002},
		{ID: "SA6003", FilterGenerated: false, Fn: c.CheckRangeStringRunes, Doc: docSA6003},
		// {ID: "SA6004", FilterGenerated: false, Fn: c.CheckSillyRegexp, Doc: docSA6004},
		{ID: "SA6005", FilterGenerated: false, Fn: c.CheckToLowerToUpperComparison, Doc: docSA6005},

		{ID: "SA9001", FilterGenerated: false, Fn: c.CheckDubiousDeferInChannelRangeLoop, Doc: docSA9001},
		{ID: "SA9002", FilterGenerated: false, Fn: c.CheckNonOctalFileMode, Doc: docSA9002},
		{ID: "SA9003", FilterGenerated: false, Fn: c.CheckEmptyBranch, Doc: docSA9003},
		{ID: "SA9004", FilterGenerated: false, Fn: c.CheckMissingEnumTypesInDeclaration, Doc: docSA9004},
		// Filtering generated code because it may include empty structs generated from data models.
		{ID: "SA9005", FilterGenerated: true, Fn: c.callChecker(checkNoopMarshal), Doc: docSA9005},
	}

	// "SA5006": c.CheckSliceOutOfBounds,
	// "SA4007": c.CheckPredeterminedBooleanExprs,
}

func (c *Checker) findDeprecated(prog *lint.Program) {
	var names []*ast.Ident

	extractDeprecatedMessage := func(docs []*ast.CommentGroup) string {
		for _, doc := range docs {
			if doc == nil {
				continue
			}
			parts := strings.Split(doc.Text(), "\n\n")
			last := parts[len(parts)-1]
			if !strings.HasPrefix(last, "Deprecated: ") {
				continue
			}
			alt := last[len("Deprecated: "):]
			alt = strings.Replace(alt, "\n", " ", -1)
			return alt
		}
		return ""
	}
	doDocs := func(pkg *packages.Package, names []*ast.Ident, docs []*ast.CommentGroup) {
		alt := extractDeprecatedMessage(docs)
		if alt == "" {
			return
		}

		for _, name := range names {
			obj := pkg.TypesInfo.ObjectOf(name)
			c.deprecatedObjs[obj] = alt
		}
	}

	for _, pkg := range prog.AllPackages {
		var docs []*ast.CommentGroup
		for _, f := range pkg.Syntax {
			docs = append(docs, f.Doc)
		}
		if alt := extractDeprecatedMessage(docs); alt != "" {
			// Don't mark package syscall as deprecated, even though
			// it is. A lot of people still use it for simple
			// constants like SIGKILL, and I am not comfortable
			// telling them to use x/sys for that.
			if pkg.PkgPath != "syscall" {
				c.deprecatedPkgs[pkg.Types] = alt
			}
		}

		docs = docs[:0]
		for _, f := range pkg.Syntax {
			fn := func(node ast.Node) bool {
				if node == nil {
					return true
				}
				var ret bool
				switch node := node.(type) {
				case *ast.GenDecl:
					switch node.Tok {
					case token.TYPE, token.CONST, token.VAR:
						docs = append(docs, node.Doc)
						return true
					default:
						return false
					}
				case *ast.FuncDecl:
					docs = append(docs, node.Doc)
					names = []*ast.Ident{node.Name}
					ret = false
				case *ast.TypeSpec:
					docs = append(docs, node.Doc)
					names = []*ast.Ident{node.Name}
					ret = true
				case *ast.ValueSpec:
					docs = append(docs, node.Doc)
					names = node.Names
					ret = false
				case *ast.File:
					return true
				case *ast.StructType:
					for _, field := range node.Fields.List {
						doDocs(pkg, field.Names, []*ast.CommentGroup{field.Doc})
					}
					return false
				case *ast.InterfaceType:
					for _, field := range node.Methods.List {
						doDocs(pkg, field.Names, []*ast.CommentGroup{field.Doc})
					}
					return false
				default:
					return false
				}
				if len(names) == 0 || len(docs) == 0 {
					return ret
				}
				doDocs(pkg, names, docs)

				docs = docs[:0]
				names = nil
				return ret
			}
			ast.Inspect(f, fn)
		}
	}
}

func (c *Checker) Init(prog *lint.Program) {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		c.funcDescs = functions.NewDescriptions(prog.SSA)
		for _, fn := range prog.AllFunctions {
			if fn.Blocks != nil {
				applyStdlibKnowledge(fn)
				ssa.OptimizeBlocks(fn)
			}
		}
		wg.Done()
	}()

	go func() {
		c.deprecatedPkgs = map[*types.Package]string{}
		c.deprecatedObjs = map[types.Object]string{}
		c.findDeprecated(prog)
		wg.Done()
	}()

	wg.Wait()
}

func (c *Checker) isInLoop(b *ssa.BasicBlock) bool {
	sets := c.funcDescs.Get(b.Parent()).Loops
	for _, set := range sets {
		if set[b] {
			return true
		}
	}
	return false
}

func applyStdlibKnowledge(fn *ssa.Function) {
	if len(fn.Blocks) == 0 {
		return
	}

	// comma-ok receiving from a time.Tick channel will never return
	// ok == false, so any branching on the value of ok can be
	// replaced with an unconditional jump. This will primarily match
	// `for range time.Tick(x)` loops, but it can also match
	// user-written code.
	for _, block := range fn.Blocks {
		if len(block.Instrs) < 3 {
			continue
		}
		if len(block.Succs) != 2 {
			continue
		}
		var instrs []*ssa.Instruction
		for i, ins := range block.Instrs {
			if _, ok := ins.(*ssa.DebugRef); ok {
				continue
			}
			instrs = append(instrs, &block.Instrs[i])
		}

		for i, ins := range instrs {
			unop, ok := (*ins).(*ssa.UnOp)
			if !ok || unop.Op != token.ARROW {
				continue
			}
			call, ok := unop.X.(*ssa.Call)
			if !ok {
				continue
			}
			if !IsCallTo(call.Common(), "time.Tick") {
				continue
			}
			ex, ok := (*instrs[i+1]).(*ssa.Extract)
			if !ok || ex.Tuple != unop || ex.Index != 1 {
				continue
			}

			ifstmt, ok := (*instrs[i+2]).(*ssa.If)
			if !ok || ifstmt.Cond != ex {
				continue
			}

			*instrs[i+2] = ssa.NewJump(block)
			succ := block.Succs[1]
			block.Succs = block.Succs[0:1]
			succ.RemovePred(block)
		}
	}
}

func (c *Checker) CheckUntrappableSignal(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !IsCallToAnyAST(j, call,
			"os/signal.Ignore", "os/signal.Notify", "os/signal.Reset") {
			return
		}
		for _, arg := range call.Args {
			if conv, ok := arg.(*ast.CallExpr); ok && isName(j, conv.Fun, "os.Signal") {
				arg = conv.Args[0]
			}

			if isName(j, arg, "os.Kill") || isName(j, arg, "syscall.SIGKILL") {
				j.Errorf(arg, "%s cannot be trapped (did you mean syscall.SIGTERM?)", Render(j, arg))
			}
			if isName(j, arg, "syscall.SIGSTOP") {
				j.Errorf(arg, "%s signal cannot be trapped", Render(j, arg))
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckTemplate(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		var kind string
		if IsCallToAST(j, call, "(*text/template.Template).Parse") {
			kind = "text"
		} else if IsCallToAST(j, call, "(*html/template.Template).Parse") {
			kind = "html"
		} else {
			return
		}
		sel := call.Fun.(*ast.SelectorExpr)
		if !IsCallToAST(j, sel.X, "text/template.New") &&
			!IsCallToAST(j, sel.X, "html/template.New") {
			// TODO(dh): this is a cheap workaround for templates with
			// different delims. A better solution with less false
			// negatives would use data flow analysis to see where the
			// template comes from and where it has been
			return
		}
		s, ok := ExprToString(j, call.Args[Arg("(*text/template.Template).Parse.text")])
		if !ok {
			return
		}
		var err error
		switch kind {
		case "text":
			_, err = texttemplate.New("").Parse(s)
		case "html":
			_, err = htmltemplate.New("").Parse(s)
		}
		if err != nil {
			// TODO(dominikh): whitelist other parse errors, if any
			if strings.Contains(err.Error(), "unexpected") {
				j.Errorf(call.Args[Arg("(*text/template.Template).Parse.text")], "%s", err)
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckTimeSleepConstant(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !IsCallToAST(j, call, "time.Sleep") {
			return
		}
		lit, ok := call.Args[Arg("time.Sleep.d")].(*ast.BasicLit)
		if !ok {
			return
		}
		n, err := strconv.Atoi(lit.Value)
		if err != nil {
			return
		}
		if n == 0 || n > 120 {
			// time.Sleep(0) is a seldom used pattern in concurrency
			// tests. >120 might be intentional. 120 was chosen
			// because the user could've meant 2 minutes.
			return
		}
		recommendation := "time.Sleep(time.Nanosecond)"
		if n != 1 {
			recommendation = fmt.Sprintf("time.Sleep(%d * time.Nanosecond)", n)
		}
		j.Errorf(call.Args[Arg("time.Sleep.d")],
			"sleeping for %d nanoseconds is probably a bug. Be explicit if it isn't: %s", n, recommendation)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckWaitgroupAdd(j *lint.Job) {
	fn := func(node ast.Node) {
		g := node.(*ast.GoStmt)
		fun, ok := g.Call.Fun.(*ast.FuncLit)
		if !ok {
			return
		}
		if len(fun.Body.List) == 0 {
			return
		}
		stmt, ok := fun.Body.List[0].(*ast.ExprStmt)
		if !ok {
			return
		}
		call, ok := stmt.X.(*ast.CallExpr)
		if !ok {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		fn, ok := j.Pkg.TypesInfo.ObjectOf(sel.Sel).(*types.Func)
		if !ok {
			return
		}
		if lint.FuncName(fn) == "(*sync.WaitGroup).Add" {
			j.Errorf(sel, "should call %s before starting the goroutine to avoid a race",
				Render(j, stmt))
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.GoStmt)(nil)}, fn)
}

func (c *Checker) CheckInfiniteEmptyLoop(j *lint.Job) {
	fn := func(node ast.Node) {
		loop := node.(*ast.ForStmt)
		if len(loop.Body.List) != 0 || loop.Post != nil {
			return
		}

		if loop.Init != nil {
			// TODO(dh): this isn't strictly necessary, it just makes
			// the check easier.
			return
		}
		// An empty loop is bad news in two cases: 1) The loop has no
		// condition. In that case, it's just a loop that spins
		// forever and as fast as it can, keeping a core busy. 2) The
		// loop condition only consists of variable or field reads and
		// operators on those. The only way those could change their
		// value is with unsynchronised access, which constitutes a
		// data race.
		//
		// If the condition contains any function calls, its behaviour
		// is dynamic and the loop might terminate. Similarly for
		// channel receives.

		if loop.Cond != nil {
			if hasSideEffects(loop.Cond) {
				return
			}
			if ident, ok := loop.Cond.(*ast.Ident); ok {
				if k, ok := j.Pkg.TypesInfo.ObjectOf(ident).(*types.Const); ok {
					if !constant.BoolVal(k.Val()) {
						// don't flag `for false {}` loops. They're a debug aid.
						return
					}
				}
			}
			j.Errorf(loop, "loop condition never changes or has a race condition")
		}
		j.Errorf(loop, "this loop will spin, using 100%% CPU")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil)}, fn)
}

func (c *Checker) CheckDeferInInfiniteLoop(j *lint.Job) {
	fn := func(node ast.Node) {
		mightExit := false
		var defers []ast.Stmt
		loop := node.(*ast.ForStmt)
		if loop.Cond != nil {
			return
		}
		fn2 := func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.ReturnStmt:
				mightExit = true
			case *ast.BranchStmt:
				// TODO(dominikh): if this sees a break in a switch or
				// select, it doesn't check if it breaks the loop or
				// just the select/switch. This causes some false
				// negatives.
				if stmt.Tok == token.BREAK {
					mightExit = true
				}
			case *ast.DeferStmt:
				defers = append(defers, stmt)
			case *ast.FuncLit:
				// Don't look into function bodies
				return false
			}
			return true
		}
		ast.Inspect(loop.Body, fn2)
		if mightExit {
			return
		}
		for _, stmt := range defers {
			j.Errorf(stmt, "defers in this infinite loop will never run")
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil)}, fn)
}

func (c *Checker) CheckDubiousDeferInChannelRangeLoop(j *lint.Job) {
	fn := func(node ast.Node) {
		loop := node.(*ast.RangeStmt)
		typ := j.Pkg.TypesInfo.TypeOf(loop.X)
		_, ok := typ.Underlying().(*types.Chan)
		if !ok {
			return
		}
		fn2 := func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.DeferStmt:
				j.Errorf(stmt, "defers in this range loop won't run unless the channel gets closed")
			case *ast.FuncLit:
				// Don't look into function bodies
				return false
			}
			return true
		}
		ast.Inspect(loop.Body, fn2)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.RangeStmt)(nil)}, fn)
}

func (c *Checker) CheckTestMainExit(j *lint.Job) {
	fn := func(node ast.Node) {
		if !isTestMain(j, node) {
			return
		}

		arg := j.Pkg.TypesInfo.ObjectOf(node.(*ast.FuncDecl).Type.Params.List[0].Names[0])
		callsRun := false
		fn2 := func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if arg != j.Pkg.TypesInfo.ObjectOf(ident) {
				return true
			}
			if sel.Sel.Name == "Run" {
				callsRun = true
				return false
			}
			return true
		}
		ast.Inspect(node.(*ast.FuncDecl).Body, fn2)

		callsExit := false
		fn3 := func(node ast.Node) bool {
			if IsCallToAST(j, node, "os.Exit") {
				callsExit = true
				return false
			}
			return true
		}
		ast.Inspect(node.(*ast.FuncDecl).Body, fn3)
		if !callsExit && callsRun {
			j.Errorf(node, "TestMain should call os.Exit to set exit code")
		}
	}
	j.Pkg.Inspector.Preorder(nil, fn)
}

func isTestMain(j *lint.Job, node ast.Node) bool {
	decl, ok := node.(*ast.FuncDecl)
	if !ok {
		return false
	}
	if decl.Name.Name != "TestMain" {
		return false
	}
	if len(decl.Type.Params.List) != 1 {
		return false
	}
	arg := decl.Type.Params.List[0]
	if len(arg.Names) != 1 {
		return false
	}
	return IsOfType(j, arg.Type, "*testing.M")
}

func (c *Checker) CheckExec(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !IsCallToAST(j, call, "os/exec.Command") {
			return
		}
		val, ok := ExprToString(j, call.Args[Arg("os/exec.Command.name")])
		if !ok {
			return
		}
		if !strings.Contains(val, " ") || strings.Contains(val, `\`) || strings.Contains(val, "/") {
			return
		}
		j.Errorf(call.Args[Arg("os/exec.Command.name")],
			"first argument to exec.Command looks like a shell command, but a program name or path are expected")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckLoopEmptyDefault(j *lint.Job) {
	fn := func(node ast.Node) {
		loop := node.(*ast.ForStmt)
		if len(loop.Body.List) != 1 || loop.Cond != nil || loop.Init != nil {
			return
		}
		sel, ok := loop.Body.List[0].(*ast.SelectStmt)
		if !ok {
			return
		}
		for _, c := range sel.Body.List {
			if comm, ok := c.(*ast.CommClause); ok && comm.Comm == nil && len(comm.Body) == 0 {
				j.Errorf(comm, "should not have an empty default case in a for+select loop. The loop will spin.")
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil)}, fn)
}

func (c *Checker) CheckLhsRhsIdentical(j *lint.Job) {
	fn := func(node ast.Node) {
		op := node.(*ast.BinaryExpr)
		switch op.Op {
		case token.EQL, token.NEQ:
			if basic, ok := j.Pkg.TypesInfo.TypeOf(op.X).Underlying().(*types.Basic); ok {
				if kind := basic.Kind(); kind == types.Float32 || kind == types.Float64 {
					// f == f and f != f might be used to check for NaN
					return
				}
			}
		case token.SUB, token.QUO, token.AND, token.REM, token.OR, token.XOR, token.AND_NOT,
			token.LAND, token.LOR, token.LSS, token.GTR, token.LEQ, token.GEQ:
		default:
			// For some ops, such as + and *, it can make sense to
			// have identical operands
			return
		}

		if Render(j, op.X) != Render(j, op.Y) {
			return
		}
		l1, ok1 := op.X.(*ast.BasicLit)
		l2, ok2 := op.Y.(*ast.BasicLit)
		if ok1 && ok2 && l1.Kind == token.INT && l2.Kind == l1.Kind && l1.Value == "0" && l2.Value == l1.Value && IsGenerated(j.File(l1)) {
			// cgo generates the following function call:
			// _cgoCheckPointer(_cgoBase0, 0 == 0) – it uses 0 == 0
			// instead of true in case the user shadowed the
			// identifier. Ideally we'd restrict this exception to
			// calls of _cgoCheckPointer, but it's not worth the
			// hassle of keeping track of the stack. <lit> <op> <lit>
			// are very rare to begin with, and we're mostly checking
			// for them to catch typos such as 1 == 1 where the user
			// meant to type i == 1. The odds of a false negative for
			// 0 == 0 are slim.
			return
		}
		j.Errorf(op, "identical expressions on the left and right side of the '%s' operator", op.Op)
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func (c *Checker) CheckScopedBreak(j *lint.Job) {
	fn := func(node ast.Node) {
		var body *ast.BlockStmt
		switch node := node.(type) {
		case *ast.ForStmt:
			body = node.Body
		case *ast.RangeStmt:
			body = node.Body
		default:
			panic(fmt.Sprintf("unreachable: %T", node))
		}
		for _, stmt := range body.List {
			var blocks [][]ast.Stmt
			switch stmt := stmt.(type) {
			case *ast.SwitchStmt:
				for _, c := range stmt.Body.List {
					blocks = append(blocks, c.(*ast.CaseClause).Body)
				}
			case *ast.SelectStmt:
				for _, c := range stmt.Body.List {
					blocks = append(blocks, c.(*ast.CommClause).Body)
				}
			default:
				continue
			}

			for _, body := range blocks {
				if len(body) == 0 {
					continue
				}
				lasts := []ast.Stmt{body[len(body)-1]}
				// TODO(dh): unfold all levels of nested block
				// statements, not just a single level if statement
				if ifs, ok := lasts[0].(*ast.IfStmt); ok {
					if len(ifs.Body.List) == 0 {
						continue
					}
					lasts[0] = ifs.Body.List[len(ifs.Body.List)-1]

					if block, ok := ifs.Else.(*ast.BlockStmt); ok {
						if len(block.List) != 0 {
							lasts = append(lasts, block.List[len(block.List)-1])
						}
					}
				}
				for _, last := range lasts {
					branch, ok := last.(*ast.BranchStmt)
					if !ok || branch.Tok != token.BREAK || branch.Label != nil {
						continue
					}
					j.Errorf(branch, "ineffective break statement. Did you mean to break out of the outer loop?")
				}
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.ForStmt)(nil), (*ast.RangeStmt)(nil)}, fn)
}

func (c *Checker) CheckUnsafePrintf(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		var arg int
		if IsCallToAnyAST(j, call, "fmt.Printf", "fmt.Sprintf", "log.Printf") {
			arg = Arg("fmt.Printf.format")
		} else if IsCallToAnyAST(j, call, "fmt.Fprintf") {
			arg = Arg("fmt.Fprintf.format")
		} else {
			return
		}
		if len(call.Args) != arg+1 {
			return
		}
		switch call.Args[arg].(type) {
		case *ast.CallExpr, *ast.Ident:
		default:
			return
		}
		j.Errorf(call.Args[arg],
			"printf-style function with dynamic format string and no further arguments should use print-style function instead")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckEarlyDefer(j *lint.Job) {
	fn := func(node ast.Node) {
		block := node.(*ast.BlockStmt)
		if len(block.List) < 2 {
			return
		}
		for i, stmt := range block.List {
			if i == len(block.List)-1 {
				break
			}
			assign, ok := stmt.(*ast.AssignStmt)
			if !ok {
				continue
			}
			if len(assign.Rhs) != 1 {
				continue
			}
			if len(assign.Lhs) < 2 {
				continue
			}
			if lhs, ok := assign.Lhs[len(assign.Lhs)-1].(*ast.Ident); ok && lhs.Name == "_" {
				continue
			}
			call, ok := assign.Rhs[0].(*ast.CallExpr)
			if !ok {
				continue
			}
			sig, ok := j.Pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
			if !ok {
				continue
			}
			if sig.Results().Len() < 2 {
				continue
			}
			last := sig.Results().At(sig.Results().Len() - 1)
			// FIXME(dh): check that it's error from universe, not
			// another type of the same name
			if last.Type().String() != "error" {
				continue
			}
			lhs, ok := assign.Lhs[0].(*ast.Ident)
			if !ok {
				continue
			}
			def, ok := block.List[i+1].(*ast.DeferStmt)
			if !ok {
				continue
			}
			sel, ok := def.Call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			ident, ok := selectorX(sel).(*ast.Ident)
			if !ok {
				continue
			}
			if ident.Obj != lhs.Obj {
				continue
			}
			if sel.Sel.Name != "Close" {
				continue
			}
			j.Errorf(def, "should check returned error before deferring %s", Render(j, def.Call))
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BlockStmt)(nil)}, fn)
}

func selectorX(sel *ast.SelectorExpr) ast.Node {
	switch x := sel.X.(type) {
	case *ast.SelectorExpr:
		return selectorX(x)
	default:
		return x
	}
}

func (c *Checker) CheckEmptyCriticalSection(j *lint.Job) {
	// Initially it might seem like this check would be easier to
	// implement in SSA. After all, we're only checking for two
	// consecutive method calls. In reality, however, there may be any
	// number of other instructions between the lock and unlock, while
	// still constituting an empty critical section. For example,
	// given `m.x().Lock(); m.x().Unlock()`, there will be a call to
	// x(). In the AST-based approach, this has a tiny potential for a
	// false positive (the second call to x might be doing work that
	// is protected by the mutex). In an SSA-based approach, however,
	// it would miss a lot of real bugs.

	mutexParams := func(s ast.Stmt) (x ast.Expr, funcName string, ok bool) {
		expr, ok := s.(*ast.ExprStmt)
		if !ok {
			return nil, "", false
		}
		call, ok := expr.X.(*ast.CallExpr)
		if !ok {
			return nil, "", false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil, "", false
		}

		fn, ok := j.Pkg.TypesInfo.ObjectOf(sel.Sel).(*types.Func)
		if !ok {
			return nil, "", false
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 0 || sig.Results().Len() != 0 {
			return nil, "", false
		}

		return sel.X, fn.Name(), true
	}

	fn := func(node ast.Node) {
		block := node.(*ast.BlockStmt)
		if len(block.List) < 2 {
			return
		}
		for i := range block.List[:len(block.List)-1] {
			sel1, method1, ok1 := mutexParams(block.List[i])
			sel2, method2, ok2 := mutexParams(block.List[i+1])

			if !ok1 || !ok2 || Render(j, sel1) != Render(j, sel2) {
				continue
			}
			if (method1 == "Lock" && method2 == "Unlock") ||
				(method1 == "RLock" && method2 == "RUnlock") {
				j.Errorf(block.List[i+1], "empty critical section")
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BlockStmt)(nil)}, fn)
}

// cgo produces code like fn(&*_Cvar_kSomeCallbacks) which we don't
// want to flag.
var cgoIdent = regexp.MustCompile(`^_C(func|var)_.+$`)

func (c *Checker) CheckIneffectiveCopy(j *lint.Job) {
	fn := func(node ast.Node) {
		if unary, ok := node.(*ast.UnaryExpr); ok {
			if star, ok := unary.X.(*ast.StarExpr); ok && unary.Op == token.AND {
				ident, ok := star.X.(*ast.Ident)
				if !ok || !cgoIdent.MatchString(ident.Name) {
					j.Errorf(unary, "&*x will be simplified to x. It will not copy x.")
				}
			}
		}

		if star, ok := node.(*ast.StarExpr); ok {
			if unary, ok := star.X.(*ast.UnaryExpr); ok && unary.Op == token.AND {
				j.Errorf(star, "*&x will be simplified to x. It will not copy x.")
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.UnaryExpr)(nil), (*ast.StarExpr)(nil)}, fn)
}

func (c *Checker) CheckDiffSizeComparison(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, b := range ssafn.Blocks {
			for _, ins := range b.Instrs {
				binop, ok := ins.(*ssa.BinOp)
				if !ok {
					continue
				}
				if binop.Op != token.EQL && binop.Op != token.NEQ {
					continue
				}
				_, ok1 := binop.X.(*ssa.Slice)
				_, ok2 := binop.Y.(*ssa.Slice)
				if !ok1 && !ok2 {
					continue
				}
				r := c.funcDescs.Get(ssafn).Ranges
				r1, ok1 := r.Get(binop.X).(vrp.StringInterval)
				r2, ok2 := r.Get(binop.Y).(vrp.StringInterval)
				if !ok1 || !ok2 {
					continue
				}
				if r1.Length.Intersection(r2.Length).Empty() {
					j.Errorf(binop, "comparing strings of different sizes for equality will always return false")
				}
			}
		}
	}
}

func (c *Checker) CheckCanonicalHeaderKey(j *lint.Job) {
	fn := func(node ast.Node, _ bool) bool {
		assign, ok := node.(*ast.AssignStmt)
		if ok {
			// TODO(dh): This risks missing some Header reads, for
			// example in `h1["foo"] = h2["foo"]` – these edge
			// cases are probably rare enough to ignore for now.
			for _, expr := range assign.Lhs {
				op, ok := expr.(*ast.IndexExpr)
				if !ok {
					continue
				}
				if IsOfType(j, op.X, "net/http.Header") {
					return false
				}
			}
			return true
		}
		op, ok := node.(*ast.IndexExpr)
		if !ok {
			return true
		}
		if !IsOfType(j, op.X, "net/http.Header") {
			return true
		}
		s, ok := ExprToString(j, op.Index)
		if !ok {
			return true
		}
		if s == http.CanonicalHeaderKey(s) {
			return true
		}
		j.Errorf(op, "keys in http.Header are canonicalized, %q is not canonical; fix the constant or use http.CanonicalHeaderKey", s)
		return true
	}
	j.Pkg.Inspector.Nodes([]ast.Node{(*ast.AssignStmt)(nil), (*ast.IndexExpr)(nil)}, fn)
}

func (c *Checker) CheckBenchmarkN(j *lint.Job) {
	fn := func(node ast.Node) {
		assign := node.(*ast.AssignStmt)
		if len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return
		}
		sel, ok := assign.Lhs[0].(*ast.SelectorExpr)
		if !ok {
			return
		}
		if sel.Sel.Name != "N" {
			return
		}
		if !IsOfType(j, sel.X, "*testing.B") {
			return
		}
		j.Errorf(assign, "should not assign to %s", Render(j, sel))
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.AssignStmt)(nil)}, fn)
}

func (c *Checker) CheckUnreadVariableValues(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		if IsExample(ssafn) {
			continue
		}
		node := ssafn.Syntax()
		if node == nil {
			continue
		}

		ast.Inspect(node, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			if len(assign.Lhs) > 1 && len(assign.Rhs) == 1 {
				// Either a function call with multiple return values,
				// or a comma-ok assignment

				val, _ := ssafn.ValueForExpr(assign.Rhs[0])
				if val == nil {
					return true
				}
				refs := val.Referrers()
				if refs == nil {
					return true
				}
				for _, ref := range *refs {
					ex, ok := ref.(*ssa.Extract)
					if !ok {
						continue
					}
					exrefs := ex.Referrers()
					if exrefs == nil {
						continue
					}
					if len(FilterDebug(*exrefs)) == 0 {
						lhs := assign.Lhs[ex.Index]
						if ident, ok := lhs.(*ast.Ident); !ok || ok && ident.Name == "_" {
							continue
						}
						j.Errorf(lhs, "this value of %s is never used", lhs)
					}
				}
				return true
			}
			for i, lhs := range assign.Lhs {
				rhs := assign.Rhs[i]
				if ident, ok := lhs.(*ast.Ident); !ok || ok && ident.Name == "_" {
					continue
				}
				val, _ := ssafn.ValueForExpr(rhs)
				if val == nil {
					continue
				}

				refs := val.Referrers()
				if refs == nil {
					// TODO investigate why refs can be nil
					return true
				}
				if len(FilterDebug(*refs)) == 0 {
					j.Errorf(lhs, "this value of %s is never used", lhs)
				}
			}
			return true
		})
	}
}

func (c *Checker) CheckPredeterminedBooleanExprs(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				ssabinop, ok := ins.(*ssa.BinOp)
				if !ok {
					continue
				}
				switch ssabinop.Op {
				case token.GTR, token.LSS, token.EQL, token.NEQ, token.LEQ, token.GEQ:
				default:
					continue
				}

				xs, ok1 := consts(ssabinop.X, nil, nil)
				ys, ok2 := consts(ssabinop.Y, nil, nil)
				if !ok1 || !ok2 || len(xs) == 0 || len(ys) == 0 {
					continue
				}

				trues := 0
				for _, x := range xs {
					for _, y := range ys {
						if x.Value == nil {
							if y.Value == nil {
								trues++
							}
							continue
						}
						if constant.Compare(x.Value, ssabinop.Op, y.Value) {
							trues++
						}
					}
				}
				b := trues != 0
				if trues == 0 || trues == len(xs)*len(ys) {
					j.Errorf(ssabinop, "binary expression is always %t for all possible values (%s %s %s)",
						b, xs, ssabinop.Op, ys)
				}
			}
		}
	}
}

func (c *Checker) CheckNilMaps(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				mu, ok := ins.(*ssa.MapUpdate)
				if !ok {
					continue
				}
				c, ok := mu.Map.(*ssa.Const)
				if !ok {
					continue
				}
				if c.Value != nil {
					continue
				}
				j.Errorf(mu, "assignment to nil map")
			}
		}
	}
}

func (c *Checker) CheckExtremeComparison(j *lint.Job) {
	isobj := func(expr ast.Expr, name string) bool {
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		return IsObject(j.Pkg.TypesInfo.ObjectOf(sel.Sel), name)
	}

	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		tx := j.Pkg.TypesInfo.TypeOf(expr.X)
		basic, ok := tx.Underlying().(*types.Basic)
		if !ok {
			return
		}

		var max string
		var min string

		switch basic.Kind() {
		case types.Uint8:
			max = "math.MaxUint8"
		case types.Uint16:
			max = "math.MaxUint16"
		case types.Uint32:
			max = "math.MaxUint32"
		case types.Uint64:
			max = "math.MaxUint64"
		case types.Uint:
			max = "math.MaxUint64"

		case types.Int8:
			min = "math.MinInt8"
			max = "math.MaxInt8"
		case types.Int16:
			min = "math.MinInt16"
			max = "math.MaxInt16"
		case types.Int32:
			min = "math.MinInt32"
			max = "math.MaxInt32"
		case types.Int64:
			min = "math.MinInt64"
			max = "math.MaxInt64"
		case types.Int:
			min = "math.MinInt64"
			max = "math.MaxInt64"
		}

		if (expr.Op == token.GTR || expr.Op == token.GEQ) && isobj(expr.Y, max) ||
			(expr.Op == token.LSS || expr.Op == token.LEQ) && isobj(expr.X, max) {
			j.Errorf(expr, "no value of type %s is greater than %s", basic, max)
		}
		if expr.Op == token.LEQ && isobj(expr.Y, max) ||
			expr.Op == token.GEQ && isobj(expr.X, max) {
			j.Errorf(expr, "every value of type %s is <= %s", basic, max)
		}

		if (basic.Info() & types.IsUnsigned) != 0 {
			if (expr.Op == token.LSS || expr.Op == token.LEQ) && IsIntLiteral(expr.Y, "0") ||
				(expr.Op == token.GTR || expr.Op == token.GEQ) && IsIntLiteral(expr.X, "0") {
				j.Errorf(expr, "no value of type %s is less than 0", basic)
			}
			if expr.Op == token.GEQ && IsIntLiteral(expr.Y, "0") ||
				expr.Op == token.LEQ && IsIntLiteral(expr.X, "0") {
				j.Errorf(expr, "every value of type %s is >= 0", basic)
			}
		} else {
			if (expr.Op == token.LSS || expr.Op == token.LEQ) && isobj(expr.Y, min) ||
				(expr.Op == token.GTR || expr.Op == token.GEQ) && isobj(expr.X, min) {
				j.Errorf(expr, "no value of type %s is less than %s", basic, min)
			}
			if expr.Op == token.GEQ && isobj(expr.Y, min) ||
				expr.Op == token.LEQ && isobj(expr.X, min) {
				j.Errorf(expr, "every value of type %s is >= %s", basic, min)
			}
		}

	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func consts(val ssa.Value, out []*ssa.Const, visitedPhis map[string]bool) ([]*ssa.Const, bool) {
	if visitedPhis == nil {
		visitedPhis = map[string]bool{}
	}
	var ok bool
	switch val := val.(type) {
	case *ssa.Phi:
		if visitedPhis[val.Name()] {
			break
		}
		visitedPhis[val.Name()] = true
		vals := val.Operands(nil)
		for _, phival := range vals {
			out, ok = consts(*phival, out, visitedPhis)
			if !ok {
				return nil, false
			}
		}
	case *ssa.Const:
		out = append(out, val)
	case *ssa.Convert:
		out, ok = consts(val.X, out, visitedPhis)
		if !ok {
			return nil, false
		}
	default:
		return nil, false
	}
	if len(out) < 2 {
		return out, true
	}
	uniq := []*ssa.Const{out[0]}
	for _, val := range out[1:] {
		if val.Value == uniq[len(uniq)-1].Value {
			continue
		}
		uniq = append(uniq, val)
	}
	return uniq, true
}

func (c *Checker) CheckLoopCondition(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		fn := func(node ast.Node) bool {
			loop, ok := node.(*ast.ForStmt)
			if !ok {
				return true
			}
			if loop.Init == nil || loop.Cond == nil || loop.Post == nil {
				return true
			}
			init, ok := loop.Init.(*ast.AssignStmt)
			if !ok || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
				return true
			}
			cond, ok := loop.Cond.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			x, ok := cond.X.(*ast.Ident)
			if !ok {
				return true
			}
			lhs, ok := init.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			if x.Obj != lhs.Obj {
				return true
			}
			if _, ok := loop.Post.(*ast.IncDecStmt); !ok {
				return true
			}

			v, isAddr := ssafn.ValueForExpr(cond.X)
			if v == nil || isAddr {
				return true
			}
			switch v := v.(type) {
			case *ssa.Phi:
				ops := v.Operands(nil)
				if len(ops) != 2 {
					return true
				}
				_, ok := (*ops[0]).(*ssa.Const)
				if !ok {
					return true
				}
				sigma, ok := (*ops[1]).(*ssa.Sigma)
				if !ok {
					return true
				}
				if sigma.X != v {
					return true
				}
			case *ssa.UnOp:
				return true
			}
			j.Errorf(cond, "variable in loop condition never changes")

			return true
		}
		Inspect(ssafn.Syntax(), fn)
	}
}

func (c *Checker) CheckArgOverwritten(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		fn := func(node ast.Node) bool {
			var typ *ast.FuncType
			var body *ast.BlockStmt
			switch fn := node.(type) {
			case *ast.FuncDecl:
				typ = fn.Type
				body = fn.Body
			case *ast.FuncLit:
				typ = fn.Type
				body = fn.Body
			}
			if body == nil {
				return true
			}
			if len(typ.Params.List) == 0 {
				return true
			}
			for _, field := range typ.Params.List {
				for _, arg := range field.Names {
					obj := j.Pkg.TypesInfo.ObjectOf(arg)
					var ssaobj *ssa.Parameter
					for _, param := range ssafn.Params {
						if param.Object() == obj {
							ssaobj = param
							break
						}
					}
					if ssaobj == nil {
						continue
					}
					refs := ssaobj.Referrers()
					if refs == nil {
						continue
					}
					if len(FilterDebug(*refs)) != 0 {
						continue
					}

					assigned := false
					ast.Inspect(body, func(node ast.Node) bool {
						assign, ok := node.(*ast.AssignStmt)
						if !ok {
							return true
						}
						for _, lhs := range assign.Lhs {
							ident, ok := lhs.(*ast.Ident)
							if !ok {
								continue
							}
							if j.Pkg.TypesInfo.ObjectOf(ident) == obj {
								assigned = true
								return false
							}
						}
						return true
					})
					if assigned {
						j.Errorf(arg, "argument %s is overwritten before first use", arg)
					}
				}
			}
			return true
		}
		Inspect(ssafn.Syntax(), fn)
	}
}

func (c *Checker) CheckIneffectiveLoop(j *lint.Job) {
	// This check detects some, but not all unconditional loop exits.
	// We give up in the following cases:
	//
	// - a goto anywhere in the loop. The goto might skip over our
	// return, and we don't check that it doesn't.
	//
	// - any nested, unlabelled continue, even if it is in another
	// loop or closure.
	fn := func(node ast.Node) {
		var body *ast.BlockStmt
		switch fn := node.(type) {
		case *ast.FuncDecl:
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		default:
			panic(fmt.Sprintf("unreachable: %T", node))
		}
		if body == nil {
			return
		}
		labels := map[*ast.Object]ast.Stmt{}
		ast.Inspect(body, func(node ast.Node) bool {
			label, ok := node.(*ast.LabeledStmt)
			if !ok {
				return true
			}
			labels[label.Label.Obj] = label.Stmt
			return true
		})

		ast.Inspect(body, func(node ast.Node) bool {
			var loop ast.Node
			var body *ast.BlockStmt
			switch node := node.(type) {
			case *ast.ForStmt:
				body = node.Body
				loop = node
			case *ast.RangeStmt:
				typ := j.Pkg.TypesInfo.TypeOf(node.X)
				if _, ok := typ.Underlying().(*types.Map); ok {
					// looping once over a map is a valid pattern for
					// getting an arbitrary element.
					return true
				}
				body = node.Body
				loop = node
			default:
				return true
			}
			if len(body.List) < 2 {
				// avoid flagging the somewhat common pattern of using
				// a range loop to get the first element in a slice,
				// or the first rune in a string.
				return true
			}
			var unconditionalExit ast.Node
			hasBranching := false
			for _, stmt := range body.List {
				switch stmt := stmt.(type) {
				case *ast.BranchStmt:
					switch stmt.Tok {
					case token.BREAK:
						if stmt.Label == nil || labels[stmt.Label.Obj] == loop {
							unconditionalExit = stmt
						}
					case token.CONTINUE:
						if stmt.Label == nil || labels[stmt.Label.Obj] == loop {
							unconditionalExit = nil
							return false
						}
					}
				case *ast.ReturnStmt:
					unconditionalExit = stmt
				case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.SelectStmt:
					hasBranching = true
				}
			}
			if unconditionalExit == nil || !hasBranching {
				return false
			}
			ast.Inspect(body, func(node ast.Node) bool {
				if branch, ok := node.(*ast.BranchStmt); ok {

					switch branch.Tok {
					case token.GOTO:
						unconditionalExit = nil
						return false
					case token.CONTINUE:
						if branch.Label != nil && labels[branch.Label.Obj] != loop {
							return true
						}
						unconditionalExit = nil
						return false
					}
				}
				return true
			})
			if unconditionalExit != nil {
				j.Errorf(unconditionalExit, "the surrounding loop is unconditionally terminated")
			}
			return true
		})
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, fn)
}

func (c *Checker) CheckNilContext(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if len(call.Args) == 0 {
			return
		}
		if typ, ok := j.Pkg.TypesInfo.TypeOf(call.Args[0]).(*types.Basic); !ok || typ.Kind() != types.UntypedNil {
			return
		}
		sig, ok := j.Pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
		if !ok {
			return
		}
		if sig.Params().Len() == 0 {
			return
		}
		if !IsType(sig.Params().At(0).Type(), "context.Context") {
			return
		}
		j.Errorf(call.Args[0],
			"do not pass a nil Context, even if a function permits it; pass context.TODO if you are unsure about which Context to use")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckSeeker(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		if sel.Sel.Name != "Seek" {
			return
		}
		if len(call.Args) != 2 {
			return
		}
		arg0, ok := call.Args[Arg("(io.Seeker).Seek.offset")].(*ast.SelectorExpr)
		if !ok {
			return
		}
		switch arg0.Sel.Name {
		case "SeekStart", "SeekCurrent", "SeekEnd":
		default:
			return
		}
		pkg, ok := arg0.X.(*ast.Ident)
		if !ok {
			return
		}
		if pkg.Name != "io" {
			return
		}
		j.Errorf(call, "the first argument of io.Seeker is the offset, but an io.Seek* constant is being used instead")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckIneffectiveAppend(j *lint.Job) {
	isAppend := func(ins ssa.Value) bool {
		call, ok := ins.(*ssa.Call)
		if !ok {
			return false
		}
		if call.Call.IsInvoke() {
			return false
		}
		if builtin, ok := call.Call.Value.(*ssa.Builtin); !ok || builtin.Name() != "append" {
			return false
		}
		return true
	}

	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				val, ok := ins.(ssa.Value)
				if !ok || !isAppend(val) {
					continue
				}

				isUsed := false
				visited := map[ssa.Instruction]bool{}
				var walkRefs func(refs []ssa.Instruction)
				walkRefs = func(refs []ssa.Instruction) {
				loop:
					for _, ref := range refs {
						if visited[ref] {
							continue
						}
						visited[ref] = true
						if _, ok := ref.(*ssa.DebugRef); ok {
							continue
						}
						switch ref := ref.(type) {
						case *ssa.Phi:
							walkRefs(*ref.Referrers())
						case *ssa.Sigma:
							walkRefs(*ref.Referrers())
						case ssa.Value:
							if !isAppend(ref) {
								isUsed = true
							} else {
								walkRefs(*ref.Referrers())
							}
						case ssa.Instruction:
							isUsed = true
							break loop
						}
					}
				}
				refs := val.Referrers()
				if refs == nil {
					continue
				}
				walkRefs(*refs)
				if !isUsed {
					j.Errorf(ins, "this result of append is never used, except maybe in other appends")
				}
			}
		}
	}
}

func (c *Checker) CheckConcurrentTesting(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				gostmt, ok := ins.(*ssa.Go)
				if !ok {
					continue
				}
				var fn *ssa.Function
				switch val := gostmt.Call.Value.(type) {
				case *ssa.Function:
					fn = val
				case *ssa.MakeClosure:
					fn = val.Fn.(*ssa.Function)
				default:
					continue
				}
				if fn.Blocks == nil {
					continue
				}
				for _, block := range fn.Blocks {
					for _, ins := range block.Instrs {
						call, ok := ins.(*ssa.Call)
						if !ok {
							continue
						}
						if call.Call.IsInvoke() {
							continue
						}
						callee := call.Call.StaticCallee()
						if callee == nil {
							continue
						}
						recv := callee.Signature.Recv()
						if recv == nil {
							continue
						}
						if !IsType(recv.Type(), "*testing.common") {
							continue
						}
						fn, ok := call.Call.StaticCallee().Object().(*types.Func)
						if !ok {
							continue
						}
						name := fn.Name()
						switch name {
						case "FailNow", "Fatal", "Fatalf", "SkipNow", "Skip", "Skipf":
						default:
							continue
						}
						j.Errorf(gostmt, "the goroutine calls T.%s, which must be called in the same goroutine as the test", name)
					}
				}
			}
		}
	}
}

func (c *Checker) CheckCyclicFinalizer(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		node := c.funcDescs.CallGraph.CreateNode(ssafn)
		for _, edge := range node.Out {
			if edge.Callee.Func.RelString(nil) != "runtime.SetFinalizer" {
				continue
			}
			arg0 := edge.Site.Common().Args[Arg("runtime.SetFinalizer.obj")]
			if iface, ok := arg0.(*ssa.MakeInterface); ok {
				arg0 = iface.X
			}
			unop, ok := arg0.(*ssa.UnOp)
			if !ok {
				continue
			}
			v, ok := unop.X.(*ssa.Alloc)
			if !ok {
				continue
			}
			arg1 := edge.Site.Common().Args[Arg("runtime.SetFinalizer.finalizer")]
			if iface, ok := arg1.(*ssa.MakeInterface); ok {
				arg1 = iface.X
			}
			mc, ok := arg1.(*ssa.MakeClosure)
			if !ok {
				continue
			}
			for _, b := range mc.Bindings {
				if b == v {
					pos := lint.DisplayPosition(j.Pkg.Fset, mc.Fn.Pos())
					j.Errorf(edge.Site, "the finalizer closes over the object, preventing the finalizer from ever running (at %s)", pos)
				}
			}
		}
	}
}

func (c *Checker) CheckSliceOutOfBounds(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				ia, ok := ins.(*ssa.IndexAddr)
				if !ok {
					continue
				}
				if _, ok := ia.X.Type().Underlying().(*types.Slice); !ok {
					continue
				}
				sr, ok1 := c.funcDescs.Get(ssafn).Ranges[ia.X].(vrp.SliceInterval)
				idxr, ok2 := c.funcDescs.Get(ssafn).Ranges[ia.Index].(vrp.IntInterval)
				if !ok1 || !ok2 || !sr.IsKnown() || !idxr.IsKnown() || sr.Length.Empty() || idxr.Empty() {
					continue
				}
				if idxr.Lower.Cmp(sr.Length.Upper) >= 0 {
					j.Errorf(ia, "index out of bounds")
				}
			}
		}
	}
}

func (c *Checker) CheckDeferLock(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			instrs := FilterDebug(block.Instrs)
			if len(instrs) < 2 {
				continue
			}
			for i, ins := range instrs[:len(instrs)-1] {
				call, ok := ins.(*ssa.Call)
				if !ok {
					continue
				}
				if !IsCallTo(call.Common(), "(*sync.Mutex).Lock") && !IsCallTo(call.Common(), "(*sync.RWMutex).RLock") {
					continue
				}
				nins, ok := instrs[i+1].(*ssa.Defer)
				if !ok {
					continue
				}
				if !IsCallTo(&nins.Call, "(*sync.Mutex).Lock") && !IsCallTo(&nins.Call, "(*sync.RWMutex).RLock") {
					continue
				}
				if call.Common().Args[0] != nins.Call.Args[0] {
					continue
				}
				name := shortCallName(call.Common())
				alt := ""
				switch name {
				case "Lock":
					alt = "Unlock"
				case "RLock":
					alt = "RUnlock"
				}
				j.Errorf(nins, "deferring %s right after having locked already; did you mean to defer %s?", name, alt)
			}
		}
	}
}

func (c *Checker) CheckNaNComparison(j *lint.Job) {
	isNaN := func(v ssa.Value) bool {
		call, ok := v.(*ssa.Call)
		if !ok {
			return false
		}
		return IsCallTo(call.Common(), "math.NaN")
	}
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				ins, ok := ins.(*ssa.BinOp)
				if !ok {
					continue
				}
				if isNaN(ins.X) || isNaN(ins.Y) {
					j.Errorf(ins, "no value is equal to NaN, not even NaN itself")
				}
			}
		}
	}
}

func (c *Checker) CheckInfiniteRecursion(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		node := c.funcDescs.CallGraph.CreateNode(ssafn)
		for _, edge := range node.Out {
			if edge.Callee != node {
				continue
			}
			if _, ok := edge.Site.(*ssa.Go); ok {
				// Recursively spawning goroutines doesn't consume
				// stack space infinitely, so don't flag it.
				continue
			}

			block := edge.Site.Block()
			canReturn := false
			for _, b := range ssafn.Blocks {
				if block.Dominates(b) {
					continue
				}
				if len(b.Instrs) == 0 {
					continue
				}
				if _, ok := b.Instrs[len(b.Instrs)-1].(*ssa.Return); ok {
					canReturn = true
					break
				}
			}
			if canReturn {
				continue
			}
			j.Errorf(edge.Site, "infinite recursive call")
		}
	}
}

func objectName(obj types.Object) string {
	if obj == nil {
		return "<nil>"
	}
	var name string
	if obj.Pkg() != nil && obj.Pkg().Scope().Lookup(obj.Name()) == obj {
		s := obj.Pkg().Path()
		if s != "" {
			name += s + "."
		}
	}
	name += obj.Name()
	return name
}

func isName(j *lint.Job, expr ast.Expr, name string) bool {
	var obj types.Object
	switch expr := expr.(type) {
	case *ast.Ident:
		obj = j.Pkg.TypesInfo.ObjectOf(expr)
	case *ast.SelectorExpr:
		obj = j.Pkg.TypesInfo.ObjectOf(expr.Sel)
	}
	return objectName(obj) == name
}

func (c *Checker) CheckLeakyTimeTick(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		if IsInMain(j, ssafn) || IsInTest(j, ssafn) {
			continue
		}
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				call, ok := ins.(*ssa.Call)
				if !ok || !IsCallTo(call.Common(), "time.Tick") {
					continue
				}
				if c.funcDescs.Get(call.Parent()).Infinite {
					continue
				}
				j.Errorf(call, "using time.Tick leaks the underlying ticker, consider using it only in endless functions, tests and the main package, and use time.NewTicker here")
			}
		}
	}
}

func (c *Checker) CheckDoubleNegation(j *lint.Job) {
	fn := func(node ast.Node) {
		unary1 := node.(*ast.UnaryExpr)
		unary2, ok := unary1.X.(*ast.UnaryExpr)
		if !ok {
			return
		}
		if unary1.Op != token.NOT || unary2.Op != token.NOT {
			return
		}
		j.Errorf(unary1, "negating a boolean twice has no effect; is this a typo?")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.UnaryExpr)(nil)}, fn)
}

func hasSideEffects(node ast.Node) bool {
	dynamic := false
	ast.Inspect(node, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			dynamic = true
			return false
		case *ast.UnaryExpr:
			if node.Op == token.ARROW {
				dynamic = true
				return false
			}
		}
		return true
	})
	return dynamic
}

func (c *Checker) CheckRepeatedIfElse(j *lint.Job) {
	seen := map[ast.Node]bool{}

	var collectConds func(ifstmt *ast.IfStmt, inits []ast.Stmt, conds []ast.Expr) ([]ast.Stmt, []ast.Expr)
	collectConds = func(ifstmt *ast.IfStmt, inits []ast.Stmt, conds []ast.Expr) ([]ast.Stmt, []ast.Expr) {
		seen[ifstmt] = true
		if ifstmt.Init != nil {
			inits = append(inits, ifstmt.Init)
		}
		conds = append(conds, ifstmt.Cond)
		if elsestmt, ok := ifstmt.Else.(*ast.IfStmt); ok {
			return collectConds(elsestmt, inits, conds)
		}
		return inits, conds
	}
	fn := func(node ast.Node) {
		ifstmt := node.(*ast.IfStmt)
		if seen[ifstmt] {
			return
		}
		inits, conds := collectConds(ifstmt, nil, nil)
		if len(inits) > 0 {
			return
		}
		for _, cond := range conds {
			if hasSideEffects(cond) {
				return
			}
		}
		counts := map[string]int{}
		for _, cond := range conds {
			s := Render(j, cond)
			counts[s]++
			if counts[s] == 2 {
				j.Errorf(cond, "this condition occurs multiple times in this if/else if chain")
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.IfStmt)(nil)}, fn)
}

func (c *Checker) CheckSillyBitwiseOps(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				ins, ok := ins.(*ssa.BinOp)
				if !ok {
					continue
				}

				if c, ok := ins.Y.(*ssa.Const); !ok || c.Value == nil || c.Value.Kind() != constant.Int || c.Uint64() != 0 {
					continue
				}
				switch ins.Op {
				case token.AND, token.OR, token.XOR:
				default:
					// we do not flag shifts because too often, x<<0 is part
					// of a pattern, x<<0, x<<8, x<<16, ...
					continue
				}
				path, _ := astutil.PathEnclosingInterval(j.File(ins), ins.Pos(), ins.Pos())
				if len(path) == 0 {
					continue
				}
				if node, ok := path[0].(*ast.BinaryExpr); !ok || !IsZero(node.Y) {
					continue
				}

				switch ins.Op {
				case token.AND:
					j.Errorf(ins, "x & 0 always equals 0")
				case token.OR, token.XOR:
					j.Errorf(ins, "x %s 0 always equals x", ins.Op)
				}
			}
		}
	}
}

func (c *Checker) CheckNonOctalFileMode(j *lint.Job) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		sig, ok := j.Pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
		if !ok {
			return
		}
		n := sig.Params().Len()
		var args []int
		for i := 0; i < n; i++ {
			typ := sig.Params().At(i).Type()
			if IsType(typ, "os.FileMode") {
				args = append(args, i)
			}
		}
		for _, i := range args {
			lit, ok := call.Args[i].(*ast.BasicLit)
			if !ok {
				continue
			}
			if len(lit.Value) == 3 &&
				lit.Value[0] != '0' &&
				lit.Value[0] >= '0' && lit.Value[0] <= '7' &&
				lit.Value[1] >= '0' && lit.Value[1] <= '7' &&
				lit.Value[2] >= '0' && lit.Value[2] <= '7' {

				v, err := strconv.ParseInt(lit.Value, 10, 64)
				if err != nil {
					continue
				}
				j.Errorf(call.Args[i], "file mode '%s' evaluates to %#o; did you mean '0%s'?", lit.Value, v, lit.Value)
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckPureFunctions(j *lint.Job) {
fnLoop:
	for _, ssafn := range j.Pkg.InitialFunctions {
		if IsInTest(j, ssafn) {
			params := ssafn.Signature.Params()
			for i := 0; i < params.Len(); i++ {
				param := params.At(i)
				if IsType(param.Type(), "*testing.B") {
					// Ignore discarded pure functions in code related
					// to benchmarks. Instead of matching BenchmarkFoo
					// functions, we match any function accepting a
					// *testing.B. Benchmarks sometimes call generic
					// functions for doing the actual work, and
					// checking for the parameter is a lot easier and
					// faster than analyzing call trees.
					continue fnLoop
				}
			}
		}

		for _, b := range ssafn.Blocks {
			for _, ins := range b.Instrs {
				ins, ok := ins.(*ssa.Call)
				if !ok {
					continue
				}
				refs := ins.Referrers()
				if refs == nil || len(FilterDebug(*refs)) > 0 {
					continue
				}
				callee := ins.Common().StaticCallee()
				if callee == nil {
					continue
				}
				if c.funcDescs.Get(callee).Pure && !c.funcDescs.Get(callee).Stub {
					j.Errorf(ins, "%s is a pure function but its return value is ignored", callee.Name())
					continue
				}
			}
		}
	}
}

func (c *Checker) isDeprecated(j *lint.Job, ident *ast.Ident) (bool, string) {
	obj := j.Pkg.TypesInfo.ObjectOf(ident)
	if obj.Pkg() == nil {
		return false, ""
	}
	alt := c.deprecatedObjs[obj]
	return alt != "", alt
}

func (c *Checker) CheckDeprecated(j *lint.Job) {
	// Selectors can appear outside of function literals, e.g. when
	// declaring package level variables.

	var ssafn *ssa.Function
	stack := 0
	fn := func(node ast.Node, push bool) bool {
		if !push {
			stack--
		} else {
			stack++
		}
		if stack == 1 {
			ssafn = nil
		}
		if fn, ok := node.(*ast.FuncDecl); ok {
			ssafn = j.Pkg.SSA.Prog.FuncValue(j.Pkg.TypesInfo.ObjectOf(fn.Name).(*types.Func))
		}
		sel, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		obj := j.Pkg.TypesInfo.ObjectOf(sel.Sel)
		if obj.Pkg() == nil {
			return true
		}
		nodePkg := j.Pkg.Types
		if nodePkg == obj.Pkg() || obj.Pkg().Path()+"_test" == nodePkg.Path() {
			// Don't flag stuff in our own package
			return true
		}
		if ok, alt := c.isDeprecated(j, sel.Sel); ok {
			// Look for the first available alternative, not the first
			// version something was deprecated in. If a function was
			// deprecated in Go 1.6, an alternative has been available
			// already in 1.0, and we're targeting 1.2, it still
			// makes sense to use the alternative from 1.0, to be
			// future-proof.
			minVersion := deprecated.Stdlib[SelectorName(j, sel)].AlternativeAvailableSince
			if !IsGoVersion(j, minVersion) {
				return true
			}

			if ssafn != nil {
				if _, ok := c.deprecatedObjs[ssafn.Object()]; ok {
					// functions that are deprecated may use deprecated
					// symbols
					return true
				}
			}
			j.Errorf(sel, "%s is deprecated: %s", Render(j, sel), alt)
			return true
		}
		return true
	}
	for _, f := range j.Pkg.Syntax {
		ast.Inspect(f, func(node ast.Node) bool {
			if node, ok := node.(*ast.ImportSpec); ok {
				p := node.Path.Value
				path := p[1 : len(p)-1]
				imp := j.Pkg.Imports[path]
				if alt := c.deprecatedPkgs[imp.Types]; alt != "" {
					j.Errorf(node, "Package %s is deprecated: %s", path, alt)
				}
			}
			return true
		})
	}
	j.Pkg.Inspector.Nodes(nil, fn)
}

func (c *Checker) callChecker(rules map[string]CallCheck) func(j *lint.Job) {
	return func(j *lint.Job) {
		c.checkCalls(j, rules)
	}
}

func (c *Checker) checkCalls(j *lint.Job, rules map[string]CallCheck) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		node := c.funcDescs.CallGraph.CreateNode(ssafn)
		for _, edge := range node.Out {
			callee := edge.Callee.Func
			obj, ok := callee.Object().(*types.Func)
			if !ok {
				continue
			}

			r, ok := rules[lint.FuncName(obj)]
			if !ok {
				continue
			}
			var args []*Argument
			ssaargs := edge.Site.Common().Args
			if callee.Signature.Recv() != nil {
				ssaargs = ssaargs[1:]
			}
			for _, arg := range ssaargs {
				if iarg, ok := arg.(*ssa.MakeInterface); ok {
					arg = iarg.X
				}
				vr := c.funcDescs.Get(edge.Site.Parent()).Ranges[arg]
				args = append(args, &Argument{Value: Value{arg, vr}})
			}
			call := &Call{
				Job:     j,
				Instr:   edge.Site,
				Args:    args,
				Checker: c,
				Parent:  edge.Site.Parent(),
			}
			r(call)
			for idx, arg := range call.Args {
				_ = idx
				for _, e := range arg.invalids {
					// path, _ := astutil.PathEnclosingInterval(f.File, edge.Site.Pos(), edge.Site.Pos())
					// if len(path) < 2 {
					// 	continue
					// }
					// astcall, ok := path[0].(*ast.CallExpr)
					// if !ok {
					// 	continue
					// }
					// j.Errorf(astcall.Args[idx], "%s", e)

					j.Errorf(edge.Site, "%s", e)
				}
			}
			for _, e := range call.invalids {
				j.Errorf(call.Instr.Common(), "%s", e)
			}
		}
	}
}

func shortCallName(call *ssa.CallCommon) string {
	if call.IsInvoke() {
		return ""
	}
	switch v := call.Value.(type) {
	case *ssa.Function:
		fn, ok := v.Object().(*types.Func)
		if !ok {
			return ""
		}
		return fn.Name()
	case *ssa.Builtin:
		return v.Name()
	}
	return ""
}

func (c *Checker) CheckWriterBufferModified(j *lint.Job) {
	// TODO(dh): this might be a good candidate for taint analysis.
	// Taint the argument as MUST_NOT_MODIFY, then propagate that
	// through functions like bytes.Split

	for _, ssafn := range j.Pkg.InitialFunctions {
		sig := ssafn.Signature
		if ssafn.Name() != "Write" || sig.Recv() == nil || sig.Params().Len() != 1 || sig.Results().Len() != 2 {
			continue
		}
		tArg, ok := sig.Params().At(0).Type().(*types.Slice)
		if !ok {
			continue
		}
		if basic, ok := tArg.Elem().(*types.Basic); !ok || basic.Kind() != types.Byte {
			continue
		}
		if basic, ok := sig.Results().At(0).Type().(*types.Basic); !ok || basic.Kind() != types.Int {
			continue
		}
		if named, ok := sig.Results().At(1).Type().(*types.Named); !ok || !IsType(named, "error") {
			continue
		}

		for _, block := range ssafn.Blocks {
			for _, ins := range block.Instrs {
				switch ins := ins.(type) {
				case *ssa.Store:
					addr, ok := ins.Addr.(*ssa.IndexAddr)
					if !ok {
						continue
					}
					if addr.X != ssafn.Params[1] {
						continue
					}
					j.Errorf(ins, "io.Writer.Write must not modify the provided buffer, not even temporarily")
				case *ssa.Call:
					if !IsCallTo(ins.Common(), "append") {
						continue
					}
					if ins.Common().Args[0] != ssafn.Params[1] {
						continue
					}
					j.Errorf(ins, "io.Writer.Write must not modify the provided buffer, not even temporarily")
				}
			}
		}
	}
}

func loopedRegexp(name string) CallCheck {
	return func(call *Call) {
		if len(extractConsts(call.Args[0].Value.Value)) == 0 {
			return
		}
		if !call.Checker.isInLoop(call.Instr.Block()) {
			return
		}
		call.Invalid(fmt.Sprintf("calling %s in a loop has poor performance, consider using regexp.Compile", name))
	}
}

func (c *Checker) CheckEmptyBranch(j *lint.Job) {
	for _, ssafn := range j.Pkg.InitialFunctions {
		if ssafn.Syntax() == nil {
			continue
		}
		if IsGenerated(j.File(ssafn.Syntax())) {
			continue
		}
		if IsExample(ssafn) {
			continue
		}
		fn := func(node ast.Node) bool {
			ifstmt, ok := node.(*ast.IfStmt)
			if !ok {
				return true
			}
			if ifstmt.Else != nil {
				b, ok := ifstmt.Else.(*ast.BlockStmt)
				if !ok || len(b.List) != 0 {
					return true
				}
				j.Errorf(ifstmt.Else, "empty branch")
			}
			if len(ifstmt.Body.List) != 0 {
				return true
			}
			j.Errorf(ifstmt, "empty branch")
			return true
		}
		Inspect(ssafn.Syntax(), fn)
	}
}

func (c *Checker) CheckMapBytesKey(j *lint.Job) {
	for _, fn := range j.Pkg.InitialFunctions {
		for _, b := range fn.Blocks {
		insLoop:
			for _, ins := range b.Instrs {
				// find []byte -> string conversions
				conv, ok := ins.(*ssa.Convert)
				if !ok || conv.Type() != types.Universe.Lookup("string").Type() {
					continue
				}
				if s, ok := conv.X.Type().(*types.Slice); !ok || s.Elem() != types.Universe.Lookup("byte").Type() {
					continue
				}
				refs := conv.Referrers()
				// need at least two (DebugRef) references: the
				// conversion and the *ast.Ident
				if refs == nil || len(*refs) < 2 {
					continue
				}
				ident := false
				// skip first reference, that's the conversion itself
				for _, ref := range (*refs)[1:] {
					switch ref := ref.(type) {
					case *ssa.DebugRef:
						if _, ok := ref.Expr.(*ast.Ident); !ok {
							// the string seems to be used somewhere
							// unexpected; the default branch should
							// catch this already, but be safe
							continue insLoop
						} else {
							ident = true
						}
					case *ssa.Lookup:
					default:
						// the string is used somewhere else than a
						// map lookup
						continue insLoop
					}
				}

				// the result of the conversion wasn't assigned to an
				// identifier
				if !ident {
					continue
				}
				j.Errorf(conv, "m[string(key)] would be more efficient than k := string(key); m[k]")
			}
		}
	}
}

func (c *Checker) CheckRangeStringRunes(j *lint.Job) {
	sharedcheck.CheckRangeStringRunes(j)
}

func (c *Checker) CheckSelfAssignment(j *lint.Job) {
	fn := func(node ast.Node) {
		assign := node.(*ast.AssignStmt)
		if assign.Tok != token.ASSIGN || len(assign.Lhs) != len(assign.Rhs) {
			return
		}
		for i, stmt := range assign.Lhs {
			rlh := Render(j, stmt)
			rrh := Render(j, assign.Rhs[i])
			if rlh == rrh {
				j.Errorf(assign, "self-assignment of %s to %s", rrh, rlh)
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.AssignStmt)(nil)}, fn)
}

func buildTagsIdentical(s1, s2 []string) bool {
	if len(s1) != len(s2) {
		return false
	}
	s1s := make([]string, len(s1))
	copy(s1s, s1)
	sort.Strings(s1s)
	s2s := make([]string, len(s2))
	copy(s2s, s2)
	sort.Strings(s2s)
	for i, s := range s1s {
		if s != s2s[i] {
			return false
		}
	}
	return true
}

func (c *Checker) CheckDuplicateBuildConstraints(job *lint.Job) {
	for _, f := range job.Pkg.Syntax {
		constraints := buildTags(f)
		for i, constraint1 := range constraints {
			for j, constraint2 := range constraints {
				if i >= j {
					continue
				}
				if buildTagsIdentical(constraint1, constraint2) {
					job.Errorf(f, "identical build constraints %q and %q",
						strings.Join(constraint1, " "),
						strings.Join(constraint2, " "))
				}
			}
		}
	}
}

func (c *Checker) CheckSillyRegexp(j *lint.Job) {
	// We could use the rule checking engine for this, but the
	// arguments aren't really invalid.
	for _, fn := range j.Pkg.InitialFunctions {
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				call, ok := ins.(*ssa.Call)
				if !ok {
					continue
				}
				switch CallName(call.Common()) {
				case "regexp.MustCompile", "regexp.Compile", "regexp.Match", "regexp.MatchReader", "regexp.MatchString":
				default:
					continue
				}
				c, ok := call.Common().Args[0].(*ssa.Const)
				if !ok {
					continue
				}
				s := constant.StringVal(c.Value)
				re, err := syntax.Parse(s, 0)
				if err != nil {
					continue
				}
				if re.Op != syntax.OpLiteral && re.Op != syntax.OpEmptyMatch {
					continue
				}
				j.Errorf(call, "regular expression does not contain any meta characters")
			}
		}
	}
}

func (c *Checker) CheckMissingEnumTypesInDeclaration(j *lint.Job) {
	fn := func(node ast.Node) {
		decl := node.(*ast.GenDecl)
		if !decl.Lparen.IsValid() {
			return
		}
		if decl.Tok != token.CONST {
			return
		}

		groups := GroupSpecs(j.Pkg.Fset, decl.Specs)
	groupLoop:
		for _, group := range groups {
			if len(group) < 2 {
				continue
			}
			if group[0].(*ast.ValueSpec).Type == nil {
				// first constant doesn't have a type
				continue groupLoop
			}
			for i, spec := range group {
				spec := spec.(*ast.ValueSpec)
				if len(spec.Names) != 1 || len(spec.Values) != 1 {
					continue groupLoop
				}
				switch v := spec.Values[0].(type) {
				case *ast.BasicLit:
				case *ast.UnaryExpr:
					if _, ok := v.X.(*ast.BasicLit); !ok {
						continue groupLoop
					}
				default:
					// if it's not a literal it might be typed, such as
					// time.Microsecond = 1000 * Nanosecond
					continue groupLoop
				}
				if i == 0 {
					continue
				}
				if spec.Type != nil {
					continue groupLoop
				}
			}
			j.Errorf(group[0], "only the first constant in this group has an explicit type")
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.GenDecl)(nil)}, fn)
}

func (c *Checker) CheckTimerResetReturnValue(j *lint.Job) {
	for _, fn := range j.Pkg.InitialFunctions {
		for _, block := range fn.Blocks {
			for _, ins := range block.Instrs {
				call, ok := ins.(*ssa.Call)
				if !ok {
					continue
				}
				if !IsCallTo(call.Common(), "(*time.Timer).Reset") {
					continue
				}
				refs := call.Referrers()
				if refs == nil {
					continue
				}
				for _, ref := range FilterDebug(*refs) {
					ifstmt, ok := ref.(*ssa.If)
					if !ok {
						continue
					}

					found := false
					for _, succ := range ifstmt.Block().Succs {
						if len(succ.Preds) != 1 {
							// Merge point, not a branch in the
							// syntactical sense.

							// FIXME(dh): this is broken for if
							// statements a la "if x || y"
							continue
						}
						ssautil.Walk(succ, func(b *ssa.BasicBlock) bool {
							if !succ.Dominates(b) {
								// We've reached the end of the branch
								return false
							}
							for _, ins := range b.Instrs {
								// TODO(dh): we should check that
								// we're receiving from the channel of
								// a time.Timer to further reduce
								// false positives. Not a key
								// priority, considering the rarity of
								// Reset and the tiny likeliness of a
								// false positive
								if ins, ok := ins.(*ssa.UnOp); ok && ins.Op == token.ARROW && IsType(ins.X.Type(), "<-chan time.Time") {
									found = true
									return false
								}
							}
							return true
						})
					}

					if found {
						j.Errorf(call, "it is not possible to use Reset's return value correctly, as there is a race condition between draining the channel and the new timer expiring")
					}
				}
			}
		}
	}
}

func (c *Checker) CheckToLowerToUpperComparison(j *lint.Job) {
	fn := func(node ast.Node) {
		binExpr := node.(*ast.BinaryExpr)

		var negative bool
		switch binExpr.Op {
		case token.EQL:
			negative = false
		case token.NEQ:
			negative = true
		default:
			return
		}

		const (
			lo = "strings.ToLower"
			up = "strings.ToUpper"
		)

		var call string
		if IsCallToAST(j, binExpr.X, lo) && IsCallToAST(j, binExpr.Y, lo) {
			call = lo
		} else if IsCallToAST(j, binExpr.X, up) && IsCallToAST(j, binExpr.Y, up) {
			call = up
		} else {
			return
		}

		bang := ""
		if negative {
			bang = "!"
		}

		j.Errorf(binExpr, "should use %sstrings.EqualFold(a, b) instead of %s(a) %s %s(b)", bang, call, binExpr.Op, call)
	}

	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.BinaryExpr)(nil)}, fn)
}

func (c *Checker) CheckUnreachableTypeCases(j *lint.Job) {
	// Check if T subsumes V in a type switch. T subsumes V if T is an interface and T's method set is a subset of V's method set.
	subsumes := func(T, V types.Type) bool {
		tIface, ok := T.Underlying().(*types.Interface)
		if !ok {
			return false
		}

		return types.Implements(V, tIface)
	}

	subsumesAny := func(Ts, Vs []types.Type) (types.Type, types.Type, bool) {
		for _, T := range Ts {
			for _, V := range Vs {
				if subsumes(T, V) {
					return T, V, true
				}
			}
		}

		return nil, nil, false
	}

	fn := func(node ast.Node) {
		tsStmt := node.(*ast.TypeSwitchStmt)

		type ccAndTypes struct {
			cc    *ast.CaseClause
			types []types.Type
		}

		// All asserted types in the order of case clauses.
		ccs := make([]ccAndTypes, 0, len(tsStmt.Body.List))
		for _, stmt := range tsStmt.Body.List {
			cc, _ := stmt.(*ast.CaseClause)

			// Exclude the 'default' case.
			if len(cc.List) == 0 {
				continue
			}

			Ts := make([]types.Type, len(cc.List))
			for i, expr := range cc.List {
				Ts[i] = j.Pkg.TypesInfo.TypeOf(expr)
			}

			ccs = append(ccs, ccAndTypes{cc: cc, types: Ts})
		}

		if len(ccs) <= 1 {
			// Zero or one case clauses, nothing to check.
			return
		}

		// Check if case clauses following cc have types that are subsumed by cc.
		for i, cc := range ccs[:len(ccs)-1] {
			for _, next := range ccs[i+1:] {
				if T, V, yes := subsumesAny(cc.types, next.types); yes {
					j.Errorf(next.cc, "unreachable case clause: %s will always match before %s", T.String(), V.String())
				}
			}
		}
	}

	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.TypeSwitchStmt)(nil)}, fn)
}

func (c *Checker) CheckSingleArgAppend(j *lint.Job) {
	fn := func(node ast.Node) {
		if !IsCallToAST(j, node, "append") {
			return
		}
		call := node.(*ast.CallExpr)
		if len(call.Args) != 1 {
			return
		}
		j.Errorf(call, "x = append(y) is equivalent to x = y")
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, fn)
}

func (c *Checker) CheckStructTags(j *lint.Job) {
	fn := func(node ast.Node) {
		for _, field := range node.(*ast.StructType).Fields.List {
			if field.Tag == nil {
				continue
			}
			tags, err := parseStructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
			if err != nil {
				j.Errorf(field.Tag, "unparseable struct tag: %s", err)
				continue
			}
			for k, v := range tags {
				if len(v) > 1 {
					j.Errorf(field.Tag, "duplicate struct tag %q", k)
					continue
				}

				switch k {
				case "json":
					checkJSONTag(j, field, v[0])
				case "xml":
					checkXMLTag(j, field, v[0])
				}
			}
		}
	}
	j.Pkg.Inspector.Preorder([]ast.Node{(*ast.StructType)(nil)}, fn)
}

func checkJSONTag(j *lint.Job, field *ast.Field, tag string) {
	if len(tag) == 0 {
		// TODO(dh): should we flag empty tags?
	}
	fields := strings.Split(tag, ",")
	for _, r := range fields[0] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("!#$%&()*+-./:<=>?@[]^_{|}~ ", r) {
			j.Errorf(field.Tag, "invalid JSON field name %q", fields[0])
		}
	}
	var co, cs, ci int
	for _, s := range fields[1:] {
		switch s {
		case "omitempty":
			co++
		case "":
			// allow stuff like "-,"
		case "string":
			cs++
			// only for string, floating point, integer and bool
			T := Dereference(j.Pkg.TypesInfo.TypeOf(field.Type).Underlying()).Underlying()
			basic, ok := T.(*types.Basic)
			if !ok || (basic.Info()&(types.IsBoolean|types.IsInteger|types.IsFloat|types.IsString)) == 0 {
				j.Errorf(field.Tag, "the JSON string option only applies to fields of type string, floating point, integer or bool, or pointers to those")
			}
		case "inline":
			ci++
		default:
			j.Errorf(field.Tag, "unknown JSON option %q", s)
		}
	}
	if co > 1 {
		j.Errorf(field.Tag, `duplicate JSON option "omitempty"`)
	}
	if cs > 1 {
		j.Errorf(field.Tag, `duplicate JSON option "string"`)
	}
	if ci > 1 {
		j.Errorf(field.Tag, `duplicate JSON option "inline"`)
	}
}

func checkXMLTag(j *lint.Job, field *ast.Field, tag string) {
	if len(tag) == 0 {
		// TODO(dh): should we flag empty tags?
	}
	fields := strings.Split(tag, ",")
	counts := map[string]int{}
	var exclusives []string
	for _, s := range fields[1:] {
		switch s {
		case "attr", "chardata", "cdata", "innerxml", "comment":
			counts[s]++
			if counts[s] == 1 {
				exclusives = append(exclusives, s)
			}
		case "omitempty", "any":
			counts[s]++
		case "":
		default:
			j.Errorf(field.Tag, "unknown XML option %q", s)
		}
	}
	for k, v := range counts {
		if v > 1 {
			j.Errorf(field.Tag, "duplicate XML option %q", k)
		}
	}
	if len(exclusives) > 1 {
		j.Errorf(field.Tag, "XML options %s are mutually exclusive", strings.Join(exclusives, " and "))
	}
}
