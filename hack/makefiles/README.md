This directory allows extending the top-level Makefile by adding custom
makefile include files (`.mk`).

For example:

```bash
echo -e "hello:\n\t@echo hello world\n" > hack/makefiles/hello.mk

make hello
# hello world
```
