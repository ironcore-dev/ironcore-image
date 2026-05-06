// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package platform

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// Match checks whether the current platform matches the target platform.
// Match will return true if all of the following conditions are met.
//   - Architecture and OS exactly match.
//   - Variant and OSVersion exactly match if target platform provided.
//   - OSFeatures of the target platform are the subsets of the OSFeatures
//     array of the current platform.
//
// Note: Variant, OSVersion and OSFeatures are optional fields, will skip
// the comparison if the target platform does not provide specific value.
func Match(got *ocispec.Platform, want *ocispec.Platform) bool {
	if got == nil && want == nil {
		return true
	}

	if got == nil || want == nil {
		return false
	}

	if got.Architecture != want.Architecture || got.OS != want.OS {
		return false
	}

	if want.OSVersion != "" && got.OSVersion != want.OSVersion {
		return false
	}

	if want.Variant != "" && got.Variant != want.Variant {
		return false
	}

	if len(want.OSFeatures) != 0 && !isSubset(want.OSFeatures, got.OSFeatures) {
		return false
	}

	return true
}

// isSubset returns true if all items in slice A are present in slice B.
func isSubset(a, b []string) bool {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	for _, v := range a {
		if _, ok := set[v]; !ok {
			return false
		}
	}

	return true
}
