// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/v2/cli/ironcore-image/common"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry"
)

func Command(prov common.Provider) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push NAME[:TAG]",
		Short: "Upload an image to a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			name := args[0]

			target, err := prov.Store(ctx)
			if err != nil {
				return err
			}

			return Run(ctx, target, name)
		},
	}

	return cmd
}

func Run(ctx context.Context, target oras.Target, name string) error {
	ref, err := registry.ParseReference(name)
	if err != nil {
		return fmt.Errorf("parsing reference: %w", err)
	}

	if ref.Reference == "" {
		ref.Reference = ref.ReferenceOrDefault()
	}

	repo, err := common.RepositoryFor(ref)
	if err != nil {
		return fmt.Errorf("creating repository: %w", err)
	}

	repo.PlainHTTP = true

	desc, err := oras.Copy(ctx, target, ref.String(), repo, ref.String(), oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("copying image: %w", err)
	}

	if err := repo.Tag(ctx, desc, ref.String()); err != nil {
		return fmt.Errorf("tagging image: %w", err)
	}
	return nil
}
