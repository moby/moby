#VT100

[![GoDoc](https://godoc.org/github.com/tonistiigi/vt100?status.svg)](https://godoc.org/github.com/tonistiigi/vt100)

This project was based on [jaguilar/vt100](https://github.com/jaguilar/vt100)

This is a vt100 screen reader. It seems to do a pretty
decent job of parsing the nethack input stream, which
is all I want it for anyway.

Here is a screenshot of the HTML-formatted screen data:

![](_readme/screencap.png)

The features we currently support:

* Cursor movement
* Erasing
* Many of the text properties -- underline, inverse, blink, etc.
* Sixteen colors
* Cursor saving and unsaving
* UTF-8
* Scrolling

Not currently supported (and no plans to support):

* Prompts
* Other cooked mode features

The API is not stable! This is a v0 package.
