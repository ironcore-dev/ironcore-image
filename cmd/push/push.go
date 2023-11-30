// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory, registryFactory common.RemoteRegistryFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push image[:tag]",
		Short: "Push a local image to a remote registry determined by the image name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			name := args[0]
			return Run(ctx, storeFactory, registryFactory, name)
		},
	}

	return cmd
}

func Run(
	ctx context.Context,
	storeFactory common.StoreFactory,
	registryFactory common.RemoteRegistryFactory,
	ref string,
) error {
	store, err := storeFactory()
	if err != nil {
		return fmt.Errorf("error creating store: %w", err)
	}

	registry, err := registryFactory()
	if err != nil {
		return fmt.Errorf("error creating remote registry: %w", err)
	}

	img, err := store.Resolve(ctx, ref)
	if err != nil {
		return fmt.Errorf("error resolving ref %s: %w", ref, err)
	}

	if err := registry.Push(ctx, ref, img); err != nil {
		return fmt.Errorf("error pushing image to %s: %w", ref, err)
	}

	fmt.Println("Successfully pushed", ref, img.Descriptor().Digest.Encoded())
	return nil
}
