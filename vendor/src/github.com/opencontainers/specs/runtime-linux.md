# Linux Runtime

## File descriptors
By default, only the `stdin`, `stdout` and `stderr` file descriptors are kept open for the application by the runtime.

The runtime may pass additional file descriptors to the application to support features such as [socket activation](http://0pointer.de/blog/projects/socket-activated-containers.html).

Some of the file descriptors may be redirected to `/dev/null` even though they are open.
