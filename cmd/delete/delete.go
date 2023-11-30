// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete image[:tag]",
		Short: "Delete a local image.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			srcImage := args[0]
			return Run(ctx, storeFactory, srcImage)
		},
	}

	return cmd
}

func Run(ctx context.Context, storeFactory common.StoreFactory, ref string) error {
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("could not create store: %w", err)
	}

	if err := s.Delete(ctx, ref); err != nil {
		return fmt.Errorf("error deleting ref %s: %w", ref, err)
	}

	ref, err = common.FuzzyResolveRef(ctx, s, ref)
	if err != nil {
		return fmt.Errorf("error resolving source: %w", err)
	}

	if err := s.Delete(ctx, ref); err != nil {
		return fmt.Errorf("error deleting ref %s: %w", ref, err)
	}

	fmt.Println("Successfully deleted", ref)
	return nil
}
