
A _bundle manifest_ contains metadata to allow recreation of filesystem images
invariant to the method of transport. Whether using rsync, tar, unzip or other
method of transmitting the image, the _bundle manifest_ can be applied to
recover any filesystem metadata lost by the transport. Not only does this
provide a consistent base describing the contents of the bundle, but because
it is stable in output, the manifest can be hashed and signed to aid in post-
transport verification. Coupled with a transport verification, these
facilities provide the components of a reliable.

A manifest is simply a protobuf that contains a list of _entries_ sorted by
`path`. Each _entry_, identified by the `path` relative to the bundle root,
contains at least the user, group, and file mode fields. Depending on the
_resource type_, futher fields are populated. The details of those fields are
described later in this document.

# Building a Manifest

# Applying a Manifest

## Common Properties

### path

The `path` field identifies an _Entry_, specifying the path relative to the
root of the bundle. A well-formed _bundle manifest_ entry list should

### user
### group
### uid
### gid

This is a string field:
- for unices, this is always an integer
- for windows, this is a group SID (??)

### mode

Based on http://golang.org/pkg/os/#FileMode. 

> _TODO(stevvooe):_ Add a description here.

# Resource Types

## Regular Files

Regular files describe

In aA regular file has the `digest` field populated in the 

## Hard Links

### Build Phase

Identifying the canonical linked file when processing hard links is, by
definition, not possible. Hard linked files are literally the same file with
different paths. There is no reliable way to always select the same, common
link target.

Because of this, hardlinked files are processed as sets, partitioned by the
device and inode numbers. After the main filesystem walk completes, the
hardlink sets are post-processed. To ensure stable manifest generation, each
set is sorted by `path` and the first element is added to the manifest as a
regular file, with a `digest`. Any remaining valid entries become hardlinks.

Entries that become hard links have their `target` field set rather than the
`digest` field. Unlike symbolic links, the `target` field is always relative
to the bundle root. For all other attributes, the values are be identical to a
regular file.

### Apply Phase

## Symbolic Links
## Device Nodes
## Named Pipes