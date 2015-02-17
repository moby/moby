page_title: Labels - custom meta-data in Docker
page_description: Learn how to work with custom meta-data in Docker, using labels.
page_keywords: Usage, user guide, labels, meta-data, docker, documentation, examples, annotating

## Labels - custom meta-data in Docker

Docker enables you to add meta-data to your images, containers, and daemons via
labels. Meta-data can serve a wide range of uses ranging from adding notes or 
licensing information to an image to identifying a host.

Labels in Docker are implemented as `<key>` / `<value>` pairs and values are
stored as *strings*.

>**note:** Support for daemon-labels was added in docker 1.4.1, labels on
>containers and images in docker 1.6.0

### Naming your labels - namespaces

Docker puts no hard restrictions on the names you pick for your labels, however,
it's easy for labels to "conflict".

For example, you're building a tool that categorizes your images in 
architectures, doing so by using an "architecture" label;

    LABEL architecture="amd64"

    LABEL architecture="ARMv7"

But a user decided to label images by Architectural style

    LABEL architecture="Art Nouveau"

To prevent such conflicts, Docker uses the convention to namespace label-names,
using a reverse domain notation;


- All (third-party) tools should namespace (prefix) their labels with the 
  reverse DNS notation of a domain controlled by the author of the tool. For 
  example, "com.example.some-label".
- Namespaced labels should only consist of lower-cased alphanumeric characters,
  dots and dashes (in short, `[a-z0-9-.]`), should start *and* end with an alpha
  numeric character, and may not contain consecutive dots or dashes.
- The `com.docker.*`, `io.docker.*` and `com.dockerproject.*` namespaces are
  reserved for Docker's internal use.
- Labels *without* namespace (dots) are reserved for CLI use. This allows end-
  users to add meta-data to their containers and images, without having to type 
  cumbersome namespaces on the command-line.


> **Note:** Even though Docker does not *enforce* you to use namespaces,
> preventing to do so will likely result in conflicting usage of labels. 
> If you're building a tool that uses labels, you *should* use namespaces
> for your labels.


### Storing structured data in labels

Labels can store any type of data, as long as its stored as a string. 


    {
        "Description": "A containerized foobar",
        "Usage": "docker run --rm example/foobar [args]",
        "License": "GPL",
        "Version": "0.0.1-beta",
        "aBoolean": true,
        "aNumber" : 0.01234,
        "aNestedArray": ["a", "b", "c"]
    }

Which can be stored in a label by serializing it to a string first;

    LABEL com.example.image-specs="{\"Description\":\"A containerized foobar\",\"Usage\":\"docker run --rm example\\/foobar [args]\",\"License\":\"GPL\",\"Version\":\"0.0.1-beta\",\"aBoolean\":true,\"aNumber\":0.01234,\"aNestedArray\":[\"a\",\"b\",\"c\"]}"


> **Note:** Although the example above shows it's *possible* to store structured 
> data in labels, Docker does not treat this data any other way than a 'regular'
> string. This means that Docker doesn't offer ways to query (filter) based on 
> nested properties. If your tool needs to filter on nested properties, the 
> tool itself should implement this.


### Adding labels to images; the `LABEL` instruction

Adding labels to your 


    LABEL [<namespace>.]<name>[=<value>] ...

The `LABEL` instruction adds a label to your image, optionally setting its value.
Quotes surrounding name and value are optional, but required if they contain
white space characters. Alternatively, backslashes can be used.

Label values only support strings


    LABEL vendor=ACME\ Incorporated
    LABEL com.example.version.is-beta
    LABEL com.example.version="0.0.1-beta"
    LABEL com.example.release-date="2015-02-12"

The `LABEL` instruction supports setting multiple labels in a single instruction
using this notation;

    LABEL com.example.version="0.0.1-beta" com.example.release-date="2015-02-12"

Wrapping is allowed by using a backslash (`\`) as continuation marker;

    LABEL vendor=ACME\ Incorporated \
          com.example.is-beta \
          com.example.version="0.0.1-beta" \
          com.example.release-date="2015-02-12"


You can view the labels via `docker inspect`

    $ docker inspect 4fa6e0f0c678

    ...
    "Labels": {
        "vendor": "ACME Incorporated",
        "com.example.is-beta": "",
        "com.example.version": "0.0.1-beta",
        "com.example.release-date": "2015-02-12"
    }
    ...

    $ docker inspect -f "{{json .Labels }}" 4fa6e0f0c678

    {"Vendor":"ACME Incorporated","com.example.is-beta":"","com.example.version":"0.0.1-beta","com.example.release-date":"2015-02-12"}

> **note:** We recommend combining labels in a single `LABEl` instruction in
> stead of using a `LABEL` instruction for each label. Each instruction in a
> Dockerfile produces a new layer, which can result in an inefficient image if
> many labels are used.

### Querying labels

Besides storing meta-data, labels can be used to filter your images and 
containers.

List all running containers that have a `com.example.is-beta` label;

    # List all running containers that have a `com.example.is-beta` label
    $ docker ps --filter "label=com.example.is-beta"


List all running containers with a "color" label set to "blue";

    $ docker ps --filter "label=color=blue"

List all images with "vendor" "ACME"

    $ docker images --filter "label=vendor=ACME"


### Daemon labels



    docker -d \
      --dns 8.8.8.8 \
      --dns 8.8.4.4 \
      -H unix:///var/run/docker.sock \
      --label com.example.environment="production" \
      --label com.example.storage="ssd"

And can be seen as part of the `docker info` output for the daemon;

    docker -D info
    Containers: 12
    Images: 672
    Storage Driver: aufs
     Root Dir: /var/lib/docker/aufs
     Backing Filesystem: extfs
     Dirs: 697
    Execution Driver: native-0.2
    Kernel Version: 3.13.0-32-generic
    Operating System: Ubuntu 14.04.1 LTS
    CPUs: 1
    Total Memory: 994.1 MiB
    Name: docker.example.com
    ID: RC3P:JTCT:32YS:XYSB:YUBG:VFED:AAJZ:W3YW:76XO:D7NN:TEVU:UCRW
    Debug mode (server): false
    Debug mode (client): true
    Fds: 11
    Goroutines: 14
    EventsListeners: 0
    Init Path: /usr/bin/docker
    Docker Root Dir: /var/lib/docker
    WARNING: No swap limit support
    Labels:
     com.example.environment=production
     com.example.storage=ssd
