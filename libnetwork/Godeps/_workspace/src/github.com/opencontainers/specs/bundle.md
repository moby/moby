# Bundle Container Format

This section defines a format for encoding a container as a *bundle* - a directory organized in a certain way, and containing all the necessary data and metadata for any compliant runtime to perform all standard operations against it. See also [OS X application bundles](http://en.wikipedia.org/wiki/Bundle_%28OS_X%29) for a similar use of the term *bundle*.

The format does not define distribution. In other words, it only specifies how a container must be stored on a local filesystem, for consumption by a runtime. It does not specify how to transfer a container between computers, how to discover containers, or assign names or versions to them. Any distribution method capable of preserving the original layout of a container, as specified here, is considered compliant.

A standard container bundle is made of the following 3 parts:

- A top-level directory holding everything else
- One or more content directories
- A configuration file

# Directory layout

A Standard Container bundle is a directory containing all the content needed to load and run a container. This includes its configuration file (`config.json`) and content directories. The main property of this directory layout is that it can be moved as a unit to another machine and run the same container.

The syntax and semantics for `config.json` are described in [this specification](config.md).

One or more *content directories* may be adjacent to the configuration file. This must include at least the root filesystem (referenced in the configuration file by the *root* field) and may include other related content (signatures, other configs, etc.). The interpretation of these resources is specified in the configuration. The names of the directories may be arbitrary, but users should consider using conventional names as in the example below.

```
/
!
--- config.json
!
--- rootfs
!
--- signatures
```

