// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package ironcoreimage

import (
	"github.com/ironcore-dev/ironcore-image/v2/internal/cli/ironcore-image/build"
	"github.com/ironcore-dev/ironcore-image/v2/internal/cli/ironcore-image/common"
	"github.com/ironcore-dev/ironcore-image/v2/internal/cli/ironcore-image/ls"
	"github.com/ironcore-dev/ironcore-image/v2/internal/cli/ironcore-image/pull"
	"github.com/ironcore-dev/ironcore-image/v2/internal/cli/ironcore-image/push"
	"github.com/spf13/cobra"
)

func Command() *cobra.Command {
	var (
		commonOpts = common.DefaultOptions
	)

	cmd := &cobra.Command{
		Use: "ironcore-image",
	}

	cmd.AddCommand(
		build.Command(&commonOpts),
		ls.Command(&commonOpts),
		pull.Command(&commonOpts),
		push.Command(&commonOpts),
	)

	commonOpts.AddFlags(cmd.PersistentFlags())

	return cmd
}
