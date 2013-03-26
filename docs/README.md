docker website
==============

This is the docker website repository

installation
------------
* Checkout this repo to your local dir
* Install sphinx: ``pip install sphinx``
* Push this to dotcloud


Usage
-----
* run make docs
* your static website can now be found in the _build dir
* change the .rst files with your favorite editor to your liking
* run *make clean* to clean up
* run *make docs* to build the new version


Notes
-----
* The index.html file is copied from the source dir to the output dir without modification. So changes to
 the index.html page should be made directly in html
* a simple way to run locally. cd into _build and then run 'python -m SimpleHTTPServer 8000'
* For the Template the css is compiled from less. When changes are needed they can be compiled using lessc ``lessc main.less`` or watched using watch-lessc ``watch-lessc -i main.less -o main.css``