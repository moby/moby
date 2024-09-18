Docker Engine Documentation
===========================

The man pages for Docker Engine are generated from the markdown sources and tooling in this directory.

## Generate the man pages

Run `make` from within this directory.
A Go toolchain is required.
The generated man pages will be placed in man*N* subdirectories, where *N* is the manual section number.

## Install the man pages

Run `make install` from within this directory.
The make variables `prefix`, `mandir`, `INSTALL`, `INSTALL_DATA` and `DESTDIR`
are supported for customizing the installation.

## Add a new man page

Create a new Markdown file in this directory with a filename *TITLE*.*SECTION*.md,
where *TITLE* is the man page title and *SECTION* is the section number.
The Makefile will pick it up automatically.

The Makefile ignores Markdown files that do not match the glob `*.*.md`,
allowing non-manpage documentation (like this README file) to coexist.
