# This file describes the standard way to build Docker, using a docker container on Windows 
# Server 2016
#
# Usage:
#
# # Assemble the full dev environment. This is slow the first time. Run this from
# # a directory containing the sources you are validating. For example from 
# # c:\go\src\github.com\docker\docker
#
# docker build -t docker -f Dockerfile.windows .
#
#
# # Build docker in a container. Run the following from a Windows cmd command prommpt,
# # replacing c:\built with the directory you want the binaries to be placed on the
# # host system.
#
# docker run --rm -v "c:\built:c:\target" docker sh -c 'cd /c/go/src/github.com/docker/docker; hack/make.sh binary; ec=$?; if [ $ec -eq 0 ]; then robocopy /c/go/src/github.com/docker/docker/bundles/$(cat VERSION)/binary /c/target/binary; fi; exit $ec'
#
# Important notes:
# ---------------
#
# Multiple commands in a single powershell RUN command are deliberately not done. This is
# because PS doesn't have a concept quite like set -e in bash. It would be possible to use 
# try-catch script blocks, but that would make this file unreadable. The problem is that
# if there are two commands eg "RUN powershell -command fail; succeed", as far as docker
# would be concerned, the return code from the overall RUN is succeed. This doesn't apply to
# RUN which uses cmd as the command interpreter such as "RUN fail; succeed".
#
# 'sleep 5' is a deliberate workaround for a current problem on containers in Windows 
# Server 2016. It ensures that the network is up and available for when the command is
# network related. This bug is being tracked internally at Microsoft and exists in TP4.
# Generally sleep 1 or 2 is probably enough, but making it 5 to make the build file
# as bullet proof as possible. This isn't a big deal as this only runs the first time.
#
# The cygwin posix utilities from GIT aren't usable interactively as at January 2016. This
# is because they require a console window which isn't present in a container in Windows.
# See the example at the top of this file. Do NOT use -it in that docker run!!! 
#
# Don't try to use a volume for passing the source through. The cygwin posix utilities will
# balk at reparse points. Again, see the example at the top of this file on how use a volume
# to get the built binary out of the container.

FROM windowsservercore

# Environment variable notes:
#  - GOLANG_VERSION should be updated to be consistent with the Linux dockerfile.
#  - FROM_DOCKERFILE is used for detection of building within a container.
ENV GOLANG_VERSION=1.5.3 \
    GIT_VERSION=2.7.0 \
    RSRC_COMMIT=ba14da1f827188454a4591717fff29999010887f \
    GOPATH=C:/go;C:/go/src/github.com/docker/docker/vendor \
    FROM_DOCKERFILE=1

# Make sure we're in temp for the downloads
WORKDIR c:/windows/temp

# Download everything else we need to install
# We want a 64-bit make.exe, not 16 or 32-bit. This was hard to find, so documenting the links
#  - http://sourceforge.net/p/mingw-w64/wiki2/Make/ -->
#  - http://sourceforge.net/projects/mingw-w64/files/External%20binary%20packages%20%28Win64%20hosted%29/ -->
#  - http://sourceforge.net/projects/mingw-w64/files/External binary packages %28Win64 hosted%29/make/
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile make.zip http://downloads.sourceforge.net/project/mingw-w64/External%20binary%20packages%20%28Win64%20hosted%29/make/make-3.82.90-20111115.zip
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile gcc.zip http://downloads.sourceforge.net/project/tdm-gcc/TDM-GCC%205%20series/5.1.0-tdm64-1/gcc-5.1.0-tdm64-1-core.zip 
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile runtime.zip http://downloads.sourceforge.net/project/tdm-gcc/MinGW-w64%20runtime/GCC%205%20series/mingw64runtime-v4-git20150618-gcc5-tdm64-1.zip
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile binutils.zip http://downloads.sourceforge.net/project/tdm-gcc/GNU%20binutils/binutils-2.25-tdm64-1.zip
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile 7zsetup.exe http://www.7-zip.org/a/7z1514-x64.exe 
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile lzma.7z http://www.7-zip.org/a/lzma1514.7z
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile gitsetup.exe https://github.com/git-for-windows/git/releases/download/v%GIT_VERSION%.windows.1/Git-%GIT_VERSION%-64-bit.exe
RUN powershell -command sleep 5; Invoke-WebRequest -UserAgent 'DockerCI' -outfile go.msi https://storage.googleapis.com/golang/go%GOLANG_VERSION%.windows-amd64.msi

# Path
RUN setx /M Path "c:\git\cmd;c:\git\bin;c:\git\usr\bin;%Path%;c:\gcc\bin;c:\7zip"

# Install and expand the bits we downloaded. 
# Note: The git, 7z and go.msi installers execute asynchronously. 
RUN powershell -command start-process .\gitsetup.exe -ArgumentList '/VERYSILENT /SUPPRESSMSGBOXES /CLOSEAPPLICATIONS /DIR=c:\git' -Wait
RUN powershell -command start-process .\7zsetup -ArgumentList '/S /D=c:/7zip' -Wait
RUN powershell -command start-process .\go.msi -ArgumentList '/quiet' -Wait
RUN powershell -command Expand-Archive gcc.zip \gcc -Force 
RUN powershell -command Expand-Archive runtime.zip \gcc -Force 
RUN powershell -command Expand-Archive binutils.zip \gcc -Force
RUN powershell -command 7z e lzma.7z bin/lzma.exe
RUN powershell -command 7z x make.zip  make-3.82.90-20111115/bin_amd64/make.exe 
RUN powershell -command mv make-3.82.90-20111115/bin_amd64/make.exe /gcc/bin/ 

# RSRC for manifest and icon	
RUN powershell -command sleep 5 ; git clone https://github.com/akavel/rsrc.git c:\go\src\github.com\akavel\rsrc 
RUN cd c:/go/src/github.com/akavel/rsrc && git checkout -q %RSRC_COMMIT% && go install -v

# Prepare for building
WORKDIR c:/
COPY . /go/src/github.com/docker/docker

