// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package pull

import (
	"context"
	"fmt"

	"github.com/ironcore-dev/ironcore-image/cmd/common"
	"github.com/ironcore-dev/ironcore-image/oci/image"
	"github.com/spf13/cobra"
)

func Command(storeFactory common.StoreFactory, registryFactory common.RemoteRegistryFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull image[:tag]",
		Short: "Pull an image from a remote registry determined by the image name.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := args[0]
			return Run(ctx, storeFactory, registryFactory, ref)
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
	s, err := storeFactory()
	if err != nil {
		return fmt.Errorf("error creating store: %w", err)
	}

	registry, err := registryFactory()
	if err != nil {
		return fmt.Errorf("could not create remote registry: %w", err)
	}

	img, err := image.Copy(ctx, s, registry, ref)
	if err != nil {
		return fmt.Errorf("error pulling ref %s: %w", ref, err)
	}

	fmt.Println("Successfully pulled", ref, img.Descriptor().Digest.Encoded())
	return nil
}
