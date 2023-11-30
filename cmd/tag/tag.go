// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package tag

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag source-image[:tag] target-image[:tag]",
		Short: "Tag a local image with a given name (and optional tag).",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			srcImage := args[0]
			tgtImage := args[1]
			return Run(ctx, storeFactory, srcImage, tgtImage)
		},
	}

	return cmd
}

func Run(ctx context.Context, storeFactory common.StoreFactory, srcImage, tgtImage string) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	desc, err := common.FuzzyResolveRef(ctx, s, srcImage)
	if err != nil {
		return fmt.Errorf("error resolving source: %w", err)
	}

	if err := s.Tag(ctx, desc, tgtImage); err != nil {
		return fmt.Errorf("error tagging image: %w", err)
	}

	fmt.Println("Successfully tagged", tgtImage, "with", desc)
	return nil
}
