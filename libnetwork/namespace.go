package libnetwork

import "syscall"

// The networkNamespace type is the default implementation of the Namespace
// interface. It simply creates a new network namespace, and moves an interface
// into it when called on method AddInterface.
type networkNamespace struct {
	path       string
	interfaces []*Interface
}

func createNetworkNamespace(path string) (Namespace, error) {
	if err := reexec(cmdReexecCreateNamespace, path); err != nil {
		return nil, err
	}
	return &networkNamespace{path: path}, nil
}

func (n *networkNamespace) AddInterface(i *Interface) error {
	// TODO Open pipe, pass fd to child and write serialized Interface on it.
	if err := reexec(cmdReexecMoveInterface, i.SrcName, i.DstName); err != nil {
		return err
	}
	n.interfaces = append(n.interfaces, i)
	return nil
}

func (n *networkNamespace) Interfaces() []*Interface {
	return n.interfaces
}

func (n *networkNamespace) Path() string {
	return n.path
}

func (n *networkNamespace) Destroy() error {
	// Assuming no running process is executing in this network namespace,
	// unmounting is sufficient to destroy it.
	return syscall.Unmount(n.path, syscall.MNT_DETACH)
}
