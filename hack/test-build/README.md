This directory is meant to test the behavior of 'docker build' in general, and its ADD command in particular.
Ideally it would be automatically called as part of the unit test suite - but that requires slightly more glue,
so for now we're just calling it manually, because it's better than nothing.

To test, simply build this directory with the following command:

	docker build .

The tests are embedded in the Dockerfile: if the build succeeds, the tests have passed.
