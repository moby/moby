# Hyper-V sockets sample applications

## Building

Building is driven by `make` (or `mingw32-make.exe`). By default both
Linux and Windows binaries are build.

Linux builds are done via Dockerfiles and create static binaries.

Windows builds require MSBuild, Visual Studio and Windows SDK
installed with a minimum version of 14290. The project/solutions
assumes 14291, so you may have to adjust the
`WindowsTargetPlatformVersion` in the project files.


## Running

All sample application assume a service ID of `3049197C-FACB-11E6-BD58-64006A7986D3` which needs to be registered *once* using:
```
$service = New-Item -Path "HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\GuestCommunicationServices" -Name 3049197C-FACB-11E6-BD58-64006A7986D3

$service.SetValue("ElementName", "Hyper-V Socket Echo Service")
```

### Echo applications 

The echo server is started with:
```
hvecho -s
```

The client supports a number of modes. By default, with no arguments supplied it will connected using loopback mode, i.e. it tries to connect to the server on the same partition:
```
hvecho -c
```

The client can be run in a VM and started with:
```
hvecho -c parent
```
which attempts to connect to the server in the parent partition.


Finally, if the server is run in a VM, the client can be invoked in the parent partition with:
```
client -c <vmid>
```
where `<vmid>` is the GUID of the VM the server is running in. The GUID can be retrieved with: `(get-vm <VM Name>).id`.
