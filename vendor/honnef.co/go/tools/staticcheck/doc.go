package staticcheck

var docSA1000 = `Invalid regular expression

Available since
    2017.1
`

var docSA1001 = `Invalid template

Available since
    2017.1
`

var docSA1002 = `Invalid format in time.Parse

Available since
    2017.1
`

var docSA1003 = `Unsupported argument to functions in encoding/binary

The encoding/binary package can only serialize types with known sizes.
This precludes the use of the 'int' and 'uint' types, as their sizes
differ on different architectures. Furthermore, it doesn't support
serializing maps, channels, strings, or functions.

Before Go 1.8, bool wasn't supported, either.

Available since
    2017.1
`

var docSA1004 = `Suspiciously small untyped constant in time.Sleep

The time.Sleep function takes a time.Duration as its only argument.
Durations are expressed in nanoseconds. Thus, calling time.Sleep(1)
will sleep for 1 nanosecond. This is a common source of bugs, as sleep
functions in other languages often accept seconds or milliseconds.

The time package provides constants such as time.Second to express
large durations. These can be combined with arithmetic to express
arbitrary durations, for example '5 * time.Second' for 5 seconds.

If you truly meant to sleep for a tiny amount of time, use
'n * time.Nanosecond" to signal to staticcheck that you did mean to sleep
for some amount of nanoseconds.

Available since
    2017.1
`

var docSA1005 = `Invalid first argument to exec.Command

os/exec runs programs directly (using variants of the fork and exec
system calls on Unix systems). This shouldn't be confused with running
a command in a shell. The shell will allow for features such as input
redirection, pipes, and general scripting. The shell is also
responsible for splitting the user's input into a program name and its
arguments. For example, the equivalent to

    ls / /tmp

would be

    exec.Command("ls", "/", "/tmp")

If you want to run a command in a shell, consider using something like
the following – but be aware that not all systems, particularly
Windows, will have a /bin/sh program:

    exec.Command("/bin/sh", "-c", "ls | grep Awesome")

Available since
    2017.1
`

var docSA1006 = `Printf with dynamic first argument and no further arguments

Using fmt.Printf with a dynamic first argument can lead to unexpected
output. The first argument is a format string, where certain character
combinations have special meaning. If, for example, a user were to
enter a string such as

    Interest rate: 5%

and you printed it with

fmt.Printf(s)

it would lead to the following output:

  Interest rate: 5%!(NOVERB).

Similarly, forming the first parameter via string concatenation with
user input should be avoided for the same reason. When printing user
input, either use a variant of fmt.Print, or use the %s Printf verb
and pass the string as an argument.

Available since
    2017.1
`

var docSA1007 = `Invalid URL in net/url.Parse

Available since
    2017.1
`

var docSA1008 = `Non-canonical key in http.Header map

Available since
    2017.1
`

var docSA1010 = `(*regexp.Regexp).FindAll called with n == 0, which will always return zero results

If n >= 0, the function returns at most n matches/submatches. To
return all results, specify a negative number.

Available since
    2017.1
`

var docSA1011 = `Various methods in the strings package expect valid UTF-8, but invalid input is provided

Available since
    2017.1
`

var docSA1012 = `A nil context.Context is being passed to a function, consider using context.TODO instead

Available since
    2017.1
`

var docSA1013 = `io.Seeker.Seek is being called with the whence constant as the first argument, but it should be the second

Available since
    2017.1
`

var docSA1014 = `Non-pointer value passed to Unmarshal or Decode

Available since
    2017.1
`

var docSA1015 = `Using time.Tick in a way that will leak. Consider using time.NewTicker, and only use time.Tick in tests, commands and endless functions

Available since
    2017.1
`

var docSA1016 = `Trapping a signal that cannot be trapped

Not all signals can be intercepted by a process. Speficially, on
UNIX-like systems, the syscall.SIGKILL and syscall.SIGSTOP signals are
never passed to the process, but instead handled directly by the
kernel. It is therefore pointless to try and handle these signals.

Available since
    2017.1
`

var docSA1017 = `Channels used with os/signal.Notify should be buffered

The os/signal package uses non-blocking channel sends when delivering
signals. If the receiving end of the channel isn't ready and the
channel is either unbuffered or full, the signal will be dropped. To
avoid missing signals, the channel should be buffered and of the
appropriate size. For a channel used for notification of just one
signal value, a buffer of size 1 is sufficient.


Available since
    2017.1
`

var docSA1018 = `strings.Replace called with n == 0, which does nothing

With n == 0, zero instances will be replaced. To replace all
instances, use a negative number, or use strings.ReplaceAll.

Available since
    2017.1
`

var docSA1019 = `Using a deprecated function, variable, constant or field

Available since
    2017.1
`

var docSA1020 = `Using an invalid host:port pair with a net.Listen-related function

Available since
    2017.1
`

var docSA1021 = `Using bytes.Equal to compare two net.IP

A net.IP stores an IPv4 or IPv6 address as a slice of bytes. The
length of the slice for an IPv4 address, however, can be either 4 or
16 bytes long, using different ways of representing IPv4 addresses. In
order to correctly compare two net.IPs, the net.IP.Equal method should
be used, as it takes both representations into account.

Available since
    2017.1
`

var docSA1023 = `Modifying the buffer in an io.Writer implementation

Write must not modify the slice data, even temporarily.

Available since
    2017.1
`

var docSA1024 = `A string cutset contains duplicate characters, suggesting TrimPrefix or TrimSuffix should be used instead of TrimLeft or TrimRight

Available since
    2017.1
`

var docSA1025 = `It is not possible to use Reset's return value correctly

Available since
    2019.1
`

var docSA1026 = `Cannot marshal channels or functions

Available since
    Unreleased
`

var docSA1027 = `Atomic access to 64-bit variable must be 64-bit aligned

On ARM, x86-32, and 32-bit MIPS, it is the caller's responsibility to
arrange for 64-bit alignment of 64-bit words accessed atomically. The
first word in a variable or in an allocated struct, array, or slice
can be relied upon to be 64-bit aligned.

You can use the structlayout tool to inspect the alignment of fields
in a struct.

Available since
    Unreleased
`

var docSA2000 = `sync.WaitGroup.Add called inside the goroutine, leading to a race condition

Available since
    2017.1
`

var docSA2001 = `Empty critical section, did you mean to defer the unlock?

Available since
    2017.1
`

var docSA2002 = `Called testing.T.FailNow or SkipNow in a goroutine, which isn't allowed

Available since
    2017.1
`

var docSA2003 = `Deferred Lock right after locking, likely meant to defer Unlock instead

Available since
    2017.1
`

var docSA3000 = `TestMain doesn't call os.Exit, hiding test failures

Test executables (and in turn 'go test') exit with a non-zero status
code if any tests failed. When specifying your own TestMain function,
it is your responsibility to arrange for this, by calling os.Exit with
the correct code. The correct code is returned by (*testing.M).Run, so
the usual way of implementing TestMain is to end it with
os.Exit(m.Run()).

Available since
    2017.1
`

var docSA3001 = `Assigning to b.N in benchmarks distorts the results

The testing package dynamically sets b.N to improve the reliability of
benchmarks and uses it in computations to determine the duration of a
single operation. Benchmark code must not alter b.N as this would
falsify results.

Available since
    2017.1
`

var docSA4000 = `Boolean expression has identical expressions on both sides

Available since
    2017.1
`

var docSA4001 = `&*x gets simplified to x, it does not copy x

Available since
    2017.1
`

var docSA4002 = `Comparing strings with known different sizes has predictable results

Available since
    2017.1
`

var docSA4003 = `Comparing unsigned values against negative values is pointless

Available since
    2017.1
`

var docSA4004 = `The loop exits unconditionally after one iteration

Available since
    2017.1
`

var docSA4005 = `Field assignment that will never be observed. Did you mean to use a pointer receiver?

Available since
    2017.1
`

var docSA4006 = `A value assigned to a variable is never read before being overwritten. Forgotten error check or dead code?

Available since
    2017.1
`

var docSA4008 = `The variable in the loop condition never changes, are you incrementing the wrong variable?

Available since
    2017.1
`

var docSA4009 = `A function argument is overwritten before its first use

Available since
    2017.1
`

var docSA4010 = `The result of append will never be observed anywhere

Available since
    2017.1
`

var docSA4011 = `Break statement with no effect. Did you mean to break out of an outer loop?

Available since
    2017.1
`

var docSA4012 = `Comparing a value against NaN even though no value is equal to NaN

Available since
    2017.1
`

var docSA4013 = `Negating a boolean twice (!!b) is the same as writing b. This is either redundant, or a typo.

Available since
    2017.1
`

var docSA4014 = `An if/else if chain has repeated conditions and no side-effects; if the condition didn't match the first time, it won't match the second time, either

Available since
    2017.1
`

var docSA4015 = `Calling functions like math.Ceil on floats converted from integers doesn't do anything useful

Available since
    2017.1
`

var docSA4016 = `Certain bitwise operations, such as x ^ 0, do not do anything useful

Available since
    2017.1
`

var docSA4017 = `A pure function's return value is discarded, making the call pointless

Available since
    2017.1
`

var docSA4018 = `Self-assignment of variables

Available since
    2017.1
`

var docSA4019 = `Multiple, identical build constraints in the same file

Available since
    2017.1
`

var docSA4020 = `Unreachable case clause in a type switch

In a type switch like the following

    type T struct{}
    func (T) Read(b []byte) (int, error) { return 0, nil }

    var v interface{} = T{}

    switch v.(type) {
    case io.Reader:
        // ...
    case T:
        // unreachable
    }

the second case clause can never be reached because T implements
io.Reader and case clauses are evaluated in source order.

Another example:

    type T struct{}
    func (T) Read(b []byte) (int, error) { return 0, nil }
    func (T) Close() error { return nil }

    var v interface{} = T{}

    switch v.(type) {
    case io.Reader:
        // ...
    case io.ReadCloser:
        // unreachable
    }

Even though T has a Close method and thus implements io.ReadCloser,
io.Reader will always match first. The method set of io.Reader is a
subset of io.ReadCloser. Thus it is impossible to match the second
case without mtching the first case.


Structurally equivalent interfaces

A special case of the previous example are structurally identical
interfaces. Given these declarations

    type T error
    type V error

    func doSomething() error {
        err, ok := doAnotherThing()
        if ok {
            return T(err)
        }

        return U(err)
    }

the following type switch will have an unreachable case clause:

    switch doSomething().(type) {
    case T:
        // ...
    case V:
        // unreachable
    }

T will always match before V because they are structurally equivalent
and therefore doSomething()'s return value implements both.

Available since
    Unreleased
`

var docSA4021 = `x = append(y) is equivalent to x = y

Available since
    Unreleased
`

var docSA5000 = `Assignment to nil map

Available since
    2017.1
`

var docSA5001 = `Defering Close before checking for a possible error

Available since
    2017.1
`

var docSA5002 = `The empty for loop (for {}) spins and can block the scheduler

Available since
    2017.1
`

var docSA5003 = `Defers in infinite loops will never execute

Defers are scoped to the surrounding function, not the surrounding
block. In a function that never returns, i.e. one containing an
infinite loop, defers will never execute.

Available since
    2017.1
`

var docSA5004 = `for { select { ... with an empty default branch spins

Available since
    2017.1
`

var docSA5005 = `The finalizer references the finalized object, preventing garbage collection

A finalizer is a function associated with an object that runs when the
garbage collector is ready to collect said object, that is when the
object is no longer referenced by anything.

If the finalizer references the object, however, it will always remain
as the final reference to that object, preventing the garbage
collector from collecting the object. The finalizer will never run,
and the object will never be collected, leading to a memory leak. That
is why the finalizer should instead use its first argument to operate
on the object. That way, the number of references can temporarily go
to zero before the object is being passed to the finalizer.

Available since
    2017.1
`

var docSA5006 = `Slice index out of bounds

Available since
    2017.1
`

var docSA5007 = `Infinite recursive call

A function that calls itself recursively needs to have an exit
condition. Otherwise it will recurse forever, until the system runs
out of memory.

This issue can be caused by simple bugs such as forgetting to add an
exit condition. It can also happen "on purpose". Some languages have
tail call optimization which makes certain infinite recursive calls
safe to use. Go, however, does not implement TCO, and as such a loop
should be used instead.

Available since
    2017.1
`

var docSA6000 = `Using regexp.Match or related in a loop, should use regexp.Compile

Available since
    2017.1
`

var docSA6001 = `Missing an optimization opportunity when indexing maps by byte slices

Map keys must be comparable, which precludes the use of byte slices.
This usually leads to using string keys and converting byte slices to
strings.

Normally, a conversion of a byte slice to a string needs to copy the data and
causes allocations. The compiler, however, recognizes m[string(b)] and
uses the data of b directly, without copying it, because it knows that
the data can't change during the map lookup. This leads to the
counter-intuitive situation that

    k := string(b)
    println(m[k])
    println(m[k])

will be less efficient than

    println(m[string(b)])
    println(m[string(b)])

because the first version needs to copy and allocate, while the second
one does not.

For some history on this optimization, check out commit
f5f5a8b6209f84961687d993b93ea0d397f5d5bf in the Go repository.

Available since
    2017.1
`

var docSA6002 = `Storing non-pointer values in sync.Pool allocates memory

A sync.Pool is used to avoid unnecessary allocations and reduce the
amount of work the garbage collector has to do.

When passing a value that is not a pointer to a function that accepts
an interface, the value needs to be placed on the heap, which means an
additional allocation. Slices are a common thing to put in sync.Pools,
and they're structs with 3 fields (length, capacity, and a pointer to
an array). In order to avoid the extra allocation, one should store a
pointer to the slice instead.

See the comments on https://go-review.googlesource.com/c/go/+/24371
that discuss this problem.

Available since
    2017.1
`

var docSA6003 = `Converting a string to a slice of runes before ranging over it

You may want to loop over the runes in a string. Instead of converting
the string to a slice of runes and looping over that, you can loop
over the string itself. That is,

    for _, r := range s {}

and

    for _, r := range []rune(s) {}

will yield the same values. The first version, however, will be faster
and avoid unnecessary memory allocations.

Do note that if you are interested in the indices, ranging over a
string and over a slice of runes will yield different indices. The
first one yields byte offsets, while the second one yields indices in
the slice of runes.

Available since
    2017.1
`

var docSA6005 = `Inefficient string comparison with strings.ToLower or strings.ToUpper

Converting two strings to the same case and comparing them like so

    if strings.ToLower(s1) == strings.ToLower(s2) {
        ...
    }

is significantly more expensive than comparing them with
strings.EqualFold(s1, s2). This is due to memory usage as well as
computational complexity.

strings.ToLower will have to allocate memory for the new strings, as
well as convert both strings fully, even if they differ on the very
first byte. strings.EqualFold, on the other hand, compares the strings
one character at a time. It doesn't need to create two intermediate
strings and can return as soon as the first non-matching character has
been found.

For a more in-depth explanation of this issue, see
https://blog.digitalocean.com/how-to-efficiently-compare-strings-in-go/

Available since
    Unreleased
`

var docSA9001 = `Defers in 'for range' loops may not run when you expect them to

Available since
    2017.1
`

var docSA9002 = `Using a non-octal os.FileMode that looks like it was meant to be in octal.

Available since
    2017.1
`

var docSA9003 = `Empty body in an if or else branch

Available since
    2017.1
`

var docSA9004 = `Only the first constant has an explicit type

In a constant declaration such as the following:

    const (
        First byte = 1
        Second     = 2
    )

the constant Second does not have the same type as the constant First.
This construct shouldn't be confused with

    const (
        First byte = iota
        Second
    )

where First and Second do indeed have the same type. The type is only
passed on when no explicit value is assigned to the constant.

When declaring enumerations with explicit values it is therefore
important not to write

    const (
          EnumFirst EnumType = 1
          EnumSecond         = 2
          EnumThird          = 3
    )

This discrepancy in types can cause various confusing behaviors and
bugs.


Wrong type in variable declarations

The most obvious issue with such incorrect enumerations expresses
itself as a compile error:

package pkg

    const (
        EnumFirst  uint8 = 1
        EnumSecond       = 2
    )

    func fn(useFirst bool) {
        x := EnumSecond
        if useFirst {
            x = EnumFirst
        }
    }

fails to compile with

    ./const.go:11:5: cannot use EnumFirst (type uint8) as type int in assignment


Losing method sets

A more subtle issue occurs with types that have methods and optional
interfaces. Consider the following:

    package main

    import "fmt"

    type Enum int

    func (e Enum) String() string {
        return "an enum"
    }

    const (
        EnumFirst  Enum = 1
        EnumSecond      = 2
    )

    func main() {
        fmt.Println(EnumFirst)
        fmt.Println(EnumSecond)
    }

This code will output

    an enum
    2

as EnumSecond has no explicit type, and thus defaults to int.

Available since
    2019.1
`

var docSA9005 = `Trying to marshal a struct with no public fields nor custom marshaling

The encoding/json and encoding/xml packages only operate on exported
fields in structs, not unexported ones. It is usually an error to try
to (un)marshal structs that only consist of unexported fields.

This check will not flag calls involving types that define custom
marshaling behavior, e.g. via MarshalJSON methods. It will also not
flag empty structs.

Available since
    Unreleased
`
