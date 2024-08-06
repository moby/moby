go-md2man 1 "January 2015" go-md2man "User Manual"
==================================================

# NAME
go-md2man - Convert markdown files into manpages

# SYNOPSIS
**go-md2man** [**-in**=*/path/to/md/file*] [**-out**=*/path/to/output*]

# DESCRIPTION
**go-md2man** converts standard markdown formatted documents into manpages. It is
written purely in Go so as to reduce dependencies on 3rd party libs.

By default, the input is stdin and the output is stdout.

# EXAMPLES
Convert the markdown file *go-md2man.1.md* into a manpage:
```
go-md2man < go-md2man.1.md > go-md2man.1
```

Same, but using command line arguments instead of shell redirection:
```
go-md2man -in=go-md2man.1.md -out=go-md2man.1
```

# HISTORY
January 2015, Originally compiled by Brian Goff (cpuguy83@gmail.com).
