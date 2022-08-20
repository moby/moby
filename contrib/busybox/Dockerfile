# Source: https://frippery.org/busybox/
# This Dockerfile builds a (32-bit) busybox images which is suitable for
# running many of the integration-cli tests for Docker against a Windows
# daemon. It will not run on nanoserver as that is 64-bit only.
#
# Based on https://github.com/jhowardmsft/busybox
# John Howard (IRC jhowardmsft, Email john.howard@microsoft.com)
#
# To build: docker build -t busybox .
# To publish: Needs someone with publishing rights
ARG WINDOWS_BASE_IMAGE=mcr.microsoft.com/windows/servercore
ARG WINDOWS_BASE_IMAGE_TAG=ltsc2019
ARG BUSYBOX_VERSION=FRP-4716-g31467ddfc

# Checksum taken from https://frippery.org/files/busybox/SHA256SUM
ARG BUSYBOX_SHA256SUM=b4fd02bb938f97e59a0242da6a7763104d9cd344f96427af178940271b22ed5c

FROM ${WINDOWS_BASE_IMAGE}:${WINDOWS_BASE_IMAGE_TAG}
RUN mkdir C:\tmp && mkdir C:\bin
ARG BUSYBOX_VERSION
ARG BUSYBOX_SHA256SUM
ADD https://frippery.org/files/busybox/busybox-w32-${BUSYBOX_VERSION}.exe /bin/busybox.exe
RUN powershell \
    if ((Get-FileHash -Path /bin/busybox.exe -Algorithm SHA256).Hash -ne $Env:BUSYBOX_SHA256SUM) { \
        Throw \"Checksum validation failed\" \
    }

RUN setx /M PATH "C:\bin;%PATH%"
RUN powershell busybox.exe --list ^|%{$nul = cmd /c mklink C:\bin\$_.exe busybox.exe}
CMD ["sh"]
