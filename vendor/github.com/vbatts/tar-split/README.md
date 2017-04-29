# tar-split

[![Build Status](https://travis-ci.org/vbatts/tar-split.svg?branch=master)](https://travis-ci.org/vbatts/tar-split)

Pristinely disassembling a tar archive, and stashing needed raw bytes and offsets to reassemble a validating original archive.

## Docs

Code API for libraries provided by `tar-split`:

* https://godoc.org/github.com/vbatts/tar-split/tar/asm
* https://godoc.org/github.com/vbatts/tar-split/tar/storage
* https://godoc.org/github.com/vbatts/tar-split/archive/tar

## Install

The command line utilitiy is installable via:

```bash
go get github.com/vbatts/tar-split/cmd/tar-split
```

## Usage

For cli usage, see its [README.md](cmd/tar-split/README.md).
For the library see the [docs](#docs)

## Demo

### Basic disassembly and assembly

This demonstrates the `tar-split` command and how to assemble a tar archive from the `tar-data.json.gz`


![basic cmd demo thumbnail](https://i.ytimg.com/vi/vh5wyjIOBtc/2.jpg?time=1445027151805)
[youtube video of basic command demo](https://youtu.be/vh5wyjIOBtc)

### Docker layer preservation

This demonstrates the tar-split integration for docker-1.8. Providing consistent tar archives for the image layer content.

![docker tar-split demo](https://i.ytimg.com/vi_webp/vh5wyjIOBtc/default.webp)
[youtube vide of docker layer checksums](https://youtu.be/tV_Dia8E8xw)

## Caveat

Eventually this should detect TARs that this is not possible with.

For example stored sparse files that have "holes" in them, will be read as a
contiguous file, though the archive contents may be recorded in sparse format.
Therefore when adding the file payload to a reassembled tar, to achieve
identical output, the file payload would need be precisely re-sparsified. This
is not something I seek to fix imediately, but would rather have an alert that
precise reassembly is not possible.
(see more http://www.gnu.org/software/tar/manual/html_node/Sparse-Formats.html)


Other caveat, while tar archives support having multiple file entries for the
same path, we will not support this feature. If there are more than one entries
with the same path, expect an err (like `ErrDuplicatePath`) or a resulting tar
stream that does not validate your original checksum/signature.

## Contract

Do not break the API of stdlib `archive/tar` in our fork (ideally find an upstream mergeable solution).

## Std Version

The version of golang stdlib `archive/tar` is from go1.6
It is minimally extended to expose the raw bytes of the TAR, rather than just the marshalled headers and file stream.


## Design

See the [design](concept/DESIGN.md).

## Stored Metadata

Since the raw bytes of the headers and padding are stored, you may be wondering
what the size implications are. The headers are at least 512 bytes per
file (sometimes more), at least 1024 null bytes on the end, and then various
padding. This makes for a constant linear growth in the stored metadata, with a
naive storage implementation.

First we'll get an archive to work with. For repeatability, we'll make an
archive from what you've just cloned:

```bash
git archive --format=tar -o tar-split.tar HEAD .
```

```bash
$ go get github.com/vbatts/tar-split/cmd/tar-split
$ tar-split checksize ./tar-split.tar
inspecting "tar-split.tar" (size 210k)
 -- number of files: 50
 -- size of metadata uncompressed: 53k
 -- size of gzip compressed metadata: 3k
```

So assuming you've managed the extraction of the archive yourself, for reuse of
the file payloads from a relative path, then the only additional storage
implications are as little as 3kb.

But let's look at a larger archive, with many files.

```bash
$ ls -sh ./d.tar
1.4G ./d.tar
$ tar-split checksize ~/d.tar 
inspecting "/home/vbatts/d.tar" (size 1420749k)
 -- number of files: 38718
 -- size of metadata uncompressed: 43261k
 -- size of gzip compressed metadata: 2251k
```

Here, an archive with 38,718 files has a compressed footprint of about 2mb.

Rolling the null bytes on the end of the archive, we will assume a
bytes-per-file rate for the storage implications.

| uncompressed | compressed |
| :----------: | :--------: |
| ~ 1kb per/file | 0.06kb per/file |


## What's Next?

* More implementations of storage Packer and Unpacker
* More implementations of FileGetter and FilePutter
* would be interesting to have an assembler stream that implements `io.Seeker`


## License

See [LICENSE](LICENSE)

