## Generate `event_messages.bin`

```console
$ docker run --rm -it -v "$(pwd):/winresources" debian:bullseye bash
root@9ad2260f6ebc:/# apt-get update -y && apt-get install -y binutils-mingw-w64-x86-64
root@9ad2260f6ebc:/# x86_64-w64-mingw32-windmc -v /winresources/event_messages.mc
root@9ad2260f6ebc:/# mv MSG00001.bin /winresources/event_messages.bin
```
