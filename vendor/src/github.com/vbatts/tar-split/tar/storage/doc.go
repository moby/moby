/*
Package storage is for metadata of a tar archive.

Packing and unpacking the Entries of the stream. The types of streams are
either segments of raw bytes (for the raw headers and various padding) and for
an entry marking a file payload.

The raw bytes are stored precisely in the packed (marshalled) Entry, whereas
the file payload marker include the name of the file, size, and crc64 checksum
(for basic file integrity).
*/
package storage
