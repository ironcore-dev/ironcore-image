// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package qcow2

type Interface interface {
	UnsafeRemoveBacking(filename string) error
	WriteChain(chain []string, filename string) error
}
