:title: Attach stream API
:description: API Documentation for the Attach command in Docker
:keywords: API, Docker, Attach, Stream, REST, documentation

=================
Docker Attach stream API
=================

.. contents:: Table of Contents

1. Brief introduction
=====================

- This is the Attach stream API for Docker

2. Format
=========

The attach format is a Header and a Payload (frame).

2.1 Header
^^^^^^^^^^

The header will contain the information on which stream write
the stream (stdout or stderr).
It also contain the size of the associated frame encoded on the last 4 bytes (uint32).

It is encoded on the first 8 bytes like this:
header := [8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}

STREAM_TYPE can be:
- 0: stdin (will be writen on stdout)
- 1: stdout
- 2: stderr

SIZE1, SIZE2, SIZE3, SIZE4 are the 4 bytes of the uint32 size.

2.1 Payload (frame)
^^^^^^^^^^^^^^^^^^^

The payload is the raw stream.

3. Implementation
=================

The simplest way to implement the Attach protocol is the following:

1) Read 8 bytes
2) chose stdout or stderr depending on the first byte
3) Extract the frame size from the last 4 byets
4) Read the extracted size and output it on the correct output
5) Goto 1)
