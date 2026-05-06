// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log/slog"
	"os"

	_ "crypto/sha256"

	ironcoreimage "github.com/ironcore-dev/ironcore-image/v2/cli/ironcore-image"
)

func main() {
	if err := ironcoreimage.Command().Execute(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}
