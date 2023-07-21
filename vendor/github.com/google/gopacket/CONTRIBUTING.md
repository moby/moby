Contributing To gopacket
========================

So you've got some code and you'd like it to be part of gopacket... wonderful!
We're happy to accept contributions, whether they're fixes to old protocols, new
protocols entirely, or anything else you think would improve the gopacket
library.  This document is designed to help you to do just that.

The first section deals with the plumbing:  how to actually get a change
submitted.

The second section deals with coding style... Go is great in that it
has a uniform style implemented by 'go fmt', but there's still some decisions
we've made that go above and beyond, and if you follow them, they won't come up
in your code review.

The third section deals with some of the implementation decisions we've made,
which may help you to understand the current code and which we may ask you to
conform to (or provide compelling reasons for ignoring).

Overall, we hope this document will help you to understand our system and write
great code which fits in, and help us to turn around on your code review quickly
so the code can make it into the master branch as quickly as possible.


How To Submit Code
------------------

We use github.com's Pull Request feature to receive code contributions from
external contributors.  See
https://help.github.com/articles/creating-a-pull-request/ for details on
how to create a request.

Also, there's a local script `gc` in the base directory of GoPacket that
runs a local set of checks, which should give you relatively high confidence
that your pull won't fail github pull checks.

```sh
go get github.com/google/gopacket
cd $GOROOT/src/pkg/github.com/google/gopacket
git checkout -b <mynewfeature>  # create a new branch to work from
... code code code ...
./gc  # Run this to do local commits, it performs a number of checks
```

To sum up:

* DO
    + Pull down the latest version.
    + Make a feature-specific branch.
    + Code using the style and methods discussed in the rest of this document.
    + Use the ./gc command to do local commits or check correctness.
    + Push your new feature branch up to github.com, as a pull request.
    + Handle comments and requests from reviewers, pushing new commits up to
      your feature branch as problems are addressed.
    + Put interesting comments and discussions into commit comments.
* DON'T
    + Push to someone else's branch without their permission.


Coding Style
------------

* Go code must be run through `go fmt`, `go vet`, and `golint`
* Follow http://golang.org/doc/effective_go.html as much as possible.
    + In particular, http://golang.org/doc/effective_go.html#mixed-caps.  Enums
      should be be CamelCase, with acronyms capitalized (TCPSourcePort, vs.
      TcpSourcePort or TCP_SOURCE_PORT).
* Bonus points for giving enum types a String() field.
* Any exported types or functions should have commentary
  (http://golang.org/doc/effective_go.html#commentary)


Coding Methods And Implementation Notes
---------------------------------------

### Error Handling

Many times, you'll be decoding a protocol and run across something bad, a packet
corruption or the like.  How do you handle this?  First off, ALWAYS report the
error.  You can do this either by returning the error from the decode() function
(most common), or if you're up for it you can implement and add an ErrorLayer
through the packet builder (the first method is a simple shortcut that does
exactly this, then stops any future decoding).

Often, you'll already have decode some part of your protocol by the time you hit
your error.  Use your own discretion to determine whether the stuff you've
already decoded should be returned to the caller or not:

```go
func decodeMyProtocol(data []byte, p gopacket.PacketBuilder) error {
  prot := &MyProtocol{}
  if len(data) < 10 {
    // This error occurred before we did ANYTHING, so there's nothing in my
    // protocol that the caller could possibly want.  Just return the error.
    return fmt.Errorf("Length %d less than 10", len(data))
  }
  prot.ImportantField1 = data[:5]
  prot.ImportantField2 = data[5:10]
  // At this point, we've already got enough information in 'prot' to
  // warrant returning it to the caller, so we'll add it now.
  p.AddLayer(prot)
  if len(data) < 15 {
    // We encountered an error later in the packet, but the caller already
    // has the important info we've gleaned so far.
    return fmt.Errorf("Length %d less than 15", len(data))
  }
  prot.ImportantField3 = data[10:15]
  return nil  // We've already added the layer, we can just return success.
}
```

In general, our code follows the approach of returning the first error it
encounters.  In general, we don't trust any bytes after the first error we see.

### What Is A Layer?

The definition of a layer is up to the discretion of the coder.  It should be
something important enough that it's actually useful to the caller (IE: every
TLV value should probably NOT be a layer).  However, it can be more granular
than a single protocol... IPv6 and SCTP both implement many layers to handle the
various parts of the protocol.  Use your best judgement, and prepare to defend
your decisions during code review. ;)

### Performance

We strive to make gopacket as fast as possible while still providing lots of
features.  In general, this means:

* Focus performance tuning on common protocols (IP4/6, TCP, etc), and optimize
  others on an as-needed basis (tons of MPLS on your network?  Time to optimize
  MPLS!)
* Use fast operations.  See the toplevel benchmark_test for benchmarks of some
  of Go's underlying features and types.
* Test your performance changes!  You should use the ./gc script's --benchmark
  flag to submit any performance-related changes.  Use pcap/gopacket_benchmark
  to test your change against a PCAP file based on your traffic patterns.
* Don't be TOO hacky.  Sometimes, removing an unused struct from a field causes
  a huge performance hit, due to the way that Go currently handles its segmented
  stack... don't be afraid to clean it up anyway.  We'll trust the Go compiler
  to get good enough over time to handle this.  Also, this type of
  compiler-specific optimization is very fragile; someone adding a field to an
  entirely different struct elsewhere in the codebase could reverse any gains
  you might achieve by aligning your allocations.
* Try to minimize memory allocations.  If possible, use []byte to reference
  pieces of the input, instead of using string, which requires copying the bytes
  into a new memory allocation.
* Think hard about what should be evaluated lazily vs. not.  In general, a
  layer's struct should almost exactly mirror the layer's frame.  Anything
  that's more interesting should be a function.  This may not always be
  possible, but it's a good rule of thumb.
* Don't fear micro-optimizations.  With the above in mind, we welcome
  micro-optimizations that we think will have positive/neutral impacts on the
  majority of workloads.  A prime example of this is pre-allocating certain
  structs within a larger one:

```go
type MyProtocol struct {
  // Most packets have 1-4 of VeryCommon, so we preallocate it here.
  initialAllocation [4]uint32
  VeryCommon []uint32
}

func decodeMyProtocol(data []byte, p gopacket.PacketBuilder) error {
  prot := &MyProtocol{}
  prot.VeryCommon = proto.initialAllocation[:0]
  for len(data) > 4 {
    field := binary.BigEndian.Uint32(data[:4])
    data = data[4:]
    // Since we're using the underlying initialAllocation, we won't need to
    // allocate new memory for the following append unless we more than 16
    // bytes of data, which should be the uncommon case.
    prot.VeryCommon = append(prot.VeryCommon, field)
  }
  p.AddLayer(prot)
  if len(data) > 0 {
    return fmt.Errorf("MyProtocol packet has %d bytes left after decoding", len(data))
  }
  return nil
}
```

### Slices And Data

If you're pulling a slice from the data you're decoding, don't copy it.  Just
use the slice itself.

```go
type MyProtocol struct {
  A, B net.IP
}
func decodeMyProtocol(data []byte, p gopacket.PacketBuilder) error {
  p.AddLayer(&MyProtocol{
    A: data[:4],
    B: data[4:8],
  })
  return nil
}
```

The caller has already agreed, by using this library, that they won't modify the
set of bytes they pass in to the decoder, or the library has already copied the
set of bytes to a read-only location.  See DecodeOptions.NoCopy for more
information.

### Enums/Types

If a protocol has an integer field (uint8, uint16, etc) with a couple of known
values that mean something special, make it a type.  This allows us to do really
nice things like adding a String() function to them, so we can more easily
display those to users.  Check out layers/enums.go for one example, as well as
layers/icmp.go for layer-specific enums.

When naming things, try for descriptiveness over suscinctness.  For example,
choose DNSResponseRecord over DNSRR.
