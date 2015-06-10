Docker Documentation
====================

This directory contains the Docker user manual in the Markdown format.
Do *not* edit the man pages in the man1 directory. Instead, amend the
Markdown (*.md) files.

# Generating man pages from the Markdown files

The recommended approach for generating the man pages is via a Docker
container using the supplied `Dockerfile` to create an image with the correct
environment. This uses `go-md2man`, a pure Go Markdown to man page generator.

### Generate the man pages 

On Linux installations, Docker includes a set of man pages you can access by typing `man command-name` on the command line. For example, `man docker` displays the `docker` man page. When using Docker on Mac OSX the man pages are not automatically included. 

You can generate and install the `man` pages yourself by following these steps:

1. Checkout the `docker` source. 

        $ git clone https://github.com/docker/docker.git
        
  If you are using Boot2Docker, you must clone into your `/Users` directory
  because Boot2Docker can only share this path with the docker containers.
		
2. Build the docker image.
   
        $ cd docker/man
        $ docker build -t docker/md2man .

3. Build the man pages.

        $ docker run -v <path-to-git-dir>/docker/man:/man:rw -w /man -i docker/md2man /man/md2man-all.sh
        
  The `md2man` Docker container processes the Markdown files and generates
 a `man1` and `man5` subdirectories in the `docker/man` directory. 

4. Copy the generated man pages to `/usr/share/man`

        $ cp -R man* /usr/share/man/



