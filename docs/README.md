Docker documentation and website
================================

Documentation
-------------
This is your definite place to contribute to the docker documentation. The documentation is generated from the
.rst files under sources.

The folder also contains the other files to create the http://docker.io website, but you can generally ignore
most of those.


Installation
------------

* Work in your own fork of the code, we accept pull requests.
* Install sphinx: ``pip install sphinx``
* If pip is not available you can probably install it using your favorite package manager as **python-pip**

Usage
-----
* change the .rst files with your favorite editor to your liking
* run *make docs* to clean up old files and generate new ones
* your static website can now be found in the _build dir
* to preview what you have generated, cd into _build/html and then run 'python -m SimpleHTTPServer 8000'

Working using github's file editor
----------------------------------
Alternatively, for small changes and typo's you might want to use github's built in file editor. It allows
you to preview your changes right online. Just be carefull not to create many commits.

Images
------
When you need to add images, try to make them as small as possible (e.g. as gif).


Notes
-----
* The index.html and gettingstarted.html files are copied from the source dir to the output dir without modification.
So changes to those pages should be made directly in html
* For the template the css is compiled from less. When changes are needed they can be compiled using
lessc ``lessc main.less`` or watched using watch-lessc ``watch-lessc -i main.less -o main.css``


Guides on using sphinx
----------------------
* To make links to certain pages create a link target like so:

  ```
    .. _hello_world:

    Hello world
    ===========

    This is.. (etc.)
  ```

  The ``_hello_world:`` will make it possible to link to this position (page and marker) from all other pages.

* Notes, warnings and alarms

  ```
    # a note (use when something is important)
    .. note::

    # a warning (orange)
    .. warning::

    # danger (red, use sparsely)
    .. danger::

* Code examples

  Start without $, so it's easy to copy and paste.