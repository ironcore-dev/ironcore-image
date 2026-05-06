// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package direct

const (
	Kind = "Direct"

	ArtifactType = "application/vnd.ironcore.direct.v1+json"

	MediaTypeConfig = "application/vnd.ironcore.direct.config.v1+json"

	MediaTypeLayerKernel     = "application/vnd.ironcore.kernel.v1"
	MediaTypeLayerInitrd     = "application/vnd.ironcore.initrd.cpio.v1"
	MediaTypeLayerInitrdGzip = MediaTypeLayerInitrd + "+gzip"
	MediaTypeLayerInitrdZstd = MediaTypeLayerInitrd + "+zstd"
	MediaTypeLayerInitrdXz   = MediaTypeLayerInitrd + "+xz"
	MediaTypeLayerInitrdLz4  = MediaTypeLayerInitrd + "+lz4"
)

var InitrdLayerMediaTypes = []string{
	MediaTypeLayerInitrd,
	MediaTypeLayerInitrdGzip,
	MediaTypeLayerInitrdZstd,
	MediaTypeLayerInitrdXz,
	MediaTypeLayerInitrdLz4,
}
