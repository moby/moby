#### goals
* small and simple - line count is not everything but less code is better
* clean lines between what we do in the pkg 
* provide primitives for working with namespaces not cater to every option
* extend via configuration not by features - host networking, no networking, veth network can be accomplished via adjusting the container.json, nothing to do with code

#### tasks
* proper tty for a new process in an existing container
* use exec or raw syscalls for new process in existing container
* setup proper user in namespace if specified
* implement hook or clean interface for cgroups
* example configs for different setups (host networking, boot init)
* improve pkg documentation with comments
* testing - this is hard in a low level pkg but we could do some, maybe
* pivot root
* selinux
* apparmor
