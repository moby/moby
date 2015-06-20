Flow of TAR stream
==================

The underlying use of `github.com/vbatts/tar-split/archive/tar` is most similar
to stdlib.


Packer interface
----------------

For ease of storage and usage of the raw bytes, there will be a storage
interface, that accepts an io.Writer (This way you could pass it an in memory
buffer or a file handle).

Having a Packer interface can allow configuration of hash.Hash for file payloads
and providing your own io.Writer.

Instead of having a state directory to store all the header information for all
Readers, we will leave that up to user of Reader. Because we can not assume an
ID for each Reader, and keeping that information differentiated.



State Directory
---------------

Perhaps we could deduplicate the header info, by hashing the rawbytes and
storing them in a directory tree like:

	./ac/dc/beef

Then reference the hash of the header info, in the positional records for the
tar stream. Though this could be a future feature, and not required for an
initial implementation. Also, this would imply an owned state directory, rather
than just writing storage info to an io.Writer.

