Export the contents of a container's filesystem using the full or shortened
container ID or container name. The output is exported to STDOUT and can be
redirected to a tar file.

Stream to a file instead of STDOUT by using **-o**.

# EXAMPLES
Export the contents of the container called angry_bell to a tar file
called angry_bell.tar:

    # docker export angry_bell > angry_bell.tar
    # docker export --output=angry_bell-latest.tar angry_bell
    # ls -sh angry_bell.tar
    321M angry_bell.tar
    # ls -sh angry_bell-latest.tar
    321M angry_bell-latest.tar

# See also
**docker-import(1)** to create an empty filesystem image
and import the contents of the tarball into it, then optionally tag it.
