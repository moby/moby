:title: Import Command
:description: Create a new filesystem image from the contents of a tarball
:keywords: import, tarball, docker, url, documentation

==========================================================================
``import`` -- Create a new filesystem image from the contents of a tarball
==========================================================================

::

    Usage: docker import URL|- [REPOSITORY [TAG]]

    Create a new filesystem image from the contents of a tarball

At this time, the URL must start with ``http`` and point to a single file archive
(.tar, .tar.gz, .tgz, .bzip, .tar.xz, .txz)
containing a root filesystem. If you would like to import from a local directory or archive,
you can use the ``-`` parameter to take the data from standard in.

Examples
--------

Import from a remote location
.............................

``$ docker import http://example.com/exampleimage.tgz exampleimagerepo``

Import from a local file
........................

Import to docker via pipe and standard in

``$ cat exampleimage.tgz | docker import - exampleimagelocal``

Import from a local directory
.............................

``$ sudo tar -c . | docker import - exampleimagedir``

Note the ``sudo`` in this example -- you must preserve the ownership of the files (especially root ownership)
during the archiving with tar. If you are not root (or sudo) when you tar, then the ownerships might not get preserved.
