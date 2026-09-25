asm
===

This library for assembly and disassembly of tar archives, facilitated by
`github.com/vbatts/tar-split/tar/storage`.


Concerns
--------

For completely safe assembly/disassembly, there will need to be a Content
Addressable Storage (CAS) directory, that maps to a checksum in the
`storage.Entity` of `storage.FileType`.

This is due to the fact that tar archives _can_ allow multiple records for the
same path, but the last one effectively wins. Even if the prior records had a
different payload. 

In this way, when assembling an archive from relative paths, if the archive has
multiple entries for the same path, then all payloads read in from a relative
path would be identical.


Thoughts
--------

Have a look-aside directory or storage. This way when a clobbering record is
encountered from the tar stream, then the payload of the prior/existing file is
stored to the CAS. This way the clobbering record's file payload can be
extracted, but we'll have preserved the payload needed to reassemble a precise
tar archive.

clobbered/path/to/file.[0-N]

*alternatively*

We could just _not_ support tar streams that have clobbering file paths.
Appending records to the archive is not incredibly common, and doesn't happen
by default for most implementations.  Not supporting them wouldn't be a
security concern either, as if it did occur, we would reassemble an archive
that doesn't validate signature/checksum, so it shouldn't be trusted anyway.

Otherwise, this will allow us to defer support for appended files as a FUTURE FEATURE.

