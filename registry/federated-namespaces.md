# Proposal: Federating namespace restrictions

This proposes minimizing the constraints the Docker client places on image names, deferring instead to the Registry at push/pull time to push back with a suitable error message around character restrictions / form.


## Preface: Current State

Today, the client is overly aware of the character restrictions and form of repository names.  These restrictions have been adjusted throughout the course of v1, and have changed non-trivially in the v1 => v2 switch.

The restrictions enforced in the client today are very v1-centric, which makes some of the relaxations in v2 unusable in their current form.

### v1

In the world of Docker Registry v1, names have the form:
```
   registry/namespace/image:tag
```

`tag` can often be omitted, and depending on context can mean `latest` or all tags.

`registry` can be omitted, and defaults to DockerHub (`index.docker.io`).

`namespace` can be omitted, defaulting to `library`.

`image` is required.


This image path is then processed as follows:
   1. If there is a single part, continue processing as `index.docker.io/library/single-part`
   1. If there are two parts, and the first part is `localhost` or contains a `:` or `.` character, continue processing as `first-part/library/second-part`
   1. If there are more than three parts, fail.

The result of the above should result in exactly three parts when split on `/`:
   1. `registry`: The hostname of the registry with which to interact.
   1. `namespace`: The namespace (on DockerHub the user or organization) under which the image exists.
   1. `image:tag`: This is further split on `:` into at most 2 pieces, with `tag` defaulting to `latest`.

These components are validated against the regular expressions (see [validateRemoteName](https://sourcegraph.com/github.com/docker/docker@624de8a9cd5652d0d685596d490997f7f2e83536/.tree/registry/config.go#startline=205&endline=205), see [ValidateTagName](https://sourcegraph.com/github.com/docker/docker@624de8a9cd5652d0d685596d490997f7f2e83536/.tree/graph/tags.go#startline=354&endline=354)):
   1. `namespace` must match `^([a-z0-9-_]*)$`
   1. `image` must match `^([a-z0-9-_.]+)$`
   1. `tag` must match `^[\w][\w.-]{0,127}$`

### v2

In the world of Docker Registry v2, names have a similar, but slightly more flexible form best described [here](https://sourcegraph.com/github.com/docker/distribution@5556cd1ba11aea6803f3c51a16bb96813ba72800/.tree/registry/api/v2/names.go#selected=56):
```
// Effectively, the name should comply with the following grammar:
//
// 	alpha-numeric := /[a-z0-9]+/
//	separator := /[._-]/
//	component := alpha-numeric [separator alpha-numeric]*
//	namespace := component ['/' component]*
```
Where `namespace` is used in the context: `registry/namespace:tag`, where as before `registry` and `tag` are optional.

Processing this image path simplifies to:
   1. If there are multiple parts, then `registry` is the first part if the first part is `localhost`, or contains a `:` or `.`.
   1. Otherwise, `registry` is `index.docker.io`.

The final component of the remainder is split on `:` into two parts, and the second half becomes the `tag`, which I believe to be validated in the same fashion as in v1.

What remains is validated against `namespace` in the above BNF grammar, and limited to `255` characters.


### v1 vs. v2

The acceptable names in v1 vs. v2 form a Venn diagram.  This covers several examples.

#### v1 only
```
registry/_foo/bar:baz
registry/foo/a-name-longer-than-255-characters:baz
registry/a-255-character-name/bar:baz
```

**NOTE**: The [Google Container Registry](https://gcr.io) makes extensive use of `gcr.io/_b_...` style prefixes to alter behaviors.

#### v2 only
```
registry/f.o.o/bar:baz
registry/f/o/o/bar:baz
```

#### both
```
registry/foo/bar:baz
```

## Part 1: Relax client-side validation

In order to continue to support both v1 and v2 within the same client, while enabling customers to start leveraging the more flexible aspects of naming afforded by the v2 registry, I propose the following grammar for image paths:
```
//  tag := /[^:]+/
//  hash := 'sha256'
//        | 'sha284'
//        | 'sha512'
//        | 'tarsum'     // TODO(mattmoor): Can we retire this?
//  digest := /[a-fA-F0-9]+/
//  image := /.+/
//  registry := 'localhost'
//            | /[^/]*[:.][^/]*/
//  path := [registry '/']? image '@' hash ':' digest
//        | [registry '/']? image [':' tag]?
```

**NOTE**: We still require a rigid form for `...@hash:digest` references because the client must validate these checksums.

## Part 2: Surface diagnostics from Docker Registry

In order to move validation server-side, we need to ensure the Registry can adequately surface diagnostic messages regarding unsupported names.

In the event that a v1 Registry is asked for a named entity outside of its own naming restrictions, it will reply with:
```
400 Bad Request

My diagnostic message about your bad input name: "ba#d/inPut/nam3"
```

The Docker CLI will surface this as:
```
$ docker push localhost:8080/ba#d/inPut/nam3
The push refers to a repository [localhost:8080/ba#d/inPut/nam3] (len: 1)
Sending image list
FATA[0000] Error: Status 400 trying to push repository ba#d/inPut/nam3: "My diagnostic message about your bad input name: \"ba#d/inPut/nam3\""
```

For a v2 Registry, the reply ALREADY utilizes `NAME_INVALID`, as part of a standard JSON response:
```
400 Bad Request
Content-Type: application/json; charset=utf-8

{
    "errors:" [
        {
            "code": "NAME_INVALID",
            "message": "<error message>",
            "detail": ...
        },
        ...
    ]
}
```

The Docker CLI surfaces this today as:
```
$ docker push mattmoor/_blazinga
The push refers to a repository [mattmoor/_blazinga] (len: 1)
07f8e8c5e660: Image push failed 
FATA[0000] Error pushing to registry: mux: variable "mattmoor/_blazinga" doesn't match, expected "^(?:[a-z0-9]+(?:[._-][a-z0-9]+)*/){0,4}[a-z0-9]+(?:[._-][a-z0-9]+)*$"
```

## Part 3: Separate Registry implementation from specification

The Registry API specifications should dictate how invalid names are communicated to the client, but the enforced restrictions on naming should be an implementation detail.

