---
published: false
---

<!-- This file is maintained within the docker/docker Github
     repository at https://github.com/docker/docker/. Make all
     pull requests against that repo. If you see this file in
     another repository, consider it read-only there, as it will
     periodically be overwritten by the definitive file. Pull
     requests which include edits to this file in other repositories
     will be rejected.
-->

This directory holds the authoritative specifications of APIs defined and implemented by Docker. Currently this includes:

 * The remote API by which a docker node can be queried over HTTP
 * The registry API by which a docker node can download and upload
   images for storage and sharing
 * The index search API by which a docker node can search the public
   index for images to download
 * The docker.io OAuth and accounts API which 3rd party services can
   use to access account information
