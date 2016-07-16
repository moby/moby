# Proposal: Image cache plugins

## Background
When building an image multiple times on the same daemon, Docker will cache image layers so that expensive steps early on in a Dockerfile, such as installing dependencies, do not need to be repeated when only later parts of the Dockerfile are edited. This allows builds to complete more quickly than they otherwise would. This is especially important for CI servers that build Docker images, because caching results in an order of magnitude improvement in build times.

This works ideally when a single machine is building an image repeatedly, but for some CI setups, such as CI servers with multiple build agents, CI servers that spin up elastic build agents, and hosted CI services, there is no guarantee that if a build is run twice, it will be run on the same Docker daemon as before. This means that the second build will take an unnecessarily long time - the cache information is on another host and not available to the build machine.

Previously, before Docker 1.10, a common workaround would be to push the built image to the registry after the first build, and, before performing a new build on another daemon, to pull the image from the registry first. This worked because the registry used to save layer parent information. However, this behaviour was removed in 1.10 because it represented a security risk - anybody could push an image to the docker registry that claimed, for example, to be the result of "FROM ubuntu; RUN apt-get update;" but in fact contained malicious code in the layer. Anybody who pulled this image and built an unrelated image with those steps would have that image poisoned.

## Use cases
The pull-then-build method was only ever a workaround for the lack of a proper method to share the image cache between Docker daemons. However, many people relied on it in order to make their build times acceptable. The issue for this, #20316, has a number of users who did different things with this feature. The use cases discussed include -

* CI setups with multiple build agents that can all build the same containers
* CI setups that had agents which were destroyed at night when not used
* CI setups where agents were created and destroyed on demand
* Companies providing hosted CI solutions, with many agents shared between their customers
* People using disposable VM's in development who rebuilt docker images when the VM came up

## Image save/load based solution
Following the discussion about this issue in #20316, #21385 was merged which kept parent references when exporting/importing images with `docker save` & `docker load`. Instead of doing a registry pull prior to a build, this functionality allows saving a built image and loading it onto the next machine to build that image, roughly emulating the behaviour of the previous push/pull workaround. However, this solution has some problems (some of which were also issues with the push/pull workaround) -

* A fair chunk of scripting is involved in shipping these tarballs between machines
* More data is transferred than with the registry workaround - the registry does not transfer images which it already has, but shipping tarballs around has no such optimisation
* Layers towards the end of a Dockerfile will be exported and imported, despite it being unlikely that they will end up being used in the subsequent build (if, for example, the source code has changed and the layer comes after a `COPY .` step)
* If there are multiple build machines, some system will need to be built to copy the tarballs to each one (or establish a central server for storing them)

Fundamentally, using save/load for sharing image cache is just as much of a workaround as push/pull was (minus the security implications). Given how important CI is to modern development workflows, and especially given the prevalence of Docker with microservices-style architectures (where many different images will need to be built as part of CI pipelines), Docker should have a purpose-built solution to the actual problem of maintaining a coherent cache.

## Why a plugin?
Different users of Docker will have different requirements for how the image cache should be shared between hosts. Here are some examples illustrating how the use cases above actually need slightly different behaviour -

* CI setups with multiple, persistent build agents have no need for a central cache store, because the Docker daemons could ask each other for cached layers on demand
* CI setups with elastic agents that are created and destroyed on demand will need some kind of central store for cache layers - all of the cache layers on a machine will need to be pushed to a central store before the host is destroyed in order to preserve them between agents.
* Hosted CI services need the ability to partition client images from other clients, to avoid the security implications discussed with the old registry pull/push solution
* Different users will want to use different storage backends for cache storage, such as Amazon S3, a dedicated server, or NFS for example.

This diversity of needs implies that rather than shipping a single implementation of an image cache sharing mechanism, Docker should instead implement a plugin interface. Different plugins could be developed independently that implemented different cache sharing strategies.

## Proposed plugin interface
I must admit I'm not all that familiar with the Docker source code, but the relevant code appears to be the function `Daemon.GetCachedImage` in `daemon/image.go`. It probes all of the local images for children of the specified parent that share an equivalent runconfig to the step being built. If the daemon finds no matching image in the local cache, it could make a request to any cache plugins, that look like the following:

```
POST /CachePlugin.GetCachedImage HTTP/1.1

{
    "parentImageId" : "...",
    "runconfig" : { /* Some JSON serialization of the runconfig */ }
}
```

The cache plugin would then look for such an image according to its implementation. If it finds one, it should download it and load it into the local docker daemon using the Docker API (presumably using the endpoint that `docker load` uses). Then, it should respond with the following-
```
HTTP/1.1 200 OK

{
    "Err" : "error_string_if_applicable",
    "imageId" : "id_of_the_image_found_and_imported",
}
```

After receiving this response, the daemon can continue just as if it had found a local cache hit, since the plugin will have loaded the cached image into the daemon.

## Risks & limitations
This implementation should be minimally disruptive to the core Docker code, since it would require changing very few code paths from the normal build-with-cache mechanism - in that sense, it is low risk. However, this approach does add to the plugin API surface, which I imagine is difficult to change - so we should take the time to get this right.

It also pushes the responsibility for actually implementing image cache sharing to plugin authors, and also pushes the responsibility for choosing a caching strategy to the users. This means that out-of-the-box, Docker will still suffer from the same problems with image cache sharing as it does now. Some work would need to be done to improve this - perhaps CI servers themselves could make use of this feature to give *their* users as better out-of-the-box experience.

## Conclusion
Everyone building the same Docker image across multiple hosts needs the cache to be coherent to get acceptable build times, but there are several different use cases for how such a cache should work depending on, amongst other things, the lifetime of the machines being used. This means that image cache sharing should be implemented as a plugin, so that everybody can choose the right implementation for their setup.
