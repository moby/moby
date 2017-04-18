# [xhyve.org](http://www.xhyve.org)

![](./xhyve_logo.png)
<!-- https://thenounproject.com/term/squirrel/57718/ -->

About
-----

The *xhyve hypervisor* is a port of [bhyve](http://www.bhyve.org) to OS X. It is built on top of [Hypervisor.framework](https://developer.apple.com/library/mac/documentation/DriversKernelHardware/Reference/Hypervisor/index.html) in OS X 10.10 Yosemite and higher, runs entirely in userspace, and has no other dependencies. It can run FreeBSD and vanilla Linux distributions and may gain support for other guest operating systems in the future.

License: BSD

Introduction: [http://www.pagetable.com/?p=831](http://www.pagetable.com/?p=831)

Requirements
------------

* OS X 10.10.3 Yosemite or later
* a 2010 or later Mac (i.e. a CPU that supports EPT)

Installation
------------

If you have homebrew, then simply:

    $ brew update
    $ brew install --HEAD xhyve

The `--HEAD` in the brew command ensures that you always get the latest changes, even if the homebrew database is not yet updated. If for any reason you don't want that simply do `brew install xhyve` .

if not then:  

Building
--------
    $ git clone https://github.com/mist64/xhyve
    $ cd xhyve
    $ make

The resulting binary will be in build/xhyve

Usage
-----

    $ xhyve -h


What is bhyve?
--------------

bhyve is the FreeBSD hypervisor, roughly analogous to KVM + QEMU on Linux. It has a focus on simplicity and being legacy free.

It exposes the following peripherals to virtual machines:

  - Local x(2)APIC
  - IO-APIC
  - 8259A PIC
  - 8253/8254 PIT
  - HPET
  - PM Timer
  - RTC
  - PCI
    - host bridge
    - passthrough
    - UART
    - AHCI (i.e. HDD and CD)
    - VirtIO block device
    - VirtIO networking
    - VirtIO RNG

Notably absent are sound, USB, HID and any kind of graphics support. With a focus on server virtualization this is not strictly a requirement. bhyve may gain desktop virtualization capabilities in the future but this doesn't seem to be a priority.

Unlike QEMU, bhyve also currently lacks any kind of guest-side firmware (QEMU uses the GPL3 [SeaBIOS](http://www.seabios.org)), but aims to provide a compatible [OVMF EFI](http://www.linux-kvm.org/page/OVMF) in the near future. It does however provide ACPI, SMBIOS and MP Tables.

bhyve architecture
------------------
                                                           Linux
               I/O        VM control       FreeBSD        NetBSD
                                                          OpenBSD
             |     A        |     A           |              |
             V     |        V     |           V              V
         +-------------++-------------++-------------++-------------+
         |             ||             ||             ||             |
         |    bhyve    ||  bhyvectl   ||  bhyveload  || grub2-bhyve |
         |             ||             ||             ||             |
         |             ||             ||             ||             |
         +-------------++-------------++-------------++-------------+
         +----------------------------------------------------------+
         |                        libvmmapi                         |
         +----------------------------------------------------------+
                                       A
                                       |                         user
         ------------------------------┼------------------------------
                                       | ioctl         FreeBSD kernel
                                       V
                         +----------------------------+
                         |        VMX/SVM host        |
                         |       VMX/SVM guest        |
                         |   VMX/SVM nested paging    |
                         |           Timers           |
                         |         Interrupts         |
                         +----------------------------+
                          vmm.ko


**vmm.ko**

The bhyve FreeBSD kernel module. Manages VM and vCPU objects, the guest physical address space and handles guest interaction with PIC, PIT, HPET, PM Timer, x(2)APIC and I/O-APIC. Contains a minimal x86 emulator to decode guest MMIO. Executes the two innermost vCPU runloops (VMX/SVM and interrupts/timers/paging). Has backends for Intel VMX and AMD SVM. Provides an ioctl and mmap API to userspace.

**libvmmapi**

Thin abstraction layer between the vmm.ko ioctl interface and the userspace C API.

**bhyve**

The userspace bhyve component (kind of a very light-weight QEMU) that executes virtual machines. Runs the guest I/O vCPU runloops. Manages ACPI, PCI and all non in-kernel devices. Interacts with vmm.ko through libvmmapi.

**bhyvectl**

Somewhat superfluous utility to introspect and manage the life cycle of virtual machines. Virtual machines and vCPUs can exist as kernel objects independently of a bhyve host process. Typically used to delete VM objects after use. Odd architectural choice.

**bhyveload**

Userspace port of the FreeBSD bootloader. Since bhyve still lacks a firmware this is a cumbersome workaround to bootstrap a guest operating system. It creates a VM object, loads the FreeBSD kernel into guest memory, sets up the initial vCPU state and then exits. Only then a VM can be executed by bhyve.

**grub2-bhyve**

Performs the same function as bhyveload but is a userspace port of [GRUB2](http://github.com/grehan-freebsd/grub2-bhyve). It is used to bootstrap guest operating systems other than FreeBSD, i.e. Linux, OpenBSD and NetBSD.

Support for Windows guests is work in progress and dependent on the EFI port.


xhyve architecture
------------------
        +----------------------------------------------------------+
        | xhyve                                                    |
        |                                                          |
        |                            I/O                           |
        |                                                          |
        |                                                          |
        |                                                          |
        |+--------------------------------------------------------+|
        ||  vmm                   VMX guest                       ||
        ||                          Timers                        ||
        ||                        Interrupts                      ||
        |+--------------------------------------------------------+|
        +----------------------------------------------------------+
        +----------------------------------------------------------+
        |                   Hypervisor.framework                   |
        +----------------------------------------------------------+
                                      A
                                      |                         user
        ------------------------------┼------------------------------
                                      |syscall            xnu kernel
                                      V
        
                                   VMX host
                               VMX nested paging


xhyve shares most of the code with bhyve but is architecturally very different. Hypervisor.framework provides an interface to the VMX VMCS guest state and a safe subset of the VMCS control fields, thus making userspace hypervisors without any additional kernel extensions possible. The VMX host state and all aspects of nested paging are handled by the OS X kernel, you can manage the guest physical address space simply through mapping of regions of your own address space.

*xhyve* is equivalent to the *bhyve* process but gains a subset of a userspace port of the vmm kernel module. SVM, PCI passthrough and the VMX host and EPT aspects are dropped. The vmm component provides a libvmmapi compatible interface to xhyve. Hypervisor.framework seems to enforce a strict 1:1 relationship between a host process/VM and host thread/vCPU, that means VMs and vCPUs can only be interacted with by the processes and threads that created them. Therefore, unlike bhyve, xhyve needs to adhere to a single process model. Multiple virtual machines can be created by launching multiple instances of xhyve. xhyve retains most of the bhyve command line interface.

*bhyvectl*, *bhyveload* and *grub2-bhyve* are incompatible with a single process model and are dropped. As a stop-gap solution until we have a proper firmware xhyve supports the Linux [kexec protocol](http://www.kernel.org/doc/Documentation/x86/boot.txt), a very simple and straightforward way to bootstrap a Linux kernel. It takes a bzImage and optionally initrd image and kernel parameter string as input.

Networking
------
If you want the same IP address across VM reboots, assign a UUID to a particular VM:

    $ xhyve [-U uuid]

**Optional:**

If you need more advanced networking and already have a configured [TAP](http://tuntaposx.sourceforge.net) device you can use it with:

	virtio-tap,tapX

instead of:

    virtio-net

Where *X* is your tap device, i.e. */dev/tapX*.

Issues
------
If you are, or were, running any version of VirtualBox, prior to 4.3.30 or 5.0,
and attempt to run xhyve your system will immediately crash as a kernel panic is
triggered. This is due to a VirtualBox bug (that got fixed in newest VirtualBox
versions) as VirtualBox wasn't playing nice with OSX's Hypervisor.framework used
by xhyve.

To get around this you either have to update to newest VirtualBox 4.3 or 5.0 or,
if you for some reason are unable to update, to reboot
your Mac after using VirtualBox and before attempting to use xhyve.
(see issues [#5](https://github.com/mist64/xhyve/issues/5) and
[#9](https://github.com/mist64/xhyve/issues/9) for the full context)

TODO
----

- vmm:
  - enable APIC access page to speed up APIC emulation (**performance**)
  - enable x2APIC MSRs (even faster) (**performance**)
  - vmm_callout:
      - is a quick'n'dirty implementation of the FreeBSD kernel callout mechanism
      - seems to be racy
      - fix races or perhaps replace with something better
      - use per vCPU timer event thread (**performance**)?
      - use hardware VMX preemption timer instead of `pthread_cond_wait` (**performance**)
  - some 32-bit guests are broken (support PAE paging in VMCS)
  - PCID guest support (**performance**)
- block_if:
  - OS X does not support `preadv`/`pwritev`, we need to serialize reads and writes for the time being until we find a better solution. (**performance**)
  - support block devices other than plain files
- virtio_net:
  - unify TAP and vmnet backends
  - vmnet: make it not require root
  - vmnet: send/receive more than a single packet at a time (**performance**)
- virtio_rnd:
  - is untested
- remove explicit state transitions:
  - since only the owning task/thread can modify the VM/vCPUs a lot of the synchronization might be unnecessary (**performance**)
- performance, performance and performance
- remove vestigial code, cleanup
