// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ironcoreimage_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIronCoreImage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IronCoreImage Suite")
}
