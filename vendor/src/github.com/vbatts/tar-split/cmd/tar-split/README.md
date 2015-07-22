## tar-split utility


## Usage

### Disassembly

```bash
$ sha256sum archive.tar 
d734a748db93ec873392470510b8a1c88929abd8fae2540dc43d5b26f7537868  archive.tar
$ mkdir ./x
$ tar-split d --output tar-data.json.gz ./archive.tar | tar -C ./x -x
time="2015-07-20T15:45:04-04:00" level=info msg="created tar-data.json.gz from ./archive.tar (read 204800 bytes)"
```

### Assembly

```bash
$ tar-split a --output new.tar --input ./tar-data.json.gz  --path ./x/
INFO[0000] created new.tar from ./x/ and ./tar-data.json.gz (wrote 204800 bytes)
$ sha256sum new.tar 
d734a748db93ec873392470510b8a1c88929abd8fae2540dc43d5b26f7537868  new.tar
```


