package logfields

const (
	// Identifiers

	Name      = "name"
	Namespace = "namespace"
	Operation = "operation"

	ID          = "id"
	ContainerID = "cid"
	ExecID      = "eid"
	ProcessID   = "pid"
	TaskID      = "tid"
	UVMID       = "uvm-id"
	SandboxID   = "sandbox-id"

	// networking and IO

	File  = "file"
	Path  = "path"
	Bytes = "bytes"
	Pipe  = "pipe"

	// Common Misc

	Attempt = "attemptNo"
	JSON    = "json"

	// Time

	StartTime = "startTime"
	EndTime   = "endTime"
	Duration  = "duration"
	Timeout   = "timeout"

	// Keys/values

	Field         = "field"
	Key           = "key"
	OCIAnnotation = "oci-annotation"
	Value         = "value"
	Options       = "options"

	// Golang type's

	ExpectedType = "expected-type"
	Bool         = "bool"
	Int32        = "int32"
	Uint32       = "uint32"
	Uint64       = "uint64"

	// task / process lifecycle

	Bundle           = "bundle"
	Terminal         = "terminal"
	Stdin            = "stdin"
	Stdout           = "stdout"
	Stderr           = "stderr"
	Checkpoint       = "checkpoint"
	ParentCheckpoint = "parent-checkpoint"
	Status           = "status"
	ExitStatus       = "exit-status"
	ExitedAt         = "exited-at"
	Signal           = "signal"
	All              = "all"
	Width            = "width"
	Height           = "height"
	Version          = "version"
	ShimPid          = "shim-pid"
	TaskPid          = "task-pid"

	// sandbox

	NetNsPath = "net-ns-path"
	Verbose   = "verbose"

	// shimdiag

	Args     = "args"
	Workdir  = "workdir"
	HostPath = "host-path"
	UVMPath  = "uvm-path"
	ReadOnly = "readonly"
	Execs    = "execs"

	// runhcs

	VMShimOperation = "vmshim-op"

	// logging and tracing

	TraceID      = "traceID"
	SpanID       = "spanID"
	ParentSpanID = "parentSpanID"
)
