# OCI Specs Roadmap

This document serves to provide a long term roadmap on our quest to a 1.0 version of the OCI container specification.
Its goal is to help both maintainers and contributors find meaningful tasks to focus on and create a low noise environment.
The items in the 1.0 roadmap can be broken down into smaller milestones that are easy to accomplish.
The topics below are broad and small working groups will be needed for each to define scope and requirements or if the feature is required at all for the OCI level.
Topics listed in the roadmap do not mean that they will be implemented or added but are areas that need discussion to see if they fit in to the goals of the OCI.

## 1.0

### Digest and Hashing

A bundle is designed to be moved between hosts. 
Although OCI doesn't define a transport method we should have a cryptographic digest of the on-disk bundle that can be used to verify that a bundle is not corrupted and in an expected configuration.

*Owner:* philips

### Review the need for runtime.json

There are some discussions about having `runtime.json` being optional for containers and specifying defaults.
Runtimes would use this standard set of defaults for containers and `runtime.json` would provide overrides for fine tuning of these extra host or platform specific settings.

*Owner:*  

### Define Container Lifecycle

Containers have a lifecycle and being able to identify and document the lifecycle of a container is very helpful for implementations of the spec.  
The lifecycle events of a container also help identify areas to implement hooks that are portable across various implementations and platforms.

*Owner:* mrunalp

### Define Standard Container Actions

Define what type of actions a runtime can perform on a container without imposing hardships on authors of platforms that do not support advanced options.

*Owner:*  

### Clarify rootfs requirement in base spec

Is the rootfs needed or should it just be expected in the bundle without having a field in the spec?

*Owner:*  

### Container Definition

Define what a software container is and its attributes in a cross platform way.

*Owner:*  

### Live Container Updates

Should we allow dynamic container updates to runtime options? 

*Owner:* vishh

### Protobuf Config 

We currently have only one language binding for the spec and that is Go.
If we change the specs format in the respository to be something like protobuf then the generation for multiple language bindings become effortless.

*Owner:* vbatts

### Validation Tooling

Provide validation tooling for compliance with OCI spec and runtime environment. 

*Owner:* mrunalp

### Version Schema

Decide on a robust versioning schema for the spec as it evolves.

*Owner:*  

### Printable/Compiled Spec

Reguardless of how the spec is written, ensure that it is easy to read and follow for first time users.

*Owner:* vbatts 

### Base Config Compatibility

Ensure that the base configuration format is viable for various platforms.

Systems: 

* Solaris
* Windows 
* Linux

*Owner:* 

### Full Lifecycle Hooks
Ensure that we have lifecycle hooks in the correct places with full coverage over the container lifecycle.

*Owner:*  
