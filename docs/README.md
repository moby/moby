Docker Documentation
====================

Overview
--------

The source for Docker documentation is here under ``sources/`` in the
form of .rst files. These files use
[reStructuredText](http://docutils.sourceforge.net/rst.html)
formatting with [Sphinx](http://sphinx-doc.org/) extensions for
structure, cross-linking and indexing.

The HTML files are built and hosted on
[readthedocs.org](https://readthedocs.org/projects/docker/), appearing
via proxy on https://docs.docker.io. The HTML files update
automatically after each change to the master or release branch of the
[docker files on GitHub](https://github.com/dotcloud/docker) thanks to
post-commit hooks. The "release" branch maps to the "latest"
documentation and the "master" branch maps to the "master"
documentation. 

**Warning**: The "master" documentation may include features not yet
part of any official docker release. "Master" docs should be used only
for understanding bleeding-edge development and "latest" should be
used for the latest official release.

If you need to manually trigger a build of an existing branch, then
you can do that through the [readthedocs
interface](https://readthedocs.org/builds/docker/). If you would like
to add new build targets, including new branches or tags, then you
must contact one of the existing maintainers and get your
readthedocs.org account added to the maintainers list, or just file an
issue on GitHub describing the branch/tag and why it needs to be added
to the docs, and one of the maintainers will add it for you.

Getting Started
---------------

To edit and test the docs, you'll need to install the Sphinx tool and
its dependencies. There are two main ways to install this tool:

###Native Installation

Install dependencies from `requirements.txt` file in your `docker/docs`
directory:

* Linux: `pip install -r docs/requirements.txt`

* Mac OS X: `[sudo] pip-2.7 -r docs/requirements.txt`

###Alternative Installation: Docker Container

If you're running ``docker`` on your development machine then you may
find it easier and cleaner to use the docs Dockerfile. This installs Sphinx
in a container, adds the local ``docs/`` directory and builds the HTML
docs inside the container, even starting a simple HTTP server on port
8000 so that you can connect and see your changes.

In the ``docker`` source directory, run:
    ```make docs```

This is the equivalent to ``make clean server`` since each container starts clean.

Usage
-----
* Follow the contribution guidelines (``../CONTRIBUTING.md``)
* Work in your own fork of the code, we accept pull requests.
* Change the ``.rst`` files with your favorite editor -- try to keep the
  lines short and respect RST and Sphinx conventions. 
* Run ``make clean docs`` to clean up old files and generate new ones,
  or just ``make docs`` to update after small changes.
* Your static website can now be found in the ``_build`` directory.
* To preview what you have generated run ``make server`` and open
  http://localhost:8000/ in your favorite browser.

``make clean docs`` must complete without any warnings or errors.

Working using GitHub's file editor
----------------------------------

Alternatively, for small changes and typos you might want to use
GitHub's built in file editor. It allows you to preview your changes
right online (though there can be some differences between GitHub
markdown and Sphinx RST). Just be careful not to create many commits.

Images
------

When you need to add images, try to make them as small as possible
(e.g. as gif). Usually images should go in the same directory as the
.rst file which references them, or in a subdirectory if one already
exists.

Notes
-----
* For the template the css is compiled from less. When changes are needed they can be compiled using
lessc ``lessc main.less`` or watched using watch-lessc ``watch-lessc -i main.less -o main.css``

Guides on using sphinx
----------------------
* To make links to certain sections create a link target like so:

  ```
    .. _hello_world:

    Hello world
    ===========

    This is.. (etc.)
  ```

  The ``_hello_world:`` will make it possible to link to this position
  (page and section heading) from all other pages. See the [Sphinx
  docs](http://sphinx-doc.org/markup/inline.html#role-ref) for more
  information and examples.

* Notes, warnings and alarms

  ```
    # a note (use when something is important)
    .. note::

    # a warning (orange)
    .. warning::

    # danger (red, use sparsely)
    .. danger::

* Code examples

  * Start typed commands with ``$ `` (dollar space) so that they 
    are easily differentiated from program output.
  * Use "sudo" with docker to ensure that your command is runnable
    even if they haven't [used the *docker*
    group](http://docs.docker.io/en/latest/use/basics/#why-sudo).

Manpages
--------

* To make the manpages, run ``make man``. Please note there is a bug
  in Sphinx 1.1.3 which makes this fail.  Upgrade to the latest version
  of Sphinx.
* Then preview the manpage by running ``man _build/man/docker.1``,
  where ``_build/man/docker.1`` is the path to the generated manfile

