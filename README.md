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
make build
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

To build an ironcore-image, prepare the OS artifacts for each target architecture
and pass them via `--config`. You can repeat `--config` for multi-arch builds.
Supported keys are `arch`, `rootfs`, `initramfs`, `kernel`, `squashfs`, `uki`,
`iso`, and `cmdline`.

```shell
ironcore-image build \
  --tag my-image:latest \
  --config arch=amd64,rootfs=./rootfs.ext4,initramfs=./initramfs.img,kernel=./vmlinuz
```

This will build the image, put it into your local OCI store (usually at `~/.ironcore`),
and create a local index manifest tagged `my-image:latest`.

For a multi-arch image, repeat `--config` for each architecture:

```shell
ironcore-image build \
  --tag my-image:latest \
  --config arch=amd64,rootfs=./rootfs-amd64.ext4,initramfs=./initramfs-amd64.img,kernel=./vmlinuz-amd64 \
  --config arch=arm64,rootfs=./rootfs-arm64.ext4,initramfs=./initramfs-arm64.img,kernel=./vmlinuz-arm64
```

To add an additional tag to an existing local image, run

```shell
ironcore-image tag my-image:latest my-image:v1
```

This will tag the same image with the name `my-image` and the tag `v1`.

To push an image to a remote registry, make sure you authenticated your
local docker client with that registry. Consult your registry provider's documentation
for instructions.

Once authenticated, tag your image, so it points towards that registry, e.g.

```shell
ironcore-image tag my-image:latest ghcr.io/ironcore-dev/ironcore-image/my-image:latest
```

To push a multi-arch image and its sub-manifests to the registry, run

```shell
ironcore-image push ghcr.io/ironcore-dev/ironcore-image/my-image:latest --push-sub-manifests
```

This pushes the index manifest and also pushes each arch-specific manifest under
the `-<arch>` suffix (for example `my-image:latest-amd64`).

To pull the pushed image, run

```shell
ironcore-image pull ghcr.io/ironcore-dev/ironcore-image/my-image:latest
```

## OCI Specification

This project also defines and publishes the OCI image specification that operating systems must conform to in order to be compatible with the IronCore ecosystem.

Following this specification ensures that OS images can be used to boot bare-metal machines via the [metal automation](https://github.com/ironcore-dev/metal-operator) layer and can also be used in virtualized environments within the IronCore virtualization stack.

See the full [OCI image layout specification](OCI-SPEC.md) for details on how to package your OS images for IronCore.

## Contributing

We'd love to get feedback from you. Please report bugs, suggestions or post questions by opening a GitHub issue.

## License

[Apache-2.0](LICENSE)
