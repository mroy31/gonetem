
# Docker images

This folder contains source for docker images used for pynetem:

- mroy31/gonetem-host
- mroy31/gonetem-server
- mroy31/gonetem-frr
- mroy31/gonetem-ovs

## Build

To build an image, you just need to go in the right folder and use the following commands:

```bash
# exemple for host
$ cd host
$ docker build -t mroy31/gonetem-host:<version> .
```

## Cross-build with buildx

If you need to build an image for several platform, you can use the new [buildx extension](https://github.com/docker/buildx).

First, install supported compilation platform with

```bash
$ docker run --privileged --rm tonistiigi/binfmt --install all
```

Then create an new builder with

```bash
$ docker buildx create --use --name mybuild
```

Finally to build an image

```bash
# exemple for host
$ cd host
$ docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t mroy31/gonetem-host:<version> .
```

## VyOS image

The process to build a VyOS image for gonetem is different since it requires a docker image build with the ISO image.
So, the steps to build this image are:

1. Download an ISO image of VyOS (for example the LTS version 1.4)
2. Create a docker image from this ISO image

```bash
$ mkdir vyos-docker-build && cd vyos-docker-build
$ mkdir rootfs
$ sudo mount -o loop vyos-1.4.0-generic-amd64.iso rootfs
$ sudo apt-get install -y squashfs-tools
$ mkdir unsquashfs
$ sudo unsquashfs -f -d unsquashfs/ rootfs/live/filesystem.squashfs
$ sudo tar -C unsquashfs -c . | docker import - vyos:1.4
$ sudo umount rootfs
$ cd ..
$ sudo rm -rf vyos-docker-build
```

3. Build the gonetem vyos image

```bash
$ cd vyos
$ docker build -t gonetem-vyos:1.4 .
# If needed, update the base vyos image name in the Dockerfile
```


