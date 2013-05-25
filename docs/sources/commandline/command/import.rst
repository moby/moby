:title: Import Command
:description: Create a new filesystem image from the contents of a tarball
:keywords: import, tarball, docker, url, documentation

==========================================================================
``import`` -- Create a new filesystem image from the contents of a tarball
==========================================================================

::

    Usage: docker import - IMAGE
    Example: tar zxvf image.tgz | docker import - new_image
    Import then contents of a tar archive as a new image
