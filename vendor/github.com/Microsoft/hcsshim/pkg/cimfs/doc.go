/*
This package provides simple go wrappers on top of the win32 CIMFS APIs.

Details about CimFS & related win32 APIs can be found here:
https://learn.microsoft.com/en-us/windows/win32/api/_cimfs/

Details about how CimFS is being used in containerd can be found here:
https://github.com/containerd/containerd/issues/8346

CIM types:
Currently we support 2 types of CIMs:
  - Standard/classic (for the lack of a better term) CIMs.
  - Block CIMs.

Standard CIMs store all the contents of a CIM in one or more region & objectID files. This
means a single CIM is made up of a `.cim` file, one or more region files and one or more
objectID files. All of these files MUST be present in the same directory in order for that
CIM to work. Block CIMs store all the data of a CIM in a single block device. A VHD can be
such a block device. For convenience CimFS also allows using a block formatted file as a
block device.

Standard CIMs can be created with the `func Create(imagePath string, oldFSName string,
newFSName string) (_ *CimFsWriter, err error)` function defined in this package, whereas
block CIMs can be created with the `func CreateBlockCIM(blockPath, oldName, newName
string, blockType BlockCIMType) (_ *CimFsWriter, err error)` function.

Verified CIMs:
A block CIM can also provide integrity checking (via a hash/Merkel tree,
similar to dm-verity on Linux). If a CIM is written and sealed, it generates a
root hash of all of its contents and shares it back with the client. Any
verified CIM can be mounted by passing a hash that we expect to be its root
hash. All read operations on such a mounted CIM will then validate that the
generated root hash matches with the one that was provided at mount time. If it
doesn't match the read fails. This allows us to guarantee that the CIM based
layered aren't being modified underneath us.

Forking & Merging CIMs:
In container world, CIMs are used for storing container image layers. Usually, one layer
is stored in one CIM. This means we need a way to combine multiple CIMs to create the
rootfs of a container. This can be achieved either by forking the CIMs or merging the
CIMs.

Forking CIMs:
Forking means every time a CIM is created for a non-base layer, we fork it off of a parent
layer CIM. This ensures that contents that are written to this CIM are merged with that of
parent layer CIMs at the time of CIM creation itself. When such a CIM is mounted we get a
combined view of the contents of this CIM as well as the parent CIM from which this CIM
was forked. However, this means that all the CIMs MUST be stored in the same directory in
order for forked CIMs to work. And every non-base layer CIM is dependent on all of its
parent layer CIMs.

Merging CIMs:
If we create one or more CIMs without forking them at the time of creation, we can still
merge those CIMs later to create a new special type of CIM called merged CIM. When
mounted, this merged CIM provides a view of the combined contents of all the layers that
were merged. The advantage of this approach is that each layer CIM (also referred to as
source CIMs in the context of merging CIMs) can be created & stored independent of its
parent CIMs. (Currently we only support merging block CIMs).

In order to create a merged CIM we need at least 2 non-forked block CIMs (we can not merge
forked & non-forked CIMs), these CIMs are also referred to as source CIMs. We first create
a new CIM (for storing the merge) via the `CreateBlockCIM` API, then call
`CimAddFsToMergedImage2` repeatedly to add the source CIMs one by one to the merged
CIM. Closing the handle on this new CIM commits it automatically. The order in which
source CIMs are added matters. A source CIM that was added before another source CIM takes
precedence when merging the CIM contents. Crating this merged CIM only combines the
metadata of all the source CIMs, however the actual data isn't copied to the merged
CIM. This is why when mounting the merged CIM, we still need to provide paths to the
source CIMs.

`CimMergeMountImage` is used to mount a merged CIM. This API expects an array of paths of
the merged CIM and all the source CIMs. Note that the array MUST include the merged CIM
path at the 0th index and all the source CIMs in the same order in which they were added
at the time of creation of the merged CIM. For example, if we merged CIMs 1.cim & 2.cim by
first adding 1.cim (via CimAddFsToMergedImage) and then adding 2.cim, then the array
should be [merged.cim, 1.cim, 2.cim]

Merged CIM specific APIs.

`CimTombstoneFile`: is used for creating a tombstone file in a CIM. Tombstone file is
similar to a whiteout file used in case of overlayFS. A tombstone's primary use case is
for merged CIMs. When multiple source CIMs are merged, a tombstone file/directory ensures
that any files with the same path in the lower layers (i.e source CIMs that are added
after the CIM that has a tombstone) do not show up in the mounted filesystem view.  For
example, imagine 1.cim has a file at path `foo/bar.txt` and 2.cim has a tombstone at path
`foo/bar.txt`. If a merged CIM is created by first adding 2.cim (via
CimAddFsToMergedImage) and then adding 1.cim and then when that merged CIM is mounted,
`foo/bar.txt` will not show up in the mounted filesystem.  A tombstone isn't required when
using forked CIMs, because we can just call `CimDeletePath` to remove a file from the
lower layers in that case. However, that doesn't work for merged CIMs since at the time of
writing one of the source CIMs, we can't delete files from other source CIMs.

`CimCreateMergeLink`: is used to create a file link that is resolved at the time of
merging CIMs. This is required if we want to create a hardlink in one source CIM that
points to a file in another source CIM. Such a hardlink can not be resolved at the time of
writing the source CIM. It can only be resolved at the time of merge. This API allows us
to create such cross layer hard links.
*/
package cimfs
