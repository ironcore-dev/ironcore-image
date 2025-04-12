# IronCore OCI Image Specification

This document defines the OCI image layout and media types expected by the IronCore ecosystem. Operating systems packaged according to this specification are compatible with IronCore's boot and virtualization layers.

## Overview

An IronCore OS image is a custom OCI image, stored and distributed via any OCI-compliant registry. It consists of:

- A **top-level index manifest** describing architecture-specific boot variants.
- One or more **artifact manifests** that represent specific boot modes (e.g., metal or virtualization).
- Well-defined **media types** for each component layer.

Following this specification ensures compatibility with the IronCore metal automation system and the IronCore virtualization layer.

---

## Index Manifest

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.ironcore.image.artifacts.v1+json",
      "digest": "sha256:ijkl9012qrst7890uvwx1234yzab5678abcd1234efgh5678mnop3456",
      "size": 2345,
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.ironcore.image.artifacts.v1+json",
      "digest": "sha256:mnop3456qrst7890uvwx1234yzab5678abcd1234efgh5678ijkl9012",
      "size": 6789,
      "platform": {
        "architecture": "arm64",
        "os": "linux"
      }
    }
  ]
}
```

## Artifact Manifest: Virtualization Boot

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.ironcore.image.artifacts.v1+json",
  "config": {
    "mediaType": "application/vnd.ironcore.image.config.v1+json",
    "digest": "sha256:configdigest1234abcd5678efgh9012ijkl3456mnop7890qrst",
    "size": 100
  },
  "layers": [
    {
      "mediaType": "application/vnd.ironcore.image.disk.img",
      "digest": "sha256:diskimgdigestabcd1234efgh5678ijkl9012mnop3456qrst7890",
      "size": 1073741824
    },
    {
      "mediaType": "application/vnd.ironcore.image.kernel",
      "digest": "sha256:kernelabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 10485760
    },
    {
      "mediaType": "application/vnd.ironcore.image.initramfs",
      "digest": "sha256:initramfsabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 20971520
    },
    {
      "mediaType": "application/vnd.ironcore.image.squashfs",
      "digest": "sha256:squashfsabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 536870912
    },
    {
      "mediaType": "application/vnd.ironcore.image.cmdline",
      "digest": "sha256:cmdlineabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 1024
    },
    {
      "mediaType": "application/vnd.ironcore.image.uki",
      "digest": "sha256:ukiabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 15728640
    },
    {
      "mediaType": "application/vnd.ironcore.image.iso",
      "digest": "sha256:isoabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 734003200
    }
  ],
  "annotations": {
    "org.opencontainers.image.title": "MyOS Virtualization (amd64)",
    "variant": "virtualization",
    "architecture": "amd64"
  }
}
```
## Artifact Manifest: Metal Boot

```json

{
  "schemaVersion": 2,
  "mediaType": "application/vnd.ironcore.image.artifacts.v1+json",
  "config": {
    "mediaType": "application/vnd.ironcore.image.config.v1+json",
    "digest": "sha256:configdigest5678efgh9012ijkl3456mnop7890qrstabcd1234",
    "size": 100
  },
  "layers": [
    {
      "mediaType": "application/vnd.ironcore.image.kernel",
      "digest": "sha256:kernelabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 10485760
    },
    {
      "mediaType": "application/vnd.ironcore.image.initramfs",
      "digest": "sha256:initramfsabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 20971520
    },
    {
      "mediaType": "application/vnd.ironcore.image.squashfs",
      "digest": "sha256:squashfsabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 536870912
    },
    {
      "mediaType": "application/vnd.ironcore.image.cmdline",
      "digest": "sha256:cmdlineabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 1024
    },
    {
      "mediaType": "application/vnd.ironcore.image.uki",
      "digest": "sha256:ukiabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 15728640
    },
    {
      "mediaType": "application/vnd.ironcore.image.iso",
      "digest": "sha256:isoabcd1234efgh5678ijkl9012mnop3456qrst7890uvwx1234",
      "size": 734003200
    }
  ],
  "annotations": {
    "org.opencontainers.image.title": "MyOS MetalBoot (amd64)",
    "variant": "metal",
    "architecture": "amd64"
  }
}
```


