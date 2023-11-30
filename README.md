# ironcore-image

[![REUSE status](https://api.reuse.software/badge/github.com/ironcore-dev/ironcore-image)](https://api.reuse.software/info/github.com/ironcore-dev/ironcore-image)
[![Go Report Card](https://goreportcard.com/badge/github.com/ironcore-dev/ironcore-image)](https://goreportcard.com/report/github.com/ironcore-dev/ironcore-image)
[![Go Reference](https://pkg.go.dev/badge/github.com/ironcore-dev/ironcore-image.svg)](https://pkg.go.dev/github.com/ironcore-dev/ironcore-image)
[![GitHub License](https://img.shields.io/static/v1?label=License&message=Apache-2.0&color=blue)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](https://makeapullrequest.com)

## Overview

ironcore-image contains a library to simplify interaction with an OCI-comptabile
registry, and it holds the specification as well as a simple command-line client
for the ironcore-image.

IronCore images are used for machine / volume pool implementors (see [ironcore](https://github.com/ironcore-dev/ironcore))
to prepare and bootstrap machines and machine disks.

They are custom OCI images, meaning they can be published in any OCI compatible registry.
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

This will install the tool available under `$GOBIN/ironcore-image`.

### Library

The library behind `ironcore-image` can be fetched by running

```shell
go get github.com/ironcore-dev/ironcore-image@latest
```

## Usage

### Library

For the docs, check out the [ironcore-image pkg.go.dev documentation](https://pkg.go.dev/github.com/ironcore-dev/ironcore-image).

### Command-Line Tool

For getting basic help, you can simply run

```shell
ironcore-image help
```

This will print the available commands.

To build an ironcore-image, you'll need the rootfs, initramfs and kernel
of your desired operating system. Once you have them on disk, simply run

```shell
ironcore-image build \
  --rootfs-file <path to rootfs file> \
  --initramfs-file <path to initramfs> \
  --kernel-file <path to kernel file>
```

This will build the image, put it into your local OCI store (usually at `~/.ironcore`)
and print out the id of the built image.

To tag the image with a more fluent name, run

```shell
ironcore-image tag <id> my-image:latest
```

This will tag the image with the name `my-image` and the tag latest.

To push an image to a remote registry, make sure you authenticated your
local docker client with that registry. Consult your registry provider's documentation
for instructions.

Once authenticated, tag your image, so it points towards that registry, e.g.

```shell
ironcore-image tag <id> ghcr.io/ironcore-dev/ironcore-image/my-image:latest
```

To push the image to the registry, run

```shell
ironcore-image push ghcr.io/ironcore-dev/ironcore-image/my-image:latest
```

> :danger: This currently doesn't do any output, wait a while for it to be done.

To pull the pushed image, run

```shell
ironcore-image pull ghcr.io/ironcore-dev/ironcore-image/my-image:latest
```

## Contributing

We'd love to get feedback from you. Please report bugs, suggestions or post questions by opening a GitHub issue.

## License

[Apache-2.0](LICENSE)
