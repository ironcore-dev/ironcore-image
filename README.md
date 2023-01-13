# onmetal-image

[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://makeapullrequest.com)
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue&style=flat-square)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/onmetal/onmetal-image.svg)](https://pkg.go.dev/github.com/onmetal/onmetal-image)

## Overview

onmetal-image contains a library to simplify interaction with an OCI-comptabile
registry and it holds the specification as well as a simple command-line client
for the onmetal image.

Onmetal images are used for machine / volume pool implementors (see [onmetal-api](https://github.com/onmetal/onmetal-api))
to prepare and bootstrap machines and machine disks.

They are custom OCI images, meaning they can managed with any OCI compatible registry.
They consist of 4 layers:

1. `Config` layer, containing additional information how to manage a machine / volume with the image.
2. `RootFS` layer, containing the root file system for the image.
3. `InitRAMFS` layer, containing the initramfs / initrd for the image.
4. `Kernel` layer, containing the kernel for the image.

## Installation

### Command-Line Tool

To install the command tool, make sure you have a working go installation
and `GOBIN` set up correctly. Then simply run

```shell
make install
```

This will install the tool available under `$GOBIN/onmetal-image`.

### Library

The library behind `onmetal-image` can be fetched by running

```shell
go get github.com/onmetal/onmetal-image@latest
```

## Usage

### Library

For the docs, check out the [onmetal-image pkg.go.dev documentation](https://pkg.go.dev/github.com/onmetal/onmetal-image).

### Command-Line Tool

For getting basic help, you can simply run

```shell
onmetal-image help
```

This will print the available commands.

To build an onmetal-image, you'll need the rootfs, initramfs and kernel
of your desired operating system. Once you have them on disk, simply run

```shell
onmetal-image build \
  --rootfs-file <path to rootfs file> \
  --initramfs-file <path to initramfs> \
  --kernel-file <path to kernel file>
```

This will build the image, put it into your local OCI store (usually at `~/.onmetal`)
and print out the id of the built image.

To tag the image with a more fluent name, run

```shell
onmetal-image tag <id> my-image:latest
```

This will tag the image with the name `my-image` and the tag latest.

To push an image to a remote registry, make sure you authenticated your
local docker client with that registry. Consult your registry provider's documentation
for instructions.

Once authenticated, tag your image so it points towards that registry, e.g.

```shell
onmetal-image tag <id> ghcr.io/onmetal/onmetal-image/my-image:latest
```

To push the image to the registry, run

```shell
onmetal-image push ghcr.io/onmetal/onmetal-image/my-image:latest
```

> :danger: This currently doesn't do any output, wait a while for it to be done.

To pull the pushed image, run

```shell
onmetal-image pull ghcr.io/onmetal/onmetal-image/my-image:latest
```

## Contributing

We'd love to get feedback from you. Please report bugs, suggestions or post questions by opening a GitHub issue.

## License

[Apache-2.0](LICENSE)

