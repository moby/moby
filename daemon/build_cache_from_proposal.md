# Proposal: Using registry images in build cache

## Background
When building an image multiple times on the same daemon, Docker will cache image layers so that expensive steps early on in a Dockerfile, such as installing dependencies, do not need to be repeated when only later parts of the Dockerfile are edited. This allows builds to complete more quickly than they otherwise would. This is especially important for CI servers that build Docker images, because caching results in an order of magnitude improvement in build times.

This works ideally when a single machine is building an image repeatedly, but for some CI setups, such as CI servers with multiple build agents, CI servers that spin up elastic build agents, and hosted CI services, there is no guarantee that if a build is run twice, it will be run on the same Docker daemon as before. This means that the second build will take an unnecessarily long time - the cache information is on another host and not available to the build machine.

Previously, before Docker 1.10, a common workaround would be to push the built image to the registry after the first build, and, before performing a new build on another daemon, to pull the image from the registry first. This worked because the registry used to save layer parent information. However, this behaviour was removed in 1.10 because it represented a security risk - anybody could push an image to the docker registry that claimed, for example, to be the result of `FROM ubuntu; RUN apt-get update;` but in fact contained malicious code in the layer. Anybody who pulled this image and built an unrelated image with those steps would have that image poisoned.

## Use cases
The pull-then-build method was only ever a workaround for the lack of a proper method to share the image cache between Docker daemons. However, many people relied on it in order to make their build times acceptable. The issue for this, #20316, has a number of users who did different things with this feature. The use cases discussed include -

* CI setups with multiple build agents that can all build the same containers
* CI setups that had agents which were destroyed at night when not used
* CI setups where agents were created and destroyed on demand
* Companies providing hosted CI solutions, with many agents shared between their customers
* People using disposable VM's in development who rebuilt docker images when the VM came up

## Image save/load based solution
Following the discussion about this issue in #20316, #21385 was merged which kept parent references when exporting/importing images with `docker save` & `docker load`. Instead of doing a registry pull prior to a build, this functionality allows saving a built image and loading it onto the next machine to build that image, roughly emulating the behaviour of the previous push/pull workaround.

An external tool has been developed, `tonistiigi/buildcache`, to help automate build systems that need to save cache information. To use it, an image is first pushed to the registry, saving all of the constituent layers (but not the parent image chain). Then, the buildcache tool saves a tarball containing the image *configuration* of the parents, but not the *layers* (which are in the registry). The process can be inverted to pull image layers from the registry with the previously built image, and then load up the image chain configuration from the tarball (which references the layers that already exist). This process emulates the previous behaviour of pulling with image chains.

However, there are a couple of downsides to this implementation, relative to the previous push-then-pull workflow:

* An external tool is needed, which users will have to discover for themselves (they need to realise they have this problem!)
* Tarballs need to be shipped around using some process other than the registry - two systems are being used for storage where ony one might suffice, increasing complexity of CI systems.

There are also some ways in which this implementation is less than optimal, that were also issues with the push-then-pull workflow:

* More data is transferred than is strictly needed. The building machine only needs layers up to the point at which its Dockerfile commands diverge from what's available in the registry, but it will in fact pull all of the layers for a given image

We should develop a way of managing the build cache that covers the most common CI use cases and stil preserves the security of explicitly opting in to loading parent image chains.

## Why not a plugin?
Different users of Docker will have different requirements for how the image cache should be shared between hosts. Here are some examples illustrating how the use cases above actually need slightly different behaviour -

* CI setups with multiple, persistent build agents have no need for a central cache store, because the Docker daemons could ask each other for cached layers on demand
* CI setups with elastic agents that are created and destroyed on demand will need some kind of central store for cache layers - all of the cache layers on a machine will need to be pushed to a central store before the host is destroyed in order to preserve them between agents.
* Hosted CI services need the ability to partition client images from other clients, to avoid the security implications discussed with the old registry pull/push solution
* Different users will want to use different storage backends for cache storage, such as Amazon S3, a dedicated server, or NFS for example.

There is a temptation to believe that rather than shipping a single implementation of an image cache sharing mechanism, Docker should instead implement a plugin interface. Different plugins could be developed independently that implemented different cache sharing strategies.

However, all such plugins will have to implement substantially the same work - pushing images around with intelligent deduplication of identical layers. The registry already does this, and all that is really needed is the appropriate glue between the daemon and the registry to make this work. Furthermore, as previously mentioned, we shouldn't rely on more external tools to solve problems that most users of Docker will have.

## Proposed implementation

### Phase 1: `--cache-from` flag

We propose giving a new flag to the `docker build` command, `--cache-from`. The flag can be specified multiple times, referring to multiple `image:tag` pairs (`latest` being inferred as usual if no tag is specified). This flag indicates to the daemon that the person building this image trusts that the images specified are not malicious and are trusted to use as cache sources. Requiring explicit opting-in to trusting images in this way protects against cache poisoning, as described above.

The user will need to first pull any images referred to in `--cache-from` flags from the registry. Note that although the registry does not give a parent image configuration chain, it *does* provide a history array for the image being pulled (as part of the `v1Compatability` object). That history array winds up populating the `Image.History` property of the image, providing Dockerfile commands (in `CreatedBy`) and linking them to layer IDs (indirectly, via the `EmptyLayer` field).

When building an image with the `--cache-from` flag, the builder will look for cached images in the normal way (by finding local images with matching runconfigs and parents). If this lookup is unsuccessful, a new type of lookup will be tried as described in the pseudocode below. Please excuse the Ruby, but it's probably the easiest way for me to express my point :)

```ruby
cache_from_images = cache_from_image_ids.map { |id| get_image_from_id(id) }
cache_from_images.each do |cache_from_image|
    # last_built_image is the image from the previous step of the Dockerfile,
    # or nil if we are running the first command in the Dockerfile
    # current_command is the string we are running for this step of the Dockerfile
    # (i.e. something like "/bin/sh -c #(nop) ADD file:abcdefg"
    last_built_image_history = last_built_image ? last_built_image.history : []
 
    # The zip operator should pad arrays with nils so that they are the same size
    cache_from_image.history.zip(last_built_image_history) do |cache_history_entry, last_built_history_entry|
        if cache_history_entry && last_built_history_entry
            # We are still looping through history entries; make sure they do not diverge
            # Assume == compares the layer ID and the CreatedBy string
            break unless cache_history_entry == last_built_history_entry
        elsif cache_history_entry
            # We've exhausted everything from last_built_image_history; if the next step of
            # cache_history_entry matches the current command, we can use its layer ID as a source for a
            # new image
            if cache_history_entry.CreatedBy == current_command
                # Actually breaks out of this entire piece of pseudocode
                return create_image(
                    layer_id: cache_history_entry.LayerId,
                    created_by: current_command, 
                    parent: last_built_image.id
                )
            else
                # The command is different
                break
            end
        else
        end
    end
end
# If we got here, we didn't find a cached image
return nil
```

This algorithm would need a bit of work before being dropped in; for one, it is needlessly `O(n^2)` because it rechecks all of the image history at every step of building; the history could be scrolled at the same time as the Dockerfile commands are being read. However, the general idea is illustrated - compare the history arrays of what we've built so far with what is in the `--cache-from` image, and if possible, create the next image in the chain by re-using the existing layer instead of actually running the Dockerfile command again.

At first glance, it seems we might be able to avoid compareing the entire image history, and instead find history entries with matching layer ID's and compare from there. However, this ignores other changes to Dockerfiles that do not change the image filesystem but nonetheless must invalidate the build cache - `ENV` declarations come to mind as an example of such.

### Phase 2: Automatic pulling

The logical next step is to automate the pulling of `--cache-from` images - if an image with the specified name & tag does not exist when looking for cached images, then the builder can pull it from the registry as per normal and continue with the above algorithm.

### Phase 3: Layer pulling optimisation

The scheme proposed here so far still downloads more data than is strictly needed. Because we are pulling the entire image before performing the cache-hit calculation, a cache-miss still causes all of the image to be downloaded. This can in theory be improved upon. At no point in the cache-hit algorithm do we ever need the actual layer data; only the metadata provided by the registry, which includes the history and the layer IDs.

We could, therefore, not pull images from the registry if they are missing. Instead, we could request only the metadata for the specified images from the registry, and run the algorithm on that. If a cache-hit is found, the required layers could be subsequently downloaded from the registry.

I have not given a great deal of thought as to how this part can be implemented, but it seems in principle possible. This phase changes no actual behaviour compared to phase 2 (aside from the fact that the `--cache-from` image becomes available in the image store, so this is a pure optimisation that could be implemented after an initial verison of this feature ships.

## Does this solution address the use cases?

Let's copy-paste the use cases from above, and discuss how the `--cache-from` solution does or does not address them.

* CI setups with multiple build agents that can all build the same containers.
  -> As long as each build agent is pushing images that it builds, then other machines will be able to pull them and use them as part of the cache. Note that, however, users will probably want to push *all* built images, regardless of whether or not they are viable (pass tests, etc): perhaps with a different tag prefix?
* CI setups that had agents which were destroyed at night when not used
  -> As long as all images are pushed to a registry before the machine is destroyed, everything is fine.
* CI setups where agents were created and destroyed on demand
  -> As above - images just need to be pushed before the machine is destroyed
* Companies providing hosted CI solutions, with many agents shared between their customers
  -> Customer images could be isolated into different registry repositories.
* People using disposable VM's in development who rebuilt docker images when the VM came up
  -> As long as they have a registry available, this should be fine. They would have needed to be pushing images to a registry for the caching to work previously anyway.
  
It looks like the `--cache-from` solution should adequately cover all the use cases for build caching across machines I can think of.

## Risks & limitations
This implementation should be minimally disruptive to the core Docker code, since it would require changing very few code paths from the normal build-with-cache mechanism - in that sense, it is low risk. Almost all of the substantial changes I believe would be confined to the function `GetCachedImage` in `daemon/image.go` (at least for the first two phases). I am unsure of how the required information for phase 3 would be pulled and cached from the registry, so this needs to be investigated further if we think it is critical for the feature to be released.

It is also worth noting that adding a new flag to the builder CLI means this needs to be passed through to the daemon, necessitating a Docker API version bump.

## Conclusion
Everyone building the same Docker image across multiple hosts needs the cache to be coherent to get acceptable build times, but there are several different use cases for how such a cache should work depending on, amongst other things, the lifetime of the machines being used. However, using the registry for caching appears to cover the most common use cases, and adding a flag to explicitly opt-in to trusting image history addresses the security concern that caused this feature to be removed in the first instance.
