// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package image

const (
	// defaultConcurrency is the default concurrency to use.
	// The value is consistent with dockerd and containerd.
	defaultConcurrency int = 3

	// defaultMaxMetadataBytes is the default amount of bytes to use
	// for caching metadata.
	defaultMaxMetadataBytes int64 = 4 * 1024 * 1024 // 4 MiB
)
