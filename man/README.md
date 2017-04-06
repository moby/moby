Docker Documentation
====================

This directory contains scripts for generating the man pages. Many of the man
pages are generated directly from the `spf13/cobra` `Command` definition. Some
legacy pages are still generated from the markdown files in this directory.
Do *not* edit the man pages in the man1 directory. Instead, update the
Cobra command or amend the Markdown files for legacy pages.


## Generate the man pages

From within the project root directory run:

    make manpages
