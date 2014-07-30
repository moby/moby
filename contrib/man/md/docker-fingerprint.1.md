% DOCKER(1) Docker User Manuals
% W. Trevor King
% MAY 2014
# NAME
docker-fingerprint - Show the fingerprint of an image

# SYNOPSIS
**docker fingerprint** IMAGE

# DESCRIPTION

Show the fingerprint of an image.  You can use this fingerprint for
signing images, and for verifying signatures made by others.  The
format of a fingerprint is:

    <namespace>/<name>:<tag>
    <metadata>
    <tarsum>

signing the fingerprint asserts that that `<namespace>/<name>:<tag>`
name applies to the image with that metadata and tarsum.

# EXAMPLE

Sign with:

    $ docker fingerprint debian:7.4 | gpg --detach-sign --armor > debian-7.4.sig

After transmitting the detached signature and image layers however you
like (possibly through insecure channels), you can verify the
signature with:

    $ docker fingerprint debian:7.4 | gpg --verify debian-7.4.sig -

which will use your web of trust to verify the image.

# HISTORY

May 2014.  Originally compiled by W. Trevor King (wking at
tremily.us).
