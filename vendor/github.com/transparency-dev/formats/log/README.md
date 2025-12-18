# Checkpoint format

This directory contains a description and supporting golang code for
a reusable Checkpoint format which the TrustFabric team uses in various
projects.

The format itself is heavily based on the
[golang sumbdb head](https://sum.golang.org/latest), and corresponding
[signed note](https://pkg.go.dev/golang.org/x/mod/sumdb/note) formats, 
and consists of two parts: a signed envelope, and a body.

### Signed envelope

The envelope (signed note) is of the form:

* One or more non-empty lines, each terminated by `\n` (the `body`)
* One line consisting of just one `\n` (i.e. a blank line)
* One or more `signature` lines, each terminated by `\n`

All signatures commit to the body only (including its trailing newline, but not
the blank line's newline - see below for an example).

The signature(s) themselves are in the sumdb note format (concrete example
[below](#example)):

`– <identity> <key_hint+signature_bytes>`
where:

* `–` is an emdash (U+2014)
* `<identity>` gives a human-readable representation of the signing ID

and the `signature_bytes` are prefixed with the first 4 bytes of the SHA256 hash
of the associated public key to act as a hint in identifying the correct key to
verify with.

For guidance on generating keys, see the
[note documentation](https://pkg.go.dev/golang.org/x/mod/sumdb/note#hdr-Generating_Keys)
and [implementation](https://cs.opensource.google/go/x/mod/+/master:sumdb/note/note.go;l=368;drc=ed3ec21bb8e252814c380df79a80f366440ddb2d).
Of particular note is that the public key and its hash commit to the algorithm
identifier.

**Differences from sumdb note:**
Whereas the golang signed note *implementation* currently supports only Ed25519
signatures, the format itself is not restricted to this scheme.

### Checkpoint body

The checkpoint body is of the form:

```text
<Origin string>
<Decimal log size>
<Base64 log root hash>
[otherdata]
```

The first 3 lines of the body **MUST** be present in all Checkpoints.

* `<Origin string>` should be a unique identifier for the log identity which issued the checkpoint.
  The format SHOULD be a URI-like structure like `<dns_name>[/<suffix>]`, where the log operator
  controls `<dns_name>`, e.g `example.com/log42`. This is only a recommendation, and clients MUST
  NOT assume that the origin is a URI following this format. This structure reduces the likelihood
  of origin collision, and gives clues to humans about the log operator and what is in the log. The
  suffix is optional and can be anything. It is used to disambiguate logs owned under the same
  prefix.

  The presence of this identifier forms part of the log claim, and guards against two
  logs producing bytewise identical checkpoints.

* `<Decimal log size>` is the ASCII decimal representation of the number of leaves committed
  to by this checkpoint. It should not have leading zeroes.

* `<Base64 log root hash>` is an
  [RFC4684 standard encoding](https://datatracker.ietf.org/doc/html/rfc4648#section-4) base-64
  representation of the log root hash at the specified log size.

* `[otherdata]` is opaque and optional, and, if necessary, can be used to tie extra
  data to the checkpoint, however its format must conform to the sumdb signed
  note spec (e.g. it must not contain blank lines.)

> Note that golang sumdb implementation is already compatible with this
`[otherdata]` extension (see
<https://github.com/golang/mod/blob/d6ab96f2441f9631f81862375ef66782fc4a9c12/sumdb/tlog/note.go#L52>).
If you plan to use `otherdata` in your log, see the section on [merging checkpoints](#merging-checkpoints).

The first signature on a checkpoint should be from the log which issued it, but there MUST NOT
be more than one signature from a log identity present on the checkpoint.

## Example

An annotated example signed checkpoint in this format is shown below:

![format](images/format.png)


This checkpoint was issued by the log known as "Moon Log", the log's size is
4027504, in the `other data` section a timestamp is encoded as a 64bit hex
value, and further application-specific data relating to the phase of the moon
at the point the checkpoint was issued is supplied following that.

## Merging Checkpoints

This checkpoint format allows a checkpoint that has been independently signed by
multiple identities to be merged, creating a single checkpoint with multiple
signatures. This is particularly useful for witnessing, where witnesses will
independently check consistency of the log and produce a counter-signed copy
containing two signatures: one for the log, and one for the witness.

The ability to merge signatures for the same body is a useful optimization.
Clients that require N witness signatures will not be required to fetch N checkpoints.
Instead they can fetch a single checkpoint and confirm it has the N required
signatures (in addition to the log signature).

Note that this optimization requires the checkpoint _body_ to be byte-equivalent.
The log signature does not need to be equal; when merging, only one of the log's
signatures over this body will be propagated. The core checkpoint format above
allows merging for any two consistent checkpoints for the same tree size.
However, if the `otherdata` extension is used then this can lead to checkpoints
that cannot be merged, even at the same tree size.

We recommend that log operators using `otherdata` consider carefully what
information is included in this. If data is included in `otherdata` that is not
fixed for a given tree size, then this can easily lead to unmergeable checkpoints.
The most commonly anticipated cause for this would be including the timestamp at
which the checkpoint was requested within the `otherdata`. In this case, no two
witnesses are likely to ever acquire the same checkpoint body. There may be cases
where this is unavoidable, but this consequence should be considered in the design.
