page_title: Installation on Raspberry Pi
page_description: Installation instructions for Docker on Raspberry Pi
page_keywords: raspberry pi, arch linux, hypriot, raspbian, virtualization, docker, documentation, installation

# Raspberry Pi

> **Note:**
> Docker on the Raspberry Pi is experimental and not officially
> supported.

There are several ways of installing Docker on the Raspberry Pi.

- Hypriot
- Raspbian
- Arch Linux

## Hypriot

[Hypriot](http://blog.hypriot.com) is a minimal distribution of
[Raspbian](http://raspbian.org) designed to run Docker. Hypriot's
version of Docker is designed to work with OverlayFS; the kernel
includes the OverlayFS module.

Hypriot ships both a SD Card image containing the entire distribution
as well as a Debian Docker package. The Debian package is not
guaranteed to work with other distributions of Linux. However, it may work
with other Raspbian based distributions if `/etc/default/docker` is
edited to remove `--storage-driver=overlay` from the `DOCKER_OPTS`.

Hypriot provides a [tool](https://github.com/hypriot/flash) to flash
an SD card for Linux and OS/X computers. Otherwise, a guide for
burning the image to the SD Card is located
[here](http://computers.tutsplus.com/articles/how-to-flash-an-sd-card-for-raspberry-pi--mac-53600).

Hypriot uses Adafruit's
[occi](http://github.com/adafruit/Adafruit-Occi) to configure the
hostname and optionally wifi settings. The `flash` tool will edit it
automatically.

If you are not using the `flash` tool, you can either edit the
`/boot/occidentalis.txt` prior to booting the Pi for the first time or
edit it and run `occi`. The file looks like:

```
# hostname for your Hypriot Raspberry Pi:
hostname=hypriot-pi

# basic wireless networking options:
wifi_ssid=SSID
wifi_password=12345
```

Lines which are commented are ignored.

On the first boot, the filesystem is resized to the size of the SD
Card and then the Pi will automatically reboot.

## Raspbian

These directions are based, in part, on
[Docker, Weave, Raspberry Pi and a bit of Networked Cloud Computing! — Medium](https://medium.com/@ALGrendel/docker-weave-a-little-cloud-and-a-raspberry-pi-381f73a4376d).

The version of
[Raspbian](http://raspbian.org) which is available on the Raspberry Pi
[download](https://www.raspberrypi.org/downloads/) page does not
include Docker. However Jessie does.

Install Raspbian to a SD card according to the image installation
[guide](https://www.raspberrypi.org/documentation/installation/installing-images/README.md).

After the first boot, ensure that the install is current:

```
sudo apt-get update; sudo apt-get -y upgrade
```

Modify the references in `/etc/apt/sources.list` from *wheezy* to
*jessie*.

Update and upgrade by:

```
sudo apt-get update; sudo apt-get upgrade; sudo apt-get dist-upgrade;
```

The steps may need to be repeated. Do so until there is nothing left
to be updated.

Once updated, Docker can be installed:

```
sudo apt-get install docker.io
```

## Arch Linux

Docker can be installed on Arch Linux by running `pacman -S docker`.
